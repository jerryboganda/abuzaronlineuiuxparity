package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Event struct {
	EventID        string          `json:"eventId"`
	Aggregate      string          `json:"aggregate"`
	AggregateID    string          `json:"aggregateId"`
	TenantID       string          `json:"tenantId"`
	BranchID       string          `json:"branchId"`
	CounterID      string          `json:"counterId"`
	OperatorID     string          `json:"operatorId"`
	OccurredAt     string          `json:"occurredAt"`
	IdempotencyKey string          `json:"idempotencyKey"`
	SchemaVersion  int             `json:"schemaVersion"`
	Payload        json.RawMessage `json:"payload"`
}

func (e Event) Validate() error {
	if e.EventID == "" || e.Aggregate == "" || e.AggregateID == "" || e.TenantID == "" || e.BranchID == "" || e.CounterID == "" || e.OperatorID == "" || e.OccurredAt == "" || e.IdempotencyKey == "" {
		return errors.New("event is missing required fields")
	}
	if e.SchemaVersion <= 0 {
		return errors.New("event schemaVersion must be positive")
	}
	if !json.Valid(e.Payload) {
		return errors.New("event payload is not valid JSON")
	}
	return nil
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("edge database path is empty")
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create edge data directory: %w", err)
		}
	}

	database, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	store := &Store{db: database}
	if path == ":memory:" {
		database.SetMaxOpenConns(1)
	}
	if err := store.migrate(ctx); err != nil {
		database.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) PendingCount(ctx context.Context) (int64, error) {
	acknowledged, err := s.Cursor(ctx, "central_pushed_sequence")
	if err != nil {
		return 0, err
	}
	cursor, _ := strconv.ParseInt(acknowledged, 10, 64)
	var count int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM edge_events WHERE origin = 'local' AND sequence_no > ?`, cursor).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) Cursor(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM edge_sync_state WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *Store) SetCursor(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO edge_sync_state (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}

func (s *Store) InsertEvent(ctx context.Context, event Event) (bool, error) {
	return s.insertEvent(ctx, event, "local")
}

// InsertPulledEvent records a server-authoritative event without counting it
// as an outgoing local event on the next central synchronization pass.
func (s *Store) InsertPulledEvent(ctx context.Context, event Event) (bool, error) {
	return s.insertEvent(ctx, event, "central")
}

func (s *Store) insertEvent(ctx context.Context, event Event, origin string) (bool, error) {
	if err := event.Validate(); err != nil {
		return false, err
	}
	if origin != "local" && origin != "central" {
		return false, errors.New("event origin is invalid")
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO edge_events
		(event_id, aggregate, aggregate_id, tenant_id, branch_id, counter_id, operator_id,
		 occurred_at, idempotency_key, schema_version, payload, origin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.EventID, event.Aggregate, event.AggregateID, event.TenantID, event.BranchID,
		event.CounterID, event.OperatorID, event.OccurredAt, event.IdempotencyKey,
		event.SchemaVersion, []byte(event.Payload), origin)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) OutgoingAfter(ctx context.Context, cursor int64, limit int) ([]Event, int64, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT sequence_no, event_id, aggregate, aggregate_id, tenant_id, branch_id,
		       counter_id, operator_id, occurred_at, idempotency_key, schema_version, payload
		FROM edge_events WHERE origin = 'local' AND sequence_no > ? ORDER BY sequence_no LIMIT ?
	`, cursor, limit)
	if err != nil {
		return nil, cursor, err
	}
	defer rows.Close()

	var events []Event
	newCursor := cursor
	for rows.Next() {
		var sequence int64
		var event Event
		var payload []byte
		if err := rows.Scan(&sequence, &event.EventID, &event.Aggregate, &event.AggregateID,
			&event.TenantID, &event.BranchID, &event.CounterID, &event.OperatorID,
			&event.OccurredAt, &event.IdempotencyKey, &event.SchemaVersion, &payload); err != nil {
			return nil, cursor, err
		}
		event.Payload = json.RawMessage(payload)
		events = append(events, event)
		newCursor = sequence
	}
	if err := rows.Err(); err != nil {
		return nil, cursor, err
	}
	return events, newCursor, nil
}

func (s *Store) EventsAfter(ctx context.Context, cursor int64, limit int) ([]Event, int64, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT sequence_no, event_id, aggregate, aggregate_id, tenant_id, branch_id,
		       counter_id, operator_id, occurred_at, idempotency_key, schema_version, payload
		FROM edge_events WHERE sequence_no > ? ORDER BY sequence_no LIMIT ?
	`, cursor, limit)
	if err != nil {
		return nil, cursor, err
	}
	defer rows.Close()

	var events []Event
	newCursor := cursor
	for rows.Next() {
		var sequence int64
		var event Event
		var payload []byte
		if err := rows.Scan(&sequence, &event.EventID, &event.Aggregate, &event.AggregateID,
			&event.TenantID, &event.BranchID, &event.CounterID, &event.OperatorID,
			&event.OccurredAt, &event.IdempotencyKey, &event.SchemaVersion, &payload); err != nil {
			return nil, cursor, err
		}
		event.Payload = json.RawMessage(payload)
		events = append(events, event)
		newCursor = sequence
	}
	if err := rows.Err(); err != nil {
		return nil, cursor, err
	}
	return events, newCursor, nil
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS edge_events (
			sequence_no INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id TEXT NOT NULL UNIQUE,
			aggregate TEXT NOT NULL,
			aggregate_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			branch_id TEXT NOT NULL,
			counter_id TEXT NOT NULL,
			operator_id TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			schema_version INTEGER NOT NULL DEFAULT 1,
			payload BLOB NOT NULL,
			origin TEXT NOT NULL DEFAULT 'local' CHECK (origin IN ('local', 'central')),
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (tenant_id, idempotency_key)
		);
		CREATE INDEX IF NOT EXISTS idx_edge_events_scope ON edge_events (tenant_id, branch_id, sequence_no);
		CREATE TABLE IF NOT EXISTS edge_sync_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		return err
	}
	// Databases created by the first vertical slice predate the origin column.
	// SQLite has no portable ADD COLUMN IF NOT EXISTS, so inspect the schema
	// before adding it and keep existing rows as local events.
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(edge_events)`)
	if err != nil {
		return err
	}
	hasOrigin := false
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == "origin" {
			hasOrigin = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if !hasOrigin {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE edge_events ADD COLUMN origin TEXT NOT NULL DEFAULT 'local'`); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_edge_events_outgoing ON edge_events (origin, sequence_no)`)
	return err
}
