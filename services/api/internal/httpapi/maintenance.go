package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type maintenanceCheck struct {
	Table  string `json:"table"`
	Rows   int64  `json:"rows"`
	Status string `json:"status"`
}

type maintenanceOperation struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	Message    string          `json:"message"`
	OccurredAt string          `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// These are application-level checks. Physical PostgreSQL checks and backup
// execution remain deployment-owned; the API must not imply that either was
// performed by counting application rows.
var maintenanceIntegrityTables = []string{
	"tenants",
	"branches",
	"counters",
	"users",
	"master_records",
	"sales_documents",
	"inventory_movements",
	"sync_events",
}

func maintenancePermission(kind string) string {
	if kind == "manage-cashier-job" {
		return "sales.read"
	}
	if strings.HasPrefix(kind, "manage-") {
		return "manage.users"
	}
	return "maintenance.write"
}

func validMaintenanceKind(kind string) bool {
	return kind != "" && len(kind) <= 96 && !strings.ContainsAny(kind, `/\?&= `)
}

func maintenanceExternalOutcome(kind string) (string, string) {
	normalized := strings.ToLower(kind)
	switch {
	case normalized == "check-database-integrity":
		return "completed", "Application-scope integrity checks completed. Physical PostgreSQL checks were not run by the API."
	case strings.Contains(normalized, "backup"), strings.Contains(normalized, "restore"):
		return "not_configured", "No deployment backup/restore adapter is configured; no database backup or restore was performed."
	case strings.Contains(normalized, "import"):
		return "not_configured", "No server-side import adapter is configured; no records were imported."
	case strings.Contains(normalized, "export"):
		return "not_configured", "No server-side export adapter is configured; no export artifact was produced."
	case strings.Contains(normalized, "send") || strings.Contains(normalized, "test-sms") || strings.Contains(normalized, "test-email"):
		return "not_configured", "No SMS or email adapter is configured; no external message was sent."
	case normalized == "inplace-initialization" || strings.Contains(normalized, "delete"):
		return "not_configured", "This destructive operation requires an explicit deployment policy; no data was changed."
	default:
		return "saved", "Maintenance configuration saved for the authenticated tenant and branch."
	}
}

func validateMaintenancePayload(kind string, payload map[string]any) error {
	if len(payload) > 100 {
		return errors.New("a maintenance request may contain no more than 100 fields")
	}
	for key, value := range payload {
		if strings.TrimSpace(key) == "" || len(key) > 120 {
			return errors.New("maintenance field names must be between 1 and 120 characters")
		}
		if text, ok := value.(string); ok && len(text) > 4096 {
			return errors.New("maintenance field values may not exceed 4096 characters")
		}
	}
	if strings.Contains(strings.ToLower(kind), "import") {
		source, _ := payload["sourceFile"].(string)
		source = strings.TrimSpace(source)
		if source == "" {
			return errors.New("sourceFile is required for import validation")
		}
		// A client-supplied server path is never opened by this API. Accept only
		// a logical file name so a future upload adapter can bind it safely.
		if strings.Contains(source, "..") || strings.ContainsAny(source, `/\:`) {
			return errors.New("sourceFile must be a logical uploaded file name, not a server path")
		}
	}
	return nil
}

func (s *Server) maintenanceAction(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	kind := strings.TrimSpace(r.PathValue("kind"))
	if !validMaintenanceKind(kind) {
		writeProblem(w, http.StatusBadRequest, "invalid_maintenance_kind", "Invalid maintenance workflow", "The requested maintenance workflow is not supported.")
		return
	}
	if !s.requirePermission(r, w, operator, maintenancePermission(kind)) {
		return
	}

	payload := make(map[string]any)
	if r.Body != nil {
		err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&payload)
		if err != nil && !errors.Is(err, io.EOF) {
			writeProblem(w, http.StatusBadRequest, "invalid_json", "Invalid JSON", "The maintenance request could not be parsed.")
			return
		}
	}
	if err := validateMaintenancePayload(kind, payload); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_maintenance_request", "Invalid maintenance request", err.Error())
		return
	}
	if err := validateBatchLockMaintenancePayload(kind, payload); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_batch_lock_request", "Invalid batch-lock request", err.Error())
		return
	}
	if err := validateCanonicalItemMaintenancePayload(kind, payload); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_item_maintenance_request", "Invalid item maintenance request", err.Error())
		return
	}

	if kind == "check-database-integrity" {
		s.handleIntegrityCheck(w, r, operator, kind, payload)
		return
	}
	if kind == "backup-database" {
		s.handleDatabaseBackup(w, r, operator, kind, payload)
		return
	}
	if kind == "restore-database" {
		s.handleDatabaseRestore(w, r, operator, kind, payload)
		return
	}

	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The maintenance store could not be opened.")
		return
	}
	defer tx.Rollback()

	mutation, err := applyCanonicalItemMaintenance(r.Context(), tx, operator, kind, payload)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "item_maintenance_failed", "Item maintenance failed", err.Error())
		return
	}
	if kind == "lock-item-batches" {
		mutation, err = applyBatchLockMaintenance(r.Context(), tx, operator, payload)
		if err != nil {
			writeProblem(w, http.StatusUnprocessableEntity, "batch_lock_failed", "Batch lock failed", err.Error())
			return
		}
	}

	saved := 0
	for caption, value := range payload {
		caption = strings.TrimSpace(caption)
		if caption == "" || isSensitiveMaintenanceField(caption) {
			continue
		}
		encodedValue, marshalErr := maintenanceValue(value)
		if marshalErr != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_maintenance_value", "Invalid maintenance value", "A maintenance value could not be encoded.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO tenant_preferences (tenant_id, branch_id, category, caption, value, position, updated_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, now())
			ON CONFLICT (tenant_id, (COALESCE(branch_id, '00000000-0000-0000-0000-000000000000'::uuid)), category, caption) DO UPDATE
			SET value = EXCLUDED.value, position = EXCLUDED.position, updated_at = now()
		`, operator.TenantID, operator.BranchID, "maintenance:"+kind, caption, encodedValue, saved); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be saved", "The maintenance settings store rejected the value.")
			return
		}
		saved++
	}

	status, message := maintenanceExternalOutcome(kind)
	operationPayload := payload
	if mutation.status != "" {
		status = mutation.status
		message = mutation.message
		operationPayload = mutation.auditPayload
	}
	operationID, err := recordMaintenanceOperation(r, tx, operator, kind, status, message, operationPayload)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_audit_failed", "Maintenance audit failed", "The maintenance operation could not be recorded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_commit_failed", "Maintenance failed", "The maintenance operation could not be committed.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"kind":        kind,
		"operationId": operationID,
		"job":         map[string]any{"id": operationID, "kind": kind, "status": status},
		"status":      status,
		"outcome":     status,
		"saved":       saved,
		"message":     message,
	})
}

