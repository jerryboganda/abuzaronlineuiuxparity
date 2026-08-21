package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReviewedPreferenceRegistryCoversCapturedTabsAndFields(t *testing.T) {
	registry := reviewedPreferenceRegistry()
	if len(registry) < 200 {
		t.Fatalf("reviewed preference registry has %d fields, want at least 200", len(registry))
	}
	seenCategories := make(map[string]bool)
	seenKeys := make(map[string]bool)
	seenStorageKeys := make(map[string]bool)
	for _, definition := range registry {
		seenCategories[definition.Category] = true
		key := definition.Category + "\x00" + definition.FieldKey
		if seenKeys[key] {
			t.Fatalf("duplicate preference field_key %q", key)
		}
		seenKeys[key] = true
		storageKey := definition.Category + "\x00" + definition.StorageCaption
		if seenStorageKeys[storageKey] {
			t.Fatalf("duplicate preference storage key %q", storageKey)
		}
		seenStorageKeys[storageKey] = true
		if definition.Caption == "" || definition.FieldKey == "" || definition.StorageCaption == "" || definition.Behavior == "" || definition.RuntimeStatus == "" {
			t.Fatalf("incomplete preference definition: %+v", definition)
		}
	}
	if len(seenCategories) != 17 {
		t.Fatalf("registry categories = %d, want 17", len(seenCategories))
	}
	schedule := preferenceDefinitionMap("Schedule")
	first, firstOK := schedule["schedule.activate.1"]
	second, secondOK := schedule["schedule.activate.2"]
	if !firstOK || !secondOK || first.FieldKey == second.FieldKey {
		t.Fatal("repeated Schedule Activate fields collapsed to one field key")
	}
}

func TestPreferenceValidationRejectsUnsafeTypedValues(t *testing.T) {
	definitions := preferenceDefinitionMap("General")
	if err := preferenceValidationError(definitions["Enable Alias Name:"], "maybe"); err == nil {
		t.Fatal("invalid boolean was accepted")
	}
	if err := preferenceValidationError(definitions["Max. Allowed Days:"], "not-a-number"); err == nil {
		t.Fatal("invalid integer was accepted")
	}
	if err := preferenceValidationError(definitions["Enable Alias Name:"], "Yes"); err != nil {
		t.Fatalf("valid boolean rejected: %v", err)
	}
}

func TestSchedulePreferenceIsExplicitlyNotConfigured(t *testing.T) {
	for _, definition := range reviewedPreferenceRegistry() {
		if definition.Category != "Schedule" {
			continue
		}
		if definition.RuntimeStatus != "not_configured" {
			t.Fatalf("schedule field %q status = %q", definition.Caption, definition.RuntimeStatus)
		}
		if !strings.Contains(strings.ToLower(definition.Behavior), "not configured") {
			t.Fatalf("schedule field %q does not document divergence: %q", definition.Caption, definition.Behavior)
		}
	}
	divergences := preferenceDivergences()
	if len(divergences) == 0 || divergences[0].Status != "not_configured" {
		t.Fatalf("schedule divergence contract = %+v", divergences)
	}
}

