package hardware

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryReportsExplicitUnavailableAdapters(t *testing.T) {
	capabilities := New().Capabilities(context.Background())
	if len(capabilities) < 6 {
		t.Fatalf("capability count = %d, want all integration categories", len(capabilities))
	}
	for _, capability := range capabilities {
		if capability.Name == "" || capability.Provider == "" || capability.Available {
			t.Fatalf("invalid capability: %+v", capability)
		}
	}
}

func TestRegistryReadinessIsExplicitWhenNoAdaptersAreConfigured(t *testing.T) {
	readiness := New().Readiness(context.Background())
	if readiness.Ready || readiness.Status != "unavailable" || !readiness.ConfigurationValid {
		t.Fatalf("readiness = %+v, want valid unavailable state", readiness)
	}
	if readiness.AvailableCount != 0 || readiness.TotalCount != 6 || len(readiness.Unavailable) != 6 {
		t.Fatalf("readiness counts = %+v, want 0/6 and six unavailable capabilities", readiness)
	}
}

func TestValidateConfigRejectsAmbiguousAdapterConfiguration(t *testing.T) {
	var typedNilPrinter *testPrinter
	for _, config := range []Config{
		{PrinterProvider: "printer-without-adapter"},
		{PrinterProvider: " printer"},
		{PrinterProvider: "printer\n"},
		{Printer: typedNilPrinter, PrinterProvider: "typed-nil-printer"},
	} {
		if err := ValidateConfig(config); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("ValidateConfig(%+v) = %v, want invalid configuration", config, err)
		}
	}
	if err := ValidateConfig(Config{Printer: &testPrinter{}}); err != nil {
		t.Fatalf("valid injected adapter rejected: %v", err)
	}
}

func TestRegistryUsesInjectedAdaptersWithoutClaimingUnavailableHardware(t *testing.T) {
	printer := &testPrinter{}
	barcode := testBarcodeLookup{}
	drawer := &testDrawer{}
	registry := NewWithConfig(Config{
		Printer:            printer,
		Barcode:            barcode,
		CashDrawer:         drawer,
		PrinterProvider:    "test-printer",
		BarcodeProvider:    "test-lookup",
		CashDrawerProvider: "test-drawer",
	})

	capabilities := registry.Capabilities(context.Background())
	if !capabilityAvailable(capabilities, CapabilityThermalPrinter, "test-printer") ||
		!capabilityAvailable(capabilities, CapabilityBarcodeScanner, "test-lookup") ||
		!capabilityAvailable(capabilities, CapabilityCashDrawer, "test-drawer") {
		t.Fatalf("injected capabilities = %+v", capabilities)
	}

	result, err := registry.PrintSaleSlip(context.Background(), SaleSlip{InvoiceNumber: "1", Total: "1.00"})
	if err != nil || !result.Printed || printer.prints != 1 {
		t.Fatalf("print result = %+v, err = %v, prints = %d", result, err, printer.prints)
	}
	item, err := registry.LookupBarcode(context.Background(), " 890123\r\n")
	if err != nil || item.Code != "890123" || item.ItemID != "item-1" {
		t.Fatalf("lookup result = %+v, err = %v", item, err)
	}
	if err := registry.KickCashDrawer(context.Background()); err != nil || drawer.kicks != 1 {
		t.Fatalf("kick err = %v, kicks = %d", err, drawer.kicks)
	}
	wantKick := []byte{0x1b, 0x70, 0x00, 0x19, 0xfa}
	if string(drawer.command.ESCPosBytes()) != string(wantKick) {
		t.Fatalf("kick command bytes = %x, want %x", drawer.command.ESCPosBytes(), wantKick)
	}
}

func TestNormalizeBarcodeTrimsScannerWhitespace(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want string
	}{
		{raw: " 890123\r\n", want: "890123"},
		{raw: "ABC-123", want: "ABC-123"},
	} {
		got, err := NormalizeBarcode(test.raw)
		if err != nil || got != test.want {
			t.Fatalf("NormalizeBarcode(%q) = %q, %v; want %q", test.raw, got, err, test.want)
		}
	}
	if _, err := NormalizeBarcode("ABC\x1d123"); err != ErrInvalidBarcode {
		t.Fatalf("control character error = %v", err)
	}
}

type testPrinter struct{ prints int }

func (p *testPrinter) Print(context.Context, []byte) error {
	p.prints++
	return nil
}

type testBarcodeLookup struct{}

func (testBarcodeLookup) Lookup(_ context.Context, code string) (BarcodeItem, error) {
	return BarcodeItem{Code: code, ItemID: "item-1", Name: "Test item"}, nil
}

type testDrawer struct {
	kicks   int
	command CashDrawerKickCommand
}

func (d *testDrawer) Kick(_ context.Context, command CashDrawerKickCommand) error {
	d.kicks++
	d.command = command
	return nil
}

func capabilityAvailable(capabilities []Capability, name, provider string) bool {
	for _, capability := range capabilities {
		if capability.Name == name {
			return capability.Available && capability.Provider == provider && capability.Reason == ""
		}
	}
	return false
}