func (s *Server) handleIntegrityCheck(w http.ResponseWriter, r *http.Request, operator *sessionContext, kind string, payload map[string]any) {
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The integrity check could not be opened.")
		return
	}
	defer tx.Rollback()

	checks := make([]maintenanceCheck, 0, len(maintenanceIntegrityTables))
	for _, table := range maintenanceIntegrityTables {
		var count int64
		query := "SELECT count(*) FROM " + table + " WHERE tenant_id = $1::uuid"
		if table == "tenants" {
			query = "SELECT count(*) FROM tenants WHERE id = $1::uuid"
		}
		if err := tx.QueryRowContext(r.Context(), query, operator.TenantID).Scan(&count); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "integrity_check_failed", "Integrity check failed", "A scoped application table could not be checked.")
			return
		}
		checks = append(checks, maintenanceCheck{Table: table, Rows: count, Status: "ok"})
	}
	branchStatus := "ok"
	var branchRows int64
	if operator.BranchID == "" {
		branchStatus = "not_configured"
	} else if err := tx.QueryRowContext(r.Context(), `
		SELECT count(*) FROM branches WHERE tenant_id = $1::uuid AND id = $2::uuid AND active
	`, operator.TenantID, operator.BranchID).Scan(&branchRows); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "integrity_check_failed", "Integrity check failed", "The current branch scope could not be checked.")
		return
	} else if branchRows != 1 {
		branchStatus = "failed"
	}
	checks = append(checks, maintenanceCheck{Table: "current_branch", Rows: branchRows, Status: branchStatus})
	status := "completed"
	message := "Application-scope integrity checks completed. Physical PostgreSQL checks were not run by the API."
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["checks"] = checks
	auditPayload["scope"] = map[string]string{"tenantId": operator.TenantID, "branchId": operator.BranchID}
	auditPayload["physicalDatabaseCheck"] = "not_configured"
	operationID, err := recordMaintenanceOperation(r, tx, operator, kind, status, message, auditPayload)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_audit_failed", "Maintenance audit failed", "The integrity check could not be audited.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_commit_failed", "Maintenance failed", "The integrity check could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":                  kind,
		"operationId":           operationID,
		"status":                status,
		"scope":                 map[string]string{"tenantId": operator.TenantID, "branchId": operator.BranchID},
		"physicalDatabaseCheck": "not_configured",
		"checks":                checks,
		"message":               message,
	})
}

