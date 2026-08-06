package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthWithoutDatabase(t *testing.T) {
	server := New(nil, "test", "http://localhost:5173")
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["database"] != "not_configured" {
		t.Fatalf("database = %v, want not_configured", body["database"])
	}
	if body["status"] != "degraded" {
		t.Fatalf("status = %v, want degraded without a database", body["status"])
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
}

func TestMetricsExposeOnlyProcessCounters(t *testing.T) {
	server := New(nil, "test", "")
	req := httptest.NewRequest(http.MethodGet, "/v1/metrics", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if _, ok := body["requestsTotal"]; !ok {
		t.Fatalf("metrics omitted requestsTotal: %v", body)
	}
	if _, ok := body["serverErrors"]; !ok {
		t.Fatalf("metrics omitted serverErrors: %v", body)
	}
	if _, exposed := body["tenantId"]; exposed {
		t.Fatalf("metrics exposed tenant scope: %v", body)
	}
}

func TestSessionContextExposesUsernameWithoutCredentials(t *testing.T) {
	operator := &sessionContext{
		UserID:      "operator-id",
		Username:    "ADMIN",
		DisplayName: "Local Administrator",
		TenantID:    "tenant-id",
		TenantCode:  "demo",
		Roles:       []string{"tenant_admin"},
	}
	encoded, err := json.Marshal(operator)
	if err != nil {
		t.Fatalf("marshal session context: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("decode session context: %v", err)
	}
	if body["username"] != "ADMIN" {
		t.Fatalf("username = %v, want ADMIN", body["username"])
	}
	if _, exposed := body["password"]; exposed || strings.Contains(string(encoded), "password") {
		t.Fatalf("session context exposed password material: %s", encoded)
	}
}

func TestTenantRouteRequiresAuthentication(t *testing.T) {
	server := New(nil, "test", "http://localhost:5173")
	req := httptest.NewRequest(http.MethodGet, "/v1/tenants", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestOperatorUpdateRequiresAuthentication(t *testing.T) {
	server := New(nil, "test", "http://localhost:5173")
	req := httptest.NewRequest(http.MethodPatch, "/v1/operators/00000000-0000-0000-0000-000000000001", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestInventoryBalanceRequiresAuthentication(t *testing.T) {
	server := New(nil, "test", "")
	req := httptest.NewRequest(http.MethodGet, "/v1/inventory/balance?itemLegacyId=ITEM-1", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNormalizeRolePermissions(t *testing.T) {
	permissions, err := normalizeRolePermissions([]string{" SALES.WRITE ", "reports.read", "sales.write", ""})
	if err != nil {
		t.Fatalf("normalizeRolePermissions returned error: %v", err)
	}
	got := strings.Join(permissions, ",")
	if got != "reports.read,sales.write" {
		t.Fatalf("normalized permissions = %q, want reports.read,sales.write", got)
	}
	if _, err := normalizeRolePermissions([]string{"unknown.permission"}); err == nil {
		t.Fatal("unsupported permission was accepted")
	}
}

func TestPermissionCheckAllowsAdminAndAssignedPermissionOnly(t *testing.T) {
	operator := &sessionContext{Roles: []string{"operator"}, Permissions: []string{"sales.read"}}
	if !hasPermission(operator, "sales.read") {
		t.Fatal("assigned permission was rejected")
	}
	if hasPermission(operator, "sales.write") {
		t.Fatal("unassigned permission was accepted")
	}
	operator.Roles = []string{"tenant_admin"}
	if !hasPermission(operator, "manage.groups") {
		t.Fatal("tenant administrator did not receive full permission bypass")
	}
}

func TestLegacyGroupEquivalentRolePaths(t *testing.T) {
	tests := []struct {
		name        string
		role        string
		permissions []string
		required    string
		allowed     bool
	}{
		{name: "administrator", role: "ADMINISTRATOR", required: "manage.groups", allowed: true},
		{name: "remote", role: "REMOTE", permissions: []string{"reports.read"}, required: "reports.read", allowed: true},
		{name: "sales officer", role: "SALES OFFICER", permissions: []string{"sales.read", "sales.write"}, required: "sales.write", allowed: true},
		{name: "shift incharge", role: "SHIFT INCHARGE", permissions: []string{"sales.read", "purchases.read"}, required: "purchases.read", allowed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operator := &sessionContext{Roles: []string{test.role}, Permissions: test.permissions}
			if got := hasPermission(operator, test.required); got != test.allowed {
				t.Fatalf("hasPermission(%q, %q) = %v, want %v", test.role, test.required, got, test.allowed)
			}
		})
	}
}

func TestRevokedLegacyRightFailsClosed(t *testing.T) {
	operator := &sessionContext{Roles: []string{"SALES OFFICER"}, Permissions: []string{"sales.read"}}
	if hasPermission(operator, "sales.write") {
		t.Fatal("revoked sales.write right was accepted")
	}
	if hasPermission(operator, "reports.read") {
		t.Fatal("unassigned reports.read right was accepted")
	}
}

func TestLegacyAllowedScopesFailClosedWhenAnAllowListExists(t *testing.T) {
	operator := &sessionContext{
		TenantID: "tenant-a",
		Scopes: map[string]map[string]bool{
			"godown": {"godown-a": true, "godown-b": false},
			"report": {"daily-sales-detail": true},
		},
	}

	if !scopeAllowed(operator, "godown", "godown-a") {
		t.Fatal("allowed godown was rejected")
	}
	if scopeAllowed(operator, "godown", "godown-b") {
		t.Fatal("revoked godown was accepted")
	}
	if scopeAllowed(operator, "godown", "godown-c") {
		t.Fatal("unlisted godown was accepted")
	}
	if !scopeAllowed(operator, "price", "price-a") {
		t.Fatal("unmigrated price scope did not preserve compatibility")
	}
	if scopeAllowed(&sessionContext{TenantID: "tenant-b", Scopes: operator.Scopes}, "report", "other-report") {
		t.Fatal("tenant-scoped report allow-list accepted an unlisted report")
	}
}

func TestImportedRightResolutionPreservesExplicitDenyAndAmbiguity(t *testing.T) {
	rights := resolveLegacyRights([]legacyRight{
		{RightCode: "CMD-100", Permission: "sales.write", Allowed: true},
		{RightCode: "CMD-100", Permission: "sales.write", Allowed: false},
		{RightCode: "CMD-200", Allowed: true},
	})
	if len(rights) != 2 {
		t.Fatalf("resolved rights = %d, want 2", len(rights))
	}
	if rights[0].RightCode != "CMD-100" || rights[0].Allowed {
		t.Fatalf("explicit deny did not win: %+v", rights[0])
	}
	if rights[1].Mapping != "ambiguous" {
		t.Fatalf("unmapped right was presented as exact: %+v", rights[1])
	}
}

func TestImportedScopeResolutionPreservesExplicitDeny(t *testing.T) {
	scopes := resolveLegacyScopes([]legacyScope{
		{ScopeKind: "godown", ScopeKey: "G1", Allowed: true},
		{ScopeKind: "godown", ScopeKey: "G1", Allowed: false},
		{ScopeKind: "price", ScopeKey: "P1", Allowed: true},
	})
	if len(scopes) != 2 {
		t.Fatalf("resolved scopes = %d, want 2", len(scopes))
	}
	for _, scope := range scopes {
		if scope.ScopeKind == "godown" && scope.Allowed {
			t.Fatal("explicit godown deny was lost")
		}
		if !scopeAllowed(&sessionContext{Roles: []string{"tenant_admin"}, Scopes: map[string]map[string]bool{"branch": {"branch-a": false}}}, "branch", "branch-a") {
			t.Fatal("tenant administrator did not retain full scope access")
		}
	}
}

func TestDeniedPermissionReturnsAuditableProblemResponse(t *testing.T) {
	server := &Server{}
	request := httptest.NewRequest(http.MethodPost, "/v1/transactions/sales", nil)
	recorder := httptest.NewRecorder()
	if server.requirePermission(request, recorder, &sessionContext{TenantID: "tenant-a"}, "sales.write") {
		t.Fatal("revoked permission unexpectedly passed")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	var problem map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["code"] != "permission_required" {
		t.Fatalf("problem code = %v, want permission_required", problem["code"])
	}
}

func TestSessionTokenIsRandomAndOnlyHashIsStored(t *testing.T) {
	first, firstHash, err := newSessionToken()
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	second, secondHash, err := newSessionToken()
	if err != nil {
		t.Fatalf("second token: %v", err)
	}
	if first == second || firstHash == secondHash {
		t.Fatal("session tokens must be unpredictable and unique")
	}
	if hashSessionToken(first) != firstHash || hashSessionToken(second) != secondHash {
		t.Fatal("session hash is not deterministic")
	}
	if len(firstHash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(firstHash))
	}
}

func TestEventScopeRejectsCrossTenantAndBranch(t *testing.T) {
	operator := &sessionContext{UserID: "operator", TenantID: "tenant-a", BranchID: "branch-a", CounterID: "counter-a"}
	valid := syncEvent{TenantID: "tenant-a", BranchID: "branch-a", CounterID: "counter-a", OperatorID: "operator"}
	if err := validateEventScope(valid, operator); err != nil {
		t.Fatalf("valid scope rejected: %v", err)
	}
	valid.TenantID = "tenant-b"
	if err := validateEventScope(valid, operator); err == nil {
		t.Fatal("cross-tenant event accepted")
	}
	valid.TenantID = "tenant-a"
	valid.BranchID = "branch-b"
	if err := validateEventScope(valid, operator); err == nil {
		t.Fatal("cross-branch event accepted")
	}
}

func TestSyncEventScopeRequiresCompleteBranchForwardingIdentity(t *testing.T) {
	operator := &sessionContext{UserID: "operator", TenantID: "tenant-a", BranchID: "branch-a", Roles: []string{"tenant_admin"}}
	if err := validateSyncEventScope(nil, nil, syncEvent{TenantID: "tenant-b", BranchID: "branch-a", CounterID: "counter-a", OperatorID: "operator"}, operator); err == nil {
		t.Fatal("cross-tenant synchronization event accepted")
	}
	if err := validateSyncEventScope(nil, nil, syncEvent{TenantID: "tenant-a", BranchID: "branch-a", OperatorID: "operator"}, operator); err == nil {
		t.Fatal("event without counter scope accepted")
	}
	operator.Roles = []string{"operator"}
	if err := validateSyncEventScope(nil, nil, syncEvent{TenantID: "tenant-a", BranchID: "branch-a", CounterID: "counter-a", OperatorID: "other"}, operator); err == nil {
		t.Fatal("non-admin event operator forwarding accepted")
	}
}

func TestReportAggregateConditionMapsLegacyLeaves(t *testing.T) {
	cases := map[string]string{
		"sale-summary":                 "se.aggregate = 'sale'",
		"sales-return-detail":          "se.aggregate = 'sale_return'",
		"purchase-return-summary":      "se.aggregate = 'return'",
		"purchase-order-summary":       "se.aggregate = 'purchase_order'",
		"stock-in-hand-category-wise":  "se.aggregate = 'inventory'",
		"quotation-detail":             "se.aggregate = 'quotation'",
		"refused-sales-detail":         "se.aggregate = 'refused_sale'",
		"unclassified-captured-report": "se.aggregate IN ('sale', 'sale_return', 'refused_sale', 'receiving', 'return', 'purchase_order', 'quotation', 'inventory')",
	}
	for kind, want := range cases {
		if got := reportAggregateCondition(kind); got != want {
			t.Errorf("reportAggregateCondition(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestPhaseNReportRegistryDefinitionsAndAggregateFilters(t *testing.T) {
	if len(phaseNReportRegistry) < 60 {
		t.Fatalf("Phase N registry contains %d definitions; expected the captured daily/sales wave", len(phaseNReportRegistry))
	}
	for kind, spec := range phaseNReportRegistry {
		definition := reportDefinitionFor(kind)
		resolvedSpec, _ := reportSpecForKey(kind)
		if resolvedSpec.financeMode != "" {
			if definition.ProjectionStatus != "real" && resolvedSpec.financeMode != "tax-withholding" {
				t.Errorf("%s financial projection status = %q", kind, definition.ProjectionStatus)
			}
			if definition.Title != spec.title {
				t.Errorf("%s title = %q, want %q", kind, definition.Title, spec.title)
			}
			continue
		}
		if definition.ProjectionStatus != "event-ledger" {
			t.Errorf("%s projection status = %q, want event-ledger", kind, definition.ProjectionStatus)
		}
		if definition.Title != spec.title {
			t.Errorf("%s title = %q, want %q", kind, definition.Title, spec.title)
		}
		if got := reportAggregateCondition(kind); got != spec.aggregateCondition {
			t.Errorf("%s aggregate condition = %q, want %q", kind, got, spec.aggregateCondition)
		}
		if len(definition.Columns) != 6 || definition.Columns[0].Label != "Event / Document" {
			t.Errorf("%s columns do not describe the event payload: %+v", kind, definition.Columns)
		}
		if !definition.Retrieval.SupportsDateRange || !definition.Retrieval.SupportsTextFilter || definition.Retrieval.Scope == "" {
			t.Errorf("%s retrieval metadata is incomplete: %+v", kind, definition.Retrieval)
		}
		if definition.Retrieval.SupportsCashCredit {
			t.Errorf("%s advertises cash/credit filtering without an event payload field for it", kind)
		}
	}
}

func TestReportRegistryResolvesAmbiguousCapturedLeafPaths(t *testing.T) {
	cases := map[string]struct {
		path string
		want string
	}{
		"sale-detail":         {path: "&Reports > Daily Reports > Sale > Sale detail", want: "sale-detail"},
		"sales-return-detail": {path: "&Reports > Daily Reports > Sales Return > Sales Return detail", want: "sales-return-detail"},
		"detail":              {path: "Reports > Sales Reports > Customer Sales > Detail", want: "customer-sales-detail"},
		"summary":             {path: "Reports > Sales Reports > Customer Sales > Summary", want: "customer-sales-summary"},
		"sales":               {path: "Reports > Sales Reports > User Wise > Sales", want: "user-wise-sales"},
	}
	for kind, test := range cases {
		if got := reportRegistryKey(kind, test.path); got != test.want {
			t.Errorf("reportRegistryKey(%q) = %q, want %q", kind, got, test.want)
		}
	}
}

func TestPhaseOReportRegistryCoversCapturedPurchaseLeaves(t *testing.T) {
	if len(phaseOReportRegistry) != 24 {
		t.Fatalf("Phase O registry contains %d definitions, want 24 mapped purchase leaves", len(phaseOReportRegistry))
	}
	for kind, spec := range phaseOReportRegistry {
		t.Run(kind, func(t *testing.T) {
			definition := reportDefinitionFor(kind)
			resolvedSpec, _ := reportSpecForKey(kind)
			if resolvedSpec.financeMode != "" {
				if definition.ProjectionStatus != "real" && resolvedSpec.financeMode != "tax-withholding" &&
					definition.ProjectionStatus != "generic-fallback" {
					t.Fatalf("financial projection status = %q", definition.ProjectionStatus)
				}
				if definition.Title != spec.title {
					t.Fatalf("title = %q, want %q", definition.Title, spec.title)
				}
				return
			}
			if definition.ProjectionStatus != "event-ledger" {
				t.Fatalf("projection status = %q, want event-ledger", definition.ProjectionStatus)
			}
			if definition.Title != spec.title {
				t.Fatalf("title = %q, want %q", definition.Title, spec.title)
			}
			if !spec.purchaseReadModel || spec.purchaseMode == "" {
				t.Fatal("purchase read-model mode is not explicit")
			}
			if reportAggregateCondition(kind) != spec.aggregateCondition {
				t.Fatalf("aggregate condition = %q, want %q", reportAggregateCondition(kind), spec.aggregateCondition)
			}
			if len(definition.Columns) != 6 || definition.Columns[2].Label != "Customer/Supplier" {
				t.Fatalf("columns do not describe available purchase values: %+v", definition.Columns)
			}
			if len(definition.Formats) != 1 || definition.Formats[0].Name != "Standard" {
				t.Fatalf("formats = %+v, want one truthful default format", definition.Formats)
			}
			if definition.Retrieval.SupportsCashCredit || !definition.Retrieval.SupportsDateRange ||
				!definition.Retrieval.SupportsTextFilter || !strings.Contains(definition.Retrieval.Scope, "supplier party ledger") {
				t.Fatalf("retrieval metadata is not purchase-scoped: %+v", definition.Retrieval)
			}
			if !strings.Contains(definition.ProjectionNote, "tax") || !strings.Contains(definition.ProjectionNote, "profit") {
				t.Fatalf("definition does not disclose unreconciled fields: %q", definition.ProjectionNote)
			}
		})
	}
}

func TestPhaseOReportRegistryResolvesEveryCapturedPurchasePath(t *testing.T) {
	paths := map[string]string{
		"purchase-detail":                               "&Reports > Daily Reports > Purchase > Purchase detail",
		"purchase-summary":                              "&Reports > Daily Reports > Purchase > Purchase summary",
		"purchase-summary2":                             "&Reports > Daily Reports > Purchase > Purchase Summary2",
		"purchase-return-detail":                        "&Reports > Daily Reports > Purchase Return > Purchase Return detail",
		"purchase-return-summary":                       "&Reports > Daily Reports > Purchase Return > Purchase Return summary",
		"purchase-order-summary":                        "&Reports > Daily Reports > Purchase Order > Purchase Order Summary",
		"p-o-based-purchase-disparity":                  "&Reports > Daily Reports > Purchase Order > P/O Based Purchase Disparity",
		"periodic-purchases":                            "&Reports > Purchase Reports > Periodic Purchases",
		"purchase-order":                                "&Reports > Purchase Reports > Purchase Order",
		"supplier-wise-detail":                          "&Reports > Purchase Reports > Supplier Wise > Detail",
		"supplier-wise-purchase-detail":                 "&Reports > Purchase Reports > Supplier Wise > Purchase Detail",
		"supplier-wise-advance-income-tax":              "&Reports > Purchase Reports > Supplier Wise > Advance Income Tax",
		"manufacturer-wise-detail":                      "&Reports > Purchase Reports > Manufacturer Wise > Detail",
		"manufacturer-wise-monthly-stock-movement":      "&Reports > Purchase Reports > Manufacturer Wise > Monthly Stock Movement",
		"monthly-purchase-graph":                        "&Reports > Purchase Reports > Monthly Purchase Graph",
		"category-wise-purchase":                        "&Reports > Purchase Reports > Category Wise Purchase",
		"days-summary":                                  "&Reports > Purchase Reports > Days Summary",
		"purchase-order-supplier-wise":                  "&Reports > Purchase Reports > Purchase Order Supplier Wise",
		"net-purchase-summary":                          "&Reports > Purchase Reports > Net Purchase Summary",
		"supplier-category-wise-input-sales-tax-report": "&Reports > Purchase Reports > Supplier Category Wise > Input Sales Tax Report",
		"withholding-tax-deduction":                     "&Reports > Purchase Reports > Withholding Tax Deduction",
		"supplier-manufacturer-wise-g-p":                "&Reports > Purchase Reports > Supplier/Manufacturer Wise G/P",
		"supplier-purchase-returns-detail":              "&Reports > Purchase Return Reports > Supplier Purchase Returns > Detail",
		"supplier-purchase-returns-summary":             "&Reports > Purchase Return Reports > Supplier Purchase Returns > Summary",
	}
	for kind, path := range paths {
		if got := reportRegistryKey("legacy-leaf", path); got != kind {
			t.Errorf("reportRegistryKey(%q, %q) = %q, want %q", "legacy-leaf", path, got, kind)
		}
	}
}

func TestPurchaseReadModelUsesCanonicalLedgersPostedFiltersAndPagination(t *testing.T) {
	query := purchaseReadModelQuery("se.aggregate = 'receiving'", "detail", "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"FROM business_documents d",
		"business_document_lines l",
		"FROM stock_ledger sl",
		"FROM party_ledger_entries p",
		"d.status = 'posted'",
		"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
		"NOT EXISTS (",
		"d.id = compatibility.aggregate_id",
		"ORDER BY occurred_at DESC, document, item LIMIT $6 OFFSET $7",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("purchase read model is missing %q", fragment)
		}
	}
	if !strings.Contains(purchaseReadModelQuery("se.aggregate = 'purchase_order'", "summary", "LIMIT $6 OFFSET $7"), "d.kind IN ('purchase-order')") {
		t.Fatal("purchase order report did not select the canonical purchase-order kind")
	}
	if !strings.Contains(purchaseReadModelQuery("se.aggregate = 'return'", "summary", "LIMIT $6 OFFSET $7"), "d.kind IN ('purchase-return')") {
		t.Fatal("purchase return report did not select the canonical purchase-return kind")
	}
}

func TestPhasePStockRegistryCoversCapturedLeaves(t *testing.T) {
	if len(phasePReportRegistry) != 27 {
		t.Fatalf("Phase P registry contains %d definitions, want 27 captured stock leaves", len(phasePReportRegistry))
	}
	for kind, spec := range phasePReportRegistry {
		t.Run(kind, func(t *testing.T) {
			definition := reportDefinitionFor(kind)
			if definition.ProjectionStatus != "real" {
				t.Fatalf("projection status = %q, want real normalized projection", definition.ProjectionStatus)
			}
			if !spec.stockReadModel || spec.stockMode == "" {
				t.Fatal("stock read-model mode is not explicit")
			}
			if len(definition.Columns) != 6 || definition.Columns[0].Label == "Event / Document" {
				t.Fatalf("columns do not describe normalized stock values: %+v", definition.Columns)
			}
			if len(definition.Formats) != 1 || definition.Formats[0].Name != "Standard" {
				t.Fatalf("formats = %+v, want one truthful default format", definition.Formats)
			}
			if !strings.Contains(definition.ProjectionNote, "Normalized") ||
				!strings.Contains(definition.Retrieval.Scope, "posted stock_ledger") {
				t.Fatalf("stock definition is missing truthful projection metadata: %+v", definition)
			}
		})
	}
}

func TestPhasePStockRegistryResolvesEveryCapturedPath(t *testing.T) {
	paths := map[string]string{
		"stock-in-hand-manufacturer-wise":                  "Reports > Stock Reports > Stock In hand > Manufacturer wise",
		"stock-in-hand-category-wise":                      "Reports > Stock Reports > Stock In hand > Category wise",
		"stock-in-hand-others":                             "Reports > Stock Reports > Stock In hand > Others",
		"stock-in-hand-class-wise":                         "Reports > Stock Reports > Stock In hand > Class Wise",
		"stock-in-hand-batch-priority-wise":                "Reports > Stock Reports > Stock In hand > Batch, Priority Wise",
		"stock-in-hand-back-date":                          "Reports > Stock Reports > Stock In hand > Back Date",
		"stock-in-hand-manufacturer-wise-format2":          "Reports > Stock Reports > Stock In hand > Manufacturer Wise (Format2)",
		"stock-in-hand-supplier-manufacturer-association":  "Reports > Stock Reports > Stock In hand > Supplier Manufacturer Association",
		"stock-in-hand-stock-quantity-format":              "Reports > Stock Reports > Stock In hand > Stock Quantity Format",
		"stock-in-hand-stock-in-hand-audit-purpose":        "Reports > Stock Reports > Stock In hand > Stock in Hand - Audit Purpose",
		"stock-in-hand-batch-priority-wise-audit-purposes": "Reports > Stock Reports > Stock In hand > Batch, Priority Wise - Audit Purposes",
		"expiry-report":                                    "Reports > Stock Reports > Expiry Report",
		"reorder-level-report":                             "Reports > Stock Reports > Reorder Level Report",
		"stock-register":                                   "Reports > Stock Reports > Stock Register",
		"item-stock-register-summary":                      "Reports > Stock Reports > Item Stock Register Summary",
		"stock-register-for-narcotics":                     "Reports > Stock Reports > Stock Register(For Narcotics)",
		"stock-and-sales":                                  "Reports > Stock Reports > Stock and Sales",
		"optimum-level-report":                             "Reports > Stock Reports > Optimum Level Report",
		"item-activity":                                    "Reports > Stock Reports > Item Activity",
		"stock-register-narcotics-format2":                 "Reports > Stock Reports > Stock Register(Narcotics Format2)",
		"expiry-report-class-wise":                         "Reports > Stock Reports > Expiry Report(Class Wise)",
		"minimum-level-report":                             "Reports > Stock Reports > Minimum Level Report",
		"reorder-optimum-level-report":                     "Reports > Stock Reports > Reorder/Optimum Level Report",
		"daily-stock-in-out":                               "Reports > Stock Reports > Daily Stock IN/OUT",
		"stock-in-out-date-wise":                           "Reports > Stock Reports > Stock IN/OUT(Date Wise)",
		"stock-management-report":                          "Reports > Stock Reports > Stock Management Report",
		"norcotics-stock-register-generic-type-wise":       "Reports > Stock Reports > Norcotics Stock Register-Generic Type Wise",
	}
	for kind, path := range paths {
		if got := reportRegistryKey("legacy-stock-leaf", path); got != kind {
			t.Errorf("reportRegistryKey(%q, %q) = %q, want %q", "legacy-stock-leaf", path, got, kind)
		}
	}
}

func TestStockReadModelUsesPostedNormalizedLedgersAndBoundedPagination(t *testing.T) {
	for _, mode := range []string{"balance", "expiry", "valuation", "movement"} {
		query := stockReadModelQuery(mode, "LIMIT $8 OFFSET $9")
		for _, fragment := range []string{
			"stock_ledger",
			"stock_batches",
			"sync_events",
			"COALESCE(NULLIF(se.payload->>'status', ''), 'posted') = 'posted'",
			"$6::uuid",
			"$7",
			"LIMIT $8 OFFSET $9",
		} {
			if !strings.Contains(query, fragment) {
				t.Errorf("%s stock query is missing %q", mode, fragment)
			}
			if mode == "movement" && !strings.Contains(query, "l.tenant_id = $1::uuid AND l.branch_id = $2::uuid") {
				t.Errorf("%s stock query is not tenant/branch scoped at the ledger source", mode)
			}
			if mode != "movement" && !strings.Contains(query, "sb.tenant_id = $1::uuid AND sb.branch_id = $2::uuid") {
				t.Errorf("%s stock query is not tenant/branch scoped at the balance source", mode)
			}
		}
		if mode != "movement" && !strings.Contains(query, "FROM stock_balances sb") {
			t.Errorf("%s stock query does not use normalized balances", mode)
		}
		if mode == "movement" && strings.Contains(query, "inventory_movements") {
			t.Fatal("movement report reintroduced the compatibility inventory fallback")
		}
		if mode == "expiry" && !strings.Contains(query, "expiry_date BETWEEN $3::date AND $4::date") {
			t.Fatal("expiry report does not filter typed expiry dates")
		}
		if mode == "valuation" && !strings.Contains(query, "on_hand * unit_cost") {
			t.Fatal("valuation report does not disclose its normalized valuation expression")
		}
	}
}

func TestDailySaleDetailReportDefinitionUsesRealProjectionMetadata(t *testing.T) {
	definition := reportDefinitionFor("daily-sales-detail")
	if definition.ProjectionStatus != "real" {
		t.Fatalf("projection status = %q, want real", definition.ProjectionStatus)
	}
	if definition.Title != "Daily Sales Detail" {
		t.Fatalf("title = %q, want Daily Sales Detail", definition.Title)
	}
	if len(definition.Formats) != 10 {
		t.Fatalf("formats = %d, want 10 captured formats", len(definition.Formats))
	}
	if definition.Retrieval.Title != "Specify Retrieval Arguements" {
		t.Fatalf("retrieval title = %q, want legacy spelling", definition.Retrieval.Title)
	}
	if definition.Letterhead.Source != "default" || definition.Letterhead.Phone != "055 3252501" {
		t.Fatalf("unexpected safe letterhead fallback: %+v", definition.Letterhead)
	}
	if hook := reportExport(definition, "csv"); hook == nil || hook.Status != "available" {
		t.Fatal("daily sale detail did not advertise CSV export")
	}
	if hook := reportExport(definition, "pdf"); hook == nil || hook.Status != "available" {
		t.Fatal("PDF export was not advertised as available")
	}
	if hook := reportExport(definition, "excel"); hook == nil || hook.Status != "available" {
		t.Fatal("Excel export was not advertised as available")
	}
}

func TestDailySaleDetailUsesCanonicalAndCompatibilityReadModel(t *testing.T) {
	query := salesReadModelQuery(reportSaleAggregate, "LIMIT $6 OFFSET $7")
	for _, fragment := range []string{
		"FROM business_documents bd",
		"business_document_lines bl",
		"FROM sales_documents sd",
		"FROM sync_events se",
		"bd.tenant_id = $1::uuid AND bd.branch_id = $2::uuid",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("daily sales detail query is missing %q", fragment)
		}
	}
}

func TestFallbackReportDefinitionIsExplicitAndTruthful(t *testing.T) {
	definition := reportDefinitionFor("unclassified-captured-report")
	if definition.ProjectionStatus != "generic-fallback" {
		t.Fatalf("projection status = %q, want generic-fallback", definition.ProjectionStatus)
	}
	if len(definition.Columns) == 0 || definition.Columns[0].Key != "document" {
		t.Fatalf("fallback columns do not preserve the generic row contract: %+v", definition.Columns)
	}
	if len(definition.Formats) != 1 || definition.Formats[0].Name != "Event ledger projection" {
		t.Fatalf("fallback formats = %+v, want explicit event ledger format", definition.Formats)
	}
	for _, kind := range []string{"stock", "item", "purchase-return"} {
		if reportDefinitionFor(kind).ProjectionStatus != "real" {
			t.Fatalf("existing concrete report %q was downgraded to fallback", kind)
		}
	}
}

func TestReportDefinitionAcceptsBoundedDatabaseLetterheadAndFormats(t *testing.T) {
	definition := reportDefinitionFor("daily-sales-detail")
	applyReportPreferences(&definition, map[string]map[string]string{
		"report:letterhead": {
			"name":  "Configured Pharmacy",
			"phone": "0300 1234567",
		},
		"report:format:daily-sales-detail": {
			"Configured Layout": "",
		},
	})
	if definition.Letterhead.Name != "Configured Pharmacy" || definition.Letterhead.Phone != "0300 1234567" || definition.Letterhead.Source != "database" {
		t.Fatalf("database letterhead was not applied: %+v", definition.Letterhead)
	}
	if len(definition.Formats) != 1 || definition.Formats[0].Source != "database" {
		t.Fatalf("database format list was not applied: %+v", definition.Formats)
	}
}

func reportExport(definition reportDefinition, format string) *reportExportHook {
	for index := range definition.Exports {
		if definition.Exports[index].Format == format {
			return &definition.Exports[index]
		}
	}
	return nil
}

func TestValidMasterKindCoversCapturedBasicDataLeaves(t *testing.T) {
	for _, kind := range []string{"customer-category", "sale-promotion", "supplier-category", "item-class", "item-basic-data", "sales-tax-schedule", "pct-codes", "sale-template"} {
		if !validMasterKind(kind) {
			t.Fatalf("captured master kind %q was rejected", kind)
		}
	}
	for _, kind := range []string{"../../users", "", "unknown-table"} {
		if validMasterKind(kind) {
			t.Fatalf("unsupported master kind %q was accepted", kind)
		}
	}
}