func TestPreferenceRuntimeStatusOnlyClaimsProvenBehavior(t *testing.T) {
	for _, definition := range reviewedPreferenceRegistry() {
		if definition.RuntimeStatus != "wired" {
			continue
		}
		key := definition.Category + "/" + definition.Caption
		switch key {
		case "Report/Default Header On Report:",
			"Sale/Check Cr Limit In Cr Sales:",
			"Sale/Price # in Cash Sale:",
			"Sale/Price # in Credit Sale:",
			"Sale/Allow Zero Retail Price:",
			"General/Default Batch:",
			"General/Default Expiry:",
			"General/Max. Allowed Days:",
			"General/Prompt Before Printing:",
			"General/Ask No. of copies in print dialog:",
			"Sale/Show Agency:",
			"Sale/Show Vehicle:",
			"Sale/Show Ship To:",
			"Sale/Show Associated Purchase Inv. Code:",
			"Sale/Show Supplier Inv. Code:",
			"Sale/Show GRN:",
			"Sale/Show Guarantee Person:",
			"Sale/Show Sale Type:",
			"Sale/Show Item Image/Photo:",
			"Purchase/Show Agency:",
			"Purchase/Show Vehicle:",
			"Purchase/Show Ship To:",
			"Purchase/Show Associated Sale Inv. Code:",
			"Purchase/Show Dept:",
			"Purchase/Show Supplier Date:",
			"Purchase/Show Supplier Amount:",
			"Purchase/Show GRN No.:",
			"Purchase/Show Item Image/Photo:",
			"General/Application Title:",
			"Purchase Return/Ask No. of Copies to Print:",
			"Quotation/Allow Quotation On Zero Price:",
			"Quotation/Show P/O Rate:",
			"Quotation/Show P/O Discount (%):",
			"Quotation/Show Special Rate:",
			"Sale Return/Show Account for Sale Return:",
			"Sale Return/Ask Amount Paid on S/Return Saving:",
			"Purchase Return/Ask Amount Received on P/Return Saving:",
			"Purchase/Ask Purchase Order:",
			"Purchase/Ask Credit Days:",
			"Purchase/Ask L. C. No.:",
			"Purchase/Ask New Purchase Order No.:",
			"Purchase Return/Ask Pur. Invoice No.:",
			"Purchase/Show Net Rate:",
			"Report/Apply Default Date for Report Arg. Window?",
			"Report/Default Start Date:",
			"Report/Show Account in Reports:",
			"Quotation/Delivery Days:",
			"Quotation/Validity Days:",
			"Quotation/Payment To:",
			"Sale/Reference No. 2:",
			"Sale/Reference No. 3:",
			"Sale/Reference No. 4:",
			"Sale/Sales Person:",
			"Sale/Account For:",
			"Sale/Message:",
			"Sale/Doctor:",
			"Sale Return/Copy Remarks in Item Description:",
			"Purchase Return/Copy Remarks in Item Description:",
			"Purchase Order/Copy Remarks in Item Description",
			"Purchase Order/Show Supplier Reference:",
			"Purchase Order/Show Delivery Place:",
			"Purchase Order/Show Usage Palace:",
			"Purchase Order/Show Required Date:",
			"Purchase Order/Show Remarks 2:",
			"Purchase Order/Show Purchase Type:",
			"Purchase Order/Show Remarks 3:",
			"Report/Refresh Time(Minutes):",
			"Quotation/Line1:",
			"Quotation/Line2:",
			"Quotation/Line3:",
			"Quotation/Line4:",
			"Quotation/Line5:",
			"Quotation/Line6:",
			"Quotation/Line7:",
			"Quotation/Line8:",
			"Purchase Order/Line1:",
			"Purchase Order/Line2:",
			"Purchase Order/Line3:",
			"Purchase Order/Line4:",
			"Purchase Order/Line5:",
			"Purchase Order/Line6:",
			"Purchase Order/Line7:",
			"Purchase Order/Line8:",
			"Purchase Order/Purchase Order Footer:",
			"Report/Report Term 1:",
			"Report/Report Term 2:",
			"Report/Report Term 3:",
			"Report/Report Term 4:",
			"Report/Report Term 5:",
			"Report/Report Term 6:",
			"BasicData/Lock Item Sale Price:",
			"BasicData/Lock Item Disc. (%):",
			"Sale/Loyalty Points:",
			"Sale/Currency:",
			"Sale/Motor Vehicle:",
			"Sale Return/Sale Return Account For:",
			"Purchase/Account For:",
			"Purchase/Show Unit Weight:",
			"Purchase/Show Total Weight:",
			"Sale/Ask Header:",
			"Sale/Customer Balance:",
			"Quotation/Ask Header:",
			"Quotation/Show Ref. No.:",
			"Purchase/Ask Header:",
			"Purchase/Ask Purchase Type:",
			"Purchase Return/Account For:",
			"Purchase Order/Show Misc.Charges:",
			"Purchase Order/Show Invoice Discount (%):",
			"Purchase Order/Show Invoice GST (%):",
			"Purchase Order/Show Invoice Flat Discount:",
			"Purchase Order/Show Grand Total:",
			"Purchase Order/Show Item Remarks/Description:",
			"Purchase/Supplier Balance:",
			"Purchase Return/Header:",
			"Sale Return/Header On Sale Return:",
			"Quotation/Manufacturer Name:",
			"Quotation/Color:",
			"Quotation/Quantity Denomination:",
			"Purchase Order/Show Item Sale Tax:",
			"Purchase Order/Show Item GST %:",
			"Purchase Order/Show Item Flat Discount:",
			"Purchase Order/Show Disc. Perc. 2.:",
			"Sale Return/Payment Mode Amt. Paid in S/Return:",
			"Sale Return/Payment A/C for Amt. Paid in S/Return:",
			"Sale Return/Round Item Total (decimal places):",
			"Sale/Print Warranted Invoice:",
			"Quotation/Pack Units:",
			"Purchase Return/Payment Mode Amt. Received in P/Return:",
			"Purchase Return/Payment A/C for Amt. Received in P/Return:",
			"Purchase Return/Round Item Total (decimal places):",
			"Purchase Return/Show Pack Qty.:",
			"Purchase Order/Default Purchase Order Category",
			"Sale/Fiscalization Machine IP:",
			"Sale/Item Disc. %:",
			"Sale/Item GST %:",
			"Sale/Extra Tax %:",
			"Sale/Disc. % On Cash Sale:",
			"Sale/Disc. % On Credit Sale:",
			"Sale/Pre-Discount %age:",
			"Sale/Claimable Disc.%:",
			"Purchase/Disc. %:",
			"Purchase/Item Level GST %:",
			"Purchase/Extra Tax %:",
			"Purchase/Sale Disc. %:",
			"Purchase/Margin%:",
			"Purchase/Net Margin %:",
			"Purchase/PCT Code:",
			"Purchase/Sales Tax Schedule:",
			"Quotation/Claimable Discount %:",
			"Sale/Alternate Alias Name:",
			"Sale/Item Flat Discount:",
			"Sale/Bonus Qty:",
			"Sale/Item Unit Sales Tax:",
			"Sale/Claimable Item:",
			"Sale/Item Packing:",
			"Sale/Packing Description:",
			"Sale/Item Description:",
			"Sale/Batch No:",
			"Sale/Item Weight Per Unit:",
			"Sale/Packing Factor Per Unit:",
			"Sale Return/Page Size:",
			"Sale Return/Sale Return Page Size:",
			"Sale Return/Alternate Alias Name:",
			"Sale Return/Packing Description:",
			"Sale Return/Item Description:",
			"Sale Return/Total Pieces:",
			"Sale Return/Thermal Print Format:",
			"Purchase/Alternate Alias Name:",
			"Purchase/Pre-Disc. Price:",
			"Purchase/Flat Discount:",
			"Purchase/Packing Description:",
			"Purchase/P/O Qty:",
			"Purchase/Bonus Quantity:",
			"Purchase/Item Description:",
			"Purchase/Sales Tax:",
			"Purchase/Pur. Tax:",
			"Purchase/Total Pieces:",
			"Purchase/Mark-up:",
			"Purchase/Manufacturer:",
			"Purchase/Sale Return Basis:",
			"Purchase Return/Alternate Alias Name:",
			"Purchase Return/Packing Description:",
			"Purchase Return/Item Description:",
			"Purchase Return/Total Pieces:",
			"Purchase Return/Price in Purchase Return:",
			"Quotation/Remote Net Sale Qty:",
			"Quotation/Remote Stock in Hand:",
			"Quotation/Remote Re-Order Qty:",
			"Quotation/Remote Optimum Qty:",
			"Quotation/Remote Min Qty:",
			"Quotation/Remote P/O Qty:",
			"Purchase Order/Item Alert:",
			"Purchase Order/Required Packs Fraction Treatment:",
			"Purchase Order/Page Size for print out:",
			"Purchase Order/P/O Supplier Consideration:",
			"General/Business Short Name:":
			continue
		default:
			t.Fatalf("unreviewed preference marked wired: %s/%s", definition.Category, definition.Caption)
		}
	}
}

func TestPreferencePermissionIsFailClosed(t *testing.T) {
	server := &Server{}
	request := preferenceTestRequest("GET", "/v1/preferences?category=General", &sessionContext{TenantID: "tenant-a"})
	recorder := httptest.NewRecorder()
	server.preferences(recorder, request)
	if recorder.Code != 403 {
		t.Fatalf("preference permission status = %d, want 403", recorder.Code)
	}
}

func preferenceTestRequest(method, target string, operator *sessionContext) *http.Request {
	return httptest.NewRequest(method, target, nil).WithContext(context.WithValue(context.Background(), sessionContextKey, operator))
}