// maintenanceDBConn carries the libpq connection parameters used to invoke
// pg_dump/pg_restore out-of-process. It is resolved from the same
// DATABASE_URL/PG* environment variables the API itself connects with, so a
// backup or restore always talks to the same PostgreSQL instance the API is
// deployed against.
type maintenanceDBConn struct {
	Host     string
	Port     string
	User     string
	Password string
	SSLMode  string
}

// errMaintenancePgToolUnavailable is returned when pg_dump/pg_restore are not
// present on PATH. Callers must fall back to an honest "not_configured"
// outcome rather than claiming a physical backup or restore happened.
var errMaintenancePgToolUnavailable = errors.New("required PostgreSQL client tool is not available on PATH")

func maintenanceDBConnFromEnv() maintenanceDBConn {
	conn := maintenanceDBConn{Host: "127.0.0.1", Port: "5432", User: "postgres", SSLMode: "disable"}
	raw := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("ABUZAR_APP_DATABASE_URL"))
	}
	if raw != "" {
		if parsed, err := url.Parse(raw); err == nil {
			if host := parsed.Hostname(); host != "" {
				conn.Host = host
			}
			if port := parsed.Port(); port != "" {
				conn.Port = port
			}
			if parsed.User != nil {
				if username := parsed.User.Username(); username != "" {
					conn.User = username
				}
				if password, ok := parsed.User.Password(); ok {
					conn.Password = password
				}
			}
			if mode := parsed.Query().Get("sslmode"); mode != "" {
				conn.SSLMode = mode
			}
		}
	}
	if host := strings.TrimSpace(os.Getenv("PGHOST")); host != "" {
		conn.Host = host
	}
	if port := strings.TrimSpace(os.Getenv("PGPORT")); port != "" {
		conn.Port = port
	}
	if user := strings.TrimSpace(os.Getenv("PGUSER")); user != "" {
		conn.User = user
	}
	if password := strings.TrimSpace(os.Getenv("PGPASSWORD")); password != "" {
		conn.Password = password
	}
	return conn
}

func maintenancePgEnv(conn maintenanceDBConn) []string {
	env := os.Environ()
	if conn.Password != "" {
		env = append(env, "PGPASSWORD="+conn.Password)
	}
	if conn.SSLMode != "" {
		env = append(env, "PGSSLMODE="+conn.SSLMode)
	}
	return env
}

