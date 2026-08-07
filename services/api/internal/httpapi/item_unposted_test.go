package httpapi

import (
	"strings"
	"testing"
)

func TestItemUnpostedTransactionsQueryIsScopedAndBounded(t *testing.T) {
	query := itemUnpostedTransactionsQuery()
	for _, fragment := range []string{
		"FROM business_documents d",
		"JOIN business_document_lines l",
		"d.tenant_id = $1::uuid",
		"d.branch_id = $2::uuid",
		"l.item_id = $3::uuid",
		"d.status = 'draft'",
		"d.deleted_at IS NULL",
		"ORDER BY d.occurred_at DESC",
		"LIMIT $4",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("unposted transaction query is missing %q", fragment)
		}
	}
}
