package httpapi

import "testing"

func TestParseVoucherPostRequestRejectsUnbalancedJournal(t *testing.T) {
	_, err := parseVoucherPostRequest([]byte(`{
		"categoryCode":"journal",
		"occurredAt":"2026-08-09T00:00:00Z",
		"lines":[
			{"accountId":"00000000-0000-0000-0000-000000000001","debit":"10","credit":"0"},
			{"accountId":"00000000-0000-0000-0000-000000000002","debit":"0","credit":"9"}
		]
	}`))
	if err == nil || err.Error() == "" {
		t.Fatal("unbalanced journal was accepted")
	}
}

func TestParseVoucherPostRequestRequiresPartyForReceipt(t *testing.T) {
	_, err := parseVoucherPostRequest([]byte(`{"categoryCode":"receipt","amount":"10.00"}`))
	if err == nil {
		t.Fatal("receipt without partyId was accepted")
	}
}

func TestVoucherDocumentKindMapping(t *testing.T) {
	if voucherDocumentKind("receipt") != "receipt-voucher" {
		t.Fatal("receipt kind mapping")
	}
	if voucherDocumentKind("payment") != "payment-voucher" {
		t.Fatal("payment kind mapping")
	}
	if voucherDocumentKind("journal") != "journal-voucher" {
		t.Fatal("journal kind mapping")
	}
	if voucherDocumentKind("other") != "" {
		t.Fatal("unknown category should be empty")
	}
}