// maintenanceBackupDir returns the server-side directory backup files are
// written to and restored from. It never accepts a client-supplied path.
func maintenanceBackupDir() string {
	if dir := strings.TrimSpace(os.Getenv("ABUZAR_MAINTENANCE_BACKUP_DIR")); dir != "" {
		return dir
	}
	return filepath.Join("tmp", "backups")
}

func currentDatabaseName(ctx context.Context, database *sql.DB) (string, error) {
	var name string
	if err := database.QueryRowContext(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

// validMaintenanceDatabaseName restricts a restore target to a plain
// PostgreSQL identifier so it can never be used to smuggle flags or paths
// into pg_restore, and so it can be safely embedded in a single --dbname=
// argument.
func validMaintenanceDatabaseName(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// sanitizeMaintenanceFileToken keeps generated backup file names readable
// while stripping anything that is not a safe path segment character.
func sanitizeMaintenanceFileToken(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	token := builder.String()
	if token == "" {
		token = "tenant"
	}
	return token
}

// runPgDump shells out to the real pg_dump binary in custom (-Fc) format so
// the resulting file can later be fed straight to pg_restore. It returns the
// size in bytes of the file actually written to disk.
func runPgDump(ctx context.Context, conn maintenanceDBConn, database, outputPath string) (int64, error) {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return 0, errMaintenancePgToolUnavailable
	}
	cmd := exec.CommandContext(ctx, "pg_dump",
		"--format=custom",
		"--no-password",
		"--file="+outputPath,
		"--host="+conn.Host,
		"--port="+conn.Port,
		"--username="+conn.User,
		"--dbname="+database,
	)
	cmd.Env = maintenancePgEnv(conn)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pg_dump failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return 0, fmt.Errorf("pg_dump reported success but wrote no file: %w", err)
	}
	return info.Size(), nil
}

// runPgRestore shells out to the real pg_restore binary against an explicit
// target database. --clean --if-exists makes the restore idempotent when the
// target database already has objects in it, while remaining a no-op on a
// freshly created, empty database.
func runPgRestore(ctx context.Context, conn maintenanceDBConn, database, inputPath string) error {
	if _, err := exec.LookPath("pg_restore"); err != nil {
		return errMaintenancePgToolUnavailable
	}
	cmd := exec.CommandContext(ctx, "pg_restore",
		"--no-password",
		"--clean",
		"--if-exists",
		"--host="+conn.Host,
		"--port="+conn.Port,
		"--username="+conn.User,
		"--dbname="+database,
		inputPath,
	)
	cmd.Env = maintenancePgEnv(conn)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_restore failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// handleDatabaseBackup performs a real pg_dump of the PostgreSQL database the
// API is currently connected to and records the resulting file path/size in
// the maintenance audit trail via recordMaintenanceOperation, exactly like
// every other maintenance operation in this file.
//
// Limitation, stated honestly: tenant isolation in this schema is enforced
// by row-level security at query time (app.tenant_id), not by giving each
// tenant a separate physical database. pg_dump operates below RLS, so it
// cannot produce a backup scoped to one tenant's rows only — every tenant
// sharing this database instance is included in the dump file. That is
// recorded in the audit payload's "scopeNote" field rather than being hidden.
func (s *Server) handleDatabaseBackup(w http.ResponseWriter, r *http.Request, operator *sessionContext, kind string, payload map[string]any) {
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The backup operation could not be opened.")
		return
	}
	defer tx.Rollback()

	databaseName, err := currentDatabaseName(r.Context(), s.database)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "backup_failed", "Backup failed", "The database being backed up could not be identified.")
		return
	}

	status := "not_configured"
	message := "pg_dump is not available on this server; no database backup was performed."
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["tenantId"] = operator.TenantID
	auditPayload["database"] = databaseName
	auditPayload["scopeNote"] = "Full PostgreSQL instance dump. Tenant isolation is enforced by row-level security at query time, not by separate databases, so pg_dump cannot produce a tenant-scoped backup; every tenant sharing this database is included."

	if _, lookErr := exec.LookPath("pg_dump"); lookErr == nil {
		dir := maintenanceBackupDir()
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			writeProblem(w, http.StatusServiceUnavailable, "backup_failed", "Backup failed", "The server-side backup directory could not be created.")
			return
		}
		filename := fmt.Sprintf("%s-%s-%s.dump",
			sanitizeMaintenanceFileToken(operator.TenantID),
			sanitizeMaintenanceFileToken(kind),
			time.Now().UTC().Format("20060102T150405.000000000Z"))
		outputPath := filepath.Join(dir, filename)
		conn := maintenanceDBConnFromEnv()
		size, dumpErr := runPgDump(r.Context(), conn, databaseName, outputPath)
		if dumpErr != nil {
			writeProblem(w, http.StatusServiceUnavailable, "backup_failed", "Backup failed", "pg_dump could not produce a backup file: "+dumpErr.Error())
			return
		}
		status = "completed"
		message = fmt.Sprintf("Full PostgreSQL dump of database %q was written to %s (%d bytes) using pg_dump --format=custom.", databaseName, outputPath, size)
		auditPayload["fileName"] = filename
		auditPayload["filePath"] = outputPath
		auditPayload["fileSizeBytes"] = size
		auditPayload["capturedAt"] = time.Now().UTC().Format(time.RFC3339)
	}

	operationID, err := recordMaintenanceOperation(r, tx, operator, kind, status, message, auditPayload)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_audit_failed", "Maintenance audit failed", "The backup operation could not be recorded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_commit_failed", "Maintenance failed", "The backup operation could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":        kind,
		"operationId": operationID,
		"status":      status,
		"database":    databaseName,
		"message":     message,
	})
}

