package hardware

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	CapabilityThermalPrinter = "thermal_printer"
	CapabilityBarcodeScanner = "barcode_scanner"
	CapabilityCashDrawer     = "cash_drawer"
	CapabilityBiometric      = "biometric_reader"
	CapabilitySMS            = "sms"
	CapabilityEmail          = "email"
)

var (
	ErrAdapterUnavailable = errors.New("hardware adapter unavailable")
	ErrInvalidBarcode     = errors.New("barcode is empty or contains control characters")
)

// PrinterAdapter receives already-rendered ESC/POS bytes. Implementations are
// intentionally injected by the branch host; this package does not claim to
// have access to a physical printer.
type PrinterAdapter interface {
	Print(context.Context, []byte) error
}

// BarcodeLookupAdapter resolves a normalized scanner value against the local
// item source. HID wedge input is handled by the caller; no scanner device is
// opened by this package.
type BarcodeLookupAdapter interface {
	Lookup(context.Context, string) (BarcodeItem, error)
}

type CashDrawerAdapter interface {
	Kick(context.Context, CashDrawerKickCommand) error
}

// CashDrawerKickCommand is a device-neutral pulse request. ESC/POS-capable
// adapters may use ESC/P 0 m t1 t2; other adapters can translate the same
// intent to their transport without the edge knowing the device protocol.
type CashDrawerKickCommand struct {
	Pin     byte
	OnTime  byte
	OffTime byte
}

func DefaultCashDrawerKickCommand() CashDrawerKickCommand {
	return CashDrawerKickCommand{Pin: 0, OnTime: 25, OffTime: 250}
}

func (command CashDrawerKickCommand) ESCPosBytes() []byte {
	return []byte{0x1b, 0x70, command.Pin, command.OnTime, command.OffTime}
}

type BiometricAdapter interface {
	Verify(context.Context, []byte) (bool, error)
}

type SMSAdapter interface {
	Send(context.Context, string, string) error
}

type EmailAdapter interface {
	Send(context.Context, string, string, string) error
}

// Config is the only way adapters enter the registry. A non-nil adapter is
// considered configured, but adapter operations still determine whether the
// physical operation succeeded.
type Config struct {
	Printer    PrinterAdapter
	Barcode    BarcodeLookupAdapter
	CashDrawer CashDrawerAdapter
	Biometric  BiometricAdapter
	SMS        SMSAdapter
	Email      EmailAdapter

	PrinterProvider    string
	BarcodeProvider    string
	CashDrawerProvider string
	BiometricProvider  string
	SMSProvider        string
	EmailProvider      string
}

type Capability struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Provider  string `json:"provider"`
	Reason    string `json:"reason,omitempty"`
}

type BarcodeItem struct {
	Code   string `json:"code"`
	ItemID string `json:"itemId"`
	Name   string `json:"name"`
}

type PrintResult struct {
	Printed  bool   `json:"printed"`
	Bytes    int    `json:"bytes"`
	Provider string `json:"provider"`
}

type Registry struct {
	printer            PrinterAdapter
	barcode            BarcodeLookupAdapter
	cashDrawer         CashDrawerAdapter
	biometric          BiometricAdapter
	sms                SMSAdapter
	email              EmailAdapter
	printerProvider    string
	barcodeProvider    string
	cashDrawerProvider string
	biometricProvider  string
	smsProvider        string
	emailProvider      string
}

// New returns a registry with no physical adapters. It is deliberately safe
// for development and production startup: capabilities are explicit and
// hardware actions return ErrAdapterUnavailable rather than succeeding.
func New() *Registry {
	return NewWithConfig(Config{})
}

func NewWithConfig(config Config) *Registry {
	return &Registry{
		printer:            config.Printer,
		barcode:            config.Barcode,
		cashDrawer:         config.CashDrawer,
		biometric:          config.Biometric,
		sms:                config.SMS,
		email:              config.Email,
		printerProvider:    provider(config.PrinterProvider),
		barcodeProvider:    provider(config.BarcodeProvider),
		cashDrawerProvider: provider(config.CashDrawerProvider),
		biometricProvider:  provider(config.BiometricProvider),
		smsProvider:        provider(config.SMSProvider),
		emailProvider:      provider(config.EmailProvider),
	}
}

func provider(value string) string {
	if value == "" {
		return "configured-adapter"
	}
	return value
}

func (r *Registry) Capabilities(_ context.Context) []Capability {
	return []Capability{
		r.capability(CapabilityThermalPrinter, r.printer != nil, r.printerProvider, "No printer adapter configured"),
		r.capability(CapabilityBarcodeScanner, r.barcode != nil, r.barcodeProvider, "No barcode lookup adapter configured"),
		r.capability(CapabilityCashDrawer, r.cashDrawer != nil, r.cashDrawerProvider, "No cash-drawer adapter configured"),
		r.capability(CapabilityBiometric, r.biometric != nil, r.biometricProvider, "No biometric adapter configured"),
		r.capability(CapabilitySMS, r.sms != nil, r.smsProvider, "No SMS provider configured"),
		r.capability(CapabilityEmail, r.email != nil, r.emailProvider, "No email provider configured"),
	}
}

func (r *Registry) capability(name string, available bool, provider, reason string) Capability {
	capability := Capability{Name: name, Available: available, Provider: provider}
	if !available {
		capability.Provider = "unconfigured"
		capability.Reason = reason
	}
	return capability
}

func (r *Registry) PrintSaleSlip(ctx context.Context, slip SaleSlip) (PrintResult, error) {
	data, err := RenderSaleSlip(slip)
	if err != nil {
		return PrintResult{}, err
	}
	return r.print(ctx, data)
}

func (r *Registry) PrintPurchaseLabels(ctx context.Context, batch PurchaseLabelBatch) (PrintResult, error) {
	data, err := RenderPurchaseLabels(batch)
	if err != nil {
		return PrintResult{}, err
	}
	return r.print(ctx, data)
}

func (r *Registry) print(ctx context.Context, data []byte) (PrintResult, error) {
	if r.printer == nil {
		return PrintResult{}, ErrAdapterUnavailable
	}
	if err := r.printer.Print(ctx, data); err != nil {
		return PrintResult{}, fmt.Errorf("print ESC/POS job: %w", err)
	}
	return PrintResult{Printed: true, Bytes: len(data), Provider: r.printerProvider}, nil
}

func NormalizeBarcode(raw string) (string, error) {
	trimmed := strings.Trim(raw, " \t\r\n")
	if trimmed == "" {
		return "", ErrInvalidBarcode
	}
	for _, character := range trimmed {
		if character < 0x20 || character == 0x7f {
			return "", ErrInvalidBarcode
		}
	}
	return trimmed, nil
}

func (r *Registry) LookupBarcode(ctx context.Context, raw string) (BarcodeItem, error) {
	code, err := NormalizeBarcode(raw)
	if err != nil {
		return BarcodeItem{}, err
	}
	if r.barcode == nil {
		return BarcodeItem{}, ErrAdapterUnavailable
	}
	item, err := r.barcode.Lookup(ctx, code)
	if err != nil {
		return BarcodeItem{}, fmt.Errorf("lookup barcode %q: %w", code, err)
	}
	item.Code = code
	return item, nil
}

func (r *Registry) KickCashDrawer(ctx context.Context) error {
	if r.cashDrawer == nil {
		return ErrAdapterUnavailable
	}
	if err := r.cashDrawer.Kick(ctx, DefaultCashDrawerKickCommand()); err != nil {
		return fmt.Errorf("kick cash drawer: %w", err)
	}
	return nil
}