// handleDatabaseRestore performs a real pg_restore of a previously produced
// backup file into an explicit target database supplied by the caller. The
// target database is never inferred or defaulted: it must be named in the
// request payload, and the handler refuses to restore into the database the
// API is currently connected to (the live, in-use database) to avoid an
// operator accidentally destroying the running deployment mid-request.
func (s *Server) handleDatabaseRestore(w http.ResponseWriter, r *http.Request, operator *sessionContext, kind string, payload map[string]any) {
	targetDatabase, textErr := maintenancePayloadText(payload, "targetDatabase")
	if textErr != nil || !validMaintenanceDatabaseName(targetDatabase) {
		writeProblem(w, http.StatusBadRequest, "invalid_restore_target", "Invalid restore target", "targetDatabase is required and must be a plain PostgreSQL database identifier (letters, digits, underscore).")
		return
	}
	backupFile, fileErr := maintenancePayloadText(payload, "backupFile")
	if fileErr != nil || backupFile == "" || strings.Contains(backupFile, "..") || strings.ContainsAny(backupFile, `/\:`) {
		writeProblem(w, http.StatusBadRequest, "invalid_backup_file", "Invalid backup file", "backupFile must be a logical file name previously produced by a backup-database operation, not a path.")
		return
	}
	inputPath := filepath.Join(maintenanceBackupDir(), backupFile)
	if info, statErr := os.Stat(inputPath); statErr != nil || info.IsDir() {
		writeProblem(w, http.StatusNotFound, "backup_not_found", "Backup not found", "The referenced backup file does not exist on this server.")
		return
	}

	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The restore operation could not be opened.")
		return
	}
	defer tx.Rollback()

	liveDatabase, err := currentDatabaseName(r.Context(), s.database)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "restore_failed", "Restore failed", "The currently connected database could not be identified.")
		return
	}
	if strings.EqualFold(liveDatabase, targetDatabase) {
		writeProblem(w, http.StatusConflict, "restore_target_is_live_database", "Restore target is the live database", "Refusing to restore into the database this API is currently connected to; target a separate database explicitly.")
		return
	}

	status := "not_configured"
	message := "pg_restore is not available on this server; no database restore was performed."
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["tenantId"] = operator.TenantID
	auditPayload["targetDatabase"] = targetDatabase
	auditPayload["backupFile"] = backupFile

	if _, lookErr := exec.LookPath("pg_restore"); lookErr == nil {
		conn := maintenanceDBConnFromEnv()
		if restoreErr := runPgRestore(r.Context(), conn, targetDatabase, inputPath); restoreErr != nil {
			writeProblem(w, http.StatusServiceUnavailable, "restore_failed", "Restore failed", "pg_restore could not restore the backup: "+restoreErr.Error())
			return
		}
		status = "completed"
		message = fmt.Sprintf("Backup file %s was restored into database %q using pg_restore --clean --if-exists.", backupFile, targetDatabase)
		auditPayload["restoredAt"] = time.Now().UTC().Format(time.RFC3339)
	}

	operationID, err := recordMaintenanceOperation(r, tx, operator, kind, status, message, auditPayload)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_audit_failed", "Maintenance audit failed", "The restore operation could not be recorded.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_commit_failed", "Maintenance failed", "The restore operation could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":           kind,
		"operationId":    operationID,
		"status":         status,
		"targetDatabase": targetDatabase,
		"message":        message,
	})
}

func (s *Server) maintenanceState(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.PathValue("kind"))
	if !validMaintenanceKind(kind) {
		writeProblem(w, http.StatusBadRequest, "invalid_maintenance_kind", "Invalid maintenance workflow", "The requested maintenance workflow is not supported.")
		return
	}
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, maintenancePermission(kind)) {
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The maintenance store could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `SELECT caption, value, position FROM tenant_preferences WHERE tenant_id = $1::uuid AND category = $2 ORDER BY position, caption`, operator.TenantID, "maintenance:"+kind)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance settings store could not be queried.")
		return
	}
	items := make([]map[string]any, 0)
	for rows.Next() {
		var caption, value string
		var position int
		if err := rows.Scan(&caption, &value, &position); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance settings response could not be decoded.")
			return
		}
		items = append(items, map[string]any{"caption": caption, "value": value, "position": position})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance settings response could not be read.")
		return
	}
	rows.Close()

	operations := make([]maintenanceOperation, 0, 10)
	operationRows, err := tx.QueryContext(r.Context(), `
		SELECT id::text, action, payload, occurred_at::text
		FROM audit_events
		WHERE tenant_id = $1::uuid
		  AND action = $2
		  AND (branch_id IS NULL OR branch_id = NULLIF($3, '')::uuid)
		ORDER BY occurred_at DESC
		LIMIT 10`, operator.TenantID, "maintenance."+kind, operator.BranchID)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance operation ledger could not be queried.")
		return
	}
	for operationRows.Next() {
		var operation maintenanceOperation
		var action string
		var rawPayload []byte
		if err := operationRows.Scan(&operation.ID, &action, &rawPayload, &operation.OccurredAt); err != nil {
			operationRows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance operation response could not be decoded.")
			return
		}
		operation.Payload = json.RawMessage(rawPayload)
		operation.Kind = kind
		var details struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(operation.Payload, &details)
		operation.Status = details.Status
		operation.Message = details.Message
		operations = append(operations, operation)
	}
	if err := operationRows.Err(); err != nil {
		operationRows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance operation response could not be read.")
		return
	}
	operationRows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "maintenance_state_failed", "Maintenance state could not be read", "The maintenance state transaction could not be committed.")
		return
	}
	var lastOperation any
	if len(operations) > 0 {
		lastOperation = operations[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "items": items, "operations": operations, "lastOperation": lastOperation})
}

func (s *Server) sessionMonitor(w http.ResponseWriter, r *http.Request) {
	operator := currentSession(r)
	if !s.requirePermission(r, w, operator, "manage.users") {
		return
	}
	if operator.BranchID == "" {
		writeProblem(w, http.StatusBadRequest, "branch_required", "Branch required", "Select an operational branch before viewing sessions.")
		return
	}
	tx, err := s.beginScopedTx(r.Context(), operator)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database_unavailable", "Database unavailable", "The session monitor could not be opened.")
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), `
		SELECT se.user_id::text, u.username, u.display_name,
		       se.branch_id::text, COALESCE(se.counter_id::text, ''),
		       se.created_at::text, se.last_seen_at::text, se.expires_at::text,
		       (se.token_hash = $3)
		FROM sessions se
		JOIN users u ON u.id = se.user_id AND u.tenant_id = se.tenant_id
		WHERE se.tenant_id = $1::uuid AND se.branch_id = $2::uuid AND se.expires_at > now()
		ORDER BY se.last_seen_at DESC`, operator.TenantID, operator.BranchID, operator.TokenHash)
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "session_monitor_failed", "Session monitor failed", "The scoped session store could not be queried.")
		return
	}
	sessions := make([]map[string]any, 0)
	for rows.Next() {
		var userID, username, displayName, branchID, counterID, createdAt, lastSeenAt, expiresAt string
		var current bool
		if err := rows.Scan(&userID, &username, &displayName, &branchID, &counterID, &createdAt, &lastSeenAt, &expiresAt, &current); err != nil {
			rows.Close()
			writeProblem(w, http.StatusServiceUnavailable, "session_monitor_failed", "Session monitor failed", "The session response could not be decoded.")
			return
		}
		sessions = append(sessions, map[string]any{
			"userId": userID, "username": username, "displayName": displayName,
			"branchId": branchID, "counterId": counterID, "createdAt": createdAt,
			"lastSeenAt": lastSeenAt, "expiresAt": expiresAt, "current": current,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeProblem(w, http.StatusServiceUnavailable, "session_monitor_failed", "Session monitor failed", "The session response could not be read.")
		return
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "session_monitor_failed", "Session monitor failed", "The session monitor transaction could not be committed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenantId": operator.TenantID, "branchId": operator.BranchID, "sessions": sessions})
}

func maintenanceValue(value any) (string, error) {
	if text, ok := value.(string); ok {
		return text, nil
	}
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

func isSensitiveMaintenanceField(caption string) bool {
	lower := strings.ToLower(caption)
	return strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token")
}

func copyMaintenancePayload(payload map[string]any) map[string]any {
	copy := make(map[string]any, len(payload)+3)
	for key, value := range payload {
		if !isSensitiveMaintenanceField(key) {
			copy[key] = value
		}
	}
	return copy
}

func recordMaintenanceOperation(r *http.Request, tx *sql.Tx, operator *sessionContext, kind, status, message string, payload map[string]any) (string, error) {
	auditPayload := copyMaintenancePayload(payload)
	auditPayload["status"] = status
	auditPayload["message"] = message
	encoded, err := json.Marshal(auditPayload)
	if err != nil {
		return "", err
	}
	var operationID string
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO audit_events (tenant_id, branch_id, operator_id, action, entity_type, payload)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, $4, 'maintenance_operation', $5::jsonb)
		RETURNING id::text
	`, operator.TenantID, operator.BranchID, operator.UserID, "maintenance."+kind, encoded).Scan(&operationID)
	return operationID, err
}

// Kept as a small compatibility helper for callers that only need an audit
// event and do not need the operation identifier.
func appendMaintenanceAudit(r *http.Request, tx *sql.Tx, operator *sessionContext, kind string, payload map[string]any) error {
	_, err := recordMaintenanceOperation(r, tx, operator, kind, "saved", "Maintenance action recorded.", payload)
	return err
}
