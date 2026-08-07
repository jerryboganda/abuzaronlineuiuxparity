package hardware

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
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
	ErrAdapterUnavailable   = errors.New("hardware adapter unavailable")
	ErrInvalidConfiguration = errors.New("invalid hardware adapter configuration")
	ErrInvalidBarcode       = errors.New("barcode is empty or contains control characters")
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

type Readiness struct {
	Ready              bool         `json:"ready"`
	Status             string       `json:"status"`
	ConfigurationValid bool         `json:"configurationValid"`
	ConfigurationError string       `json:"configurationError,omitempty"`
	AvailableCount     int          `json:"availableCount"`
	TotalCount         int          `json:"totalCount"`
	Unavailable        []string     `json:"unavailable"`
	Capabilities       []Capability `json:"capabilities"`
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
	configurationError error
}

// New returns a registry with no physical adapters. It is deliberately safe
// for development and production startup: capabilities are explicit and
// hardware actions return ErrAdapterUnavailable rather than succeeding.
func New() *Registry {
	return NewWithConfig(Config{})
}

func NewWithConfig(config Config) *Registry {
	registry := &Registry{
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
	if err := ValidateConfig(config); err != nil {
		registry.configurationError = err
	}
	if !adapterPresent(config.Printer) {
		registry.printer = nil
	}
	if !adapterPresent(config.Barcode) {
		registry.barcode = nil
	}
	if !adapterPresent(config.CashDrawer) {
		registry.cashDrawer = nil
	}
	if !adapterPresent(config.Biometric) {
		registry.biometric = nil
	}
	if !adapterPresent(config.SMS) {
		registry.sms = nil
	}
	if !adapterPresent(config.Email) {
		registry.email = nil
	}
	return registry
}

func ValidateConfig(config Config) error {
	checks := []struct {
		name     string
		adapter  any
		provider string
	}{
		{name: CapabilityThermalPrinter, adapter: config.Printer, provider: config.PrinterProvider},
		{name: CapabilityBarcodeScanner, adapter: config.Barcode, provider: config.BarcodeProvider},
		{name: CapabilityCashDrawer, adapter: config.CashDrawer, provider: config.CashDrawerProvider},
		{name: CapabilityBiometric, adapter: config.Biometric, provider: config.BiometricProvider},
		{name: CapabilitySMS, adapter: config.SMS, provider: config.SMSProvider},
		{name: CapabilityEmail, adapter: config.Email, provider: config.EmailProvider},
	}
	for _, check := range checks {
		if !adapterPresent(check.adapter) && check.provider != "" {
			return fmt.Errorf("%w: %s provider is set without an adapter", ErrInvalidConfiguration, check.name)
		}
		if strings.TrimSpace(check.provider) != check.provider ||
			strings.IndexFunc(check.provider, unicode.IsControl) >= 0 {
			return fmt.Errorf("%w: %s provider contains surrounding whitespace or control characters", ErrInvalidConfiguration, check.name)
		}
	}
	return nil
}

func adapterPresent(adapter any) bool {
	if adapter == nil {
		return false
	}
	value := reflect.ValueOf(adapter)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}

func provider(value string) string {
	if value == "" {
		return "configured-adapter"
	}
	return value
}

func (r *Registry) Capabilities(_ context.Context) []Capability {
	capabilities := []Capability{
		r.capability(CapabilityThermalPrinter, r.printer != nil, r.printerProvider, "No printer adapter configured"),
		r.capability(CapabilityBarcodeScanner, r.barcode != nil, r.barcodeProvider, "No barcode lookup adapter configured"),
		r.capability(CapabilityCashDrawer, r.cashDrawer != nil, r.cashDrawerProvider, "No cash-drawer adapter configured"),
		r.capability(CapabilityBiometric, r.biometric != nil, r.biometricProvider, "No biometric adapter configured"),
		r.capability(CapabilitySMS, r.sms != nil, r.smsProvider, "No SMS provider configured"),
		r.capability(CapabilityEmail, r.email != nil, r.emailProvider, "No email provider configured"),
	}
	if r.configurationError != nil {
		for index := range capabilities {
			capabilities[index].Available = false
			capabilities[index].Provider = "unconfigured"
			capabilities[index].Reason = "Invalid hardware configuration: " + r.configurationError.Error()
		}
	}
	return capabilities
}

func (r *Registry) Readiness(ctx context.Context) Readiness {
	capabilities := r.Capabilities(ctx)
	unavailable := make([]string, 0, len(capabilities))
	availableCount := 0
	for _, capability := range capabilities {
		if capability.Available {
			availableCount++
			continue
		}
		unavailable = append(unavailable, capability.Name)
	}
	status := "ready"
	if len(unavailable) == len(capabilities) {
		status = "unavailable"
	} else if len(unavailable) > 0 {
		status = "degraded"
	}
	if r.configurationError != nil {
		status = "invalid_configuration"
	}
	return Readiness{
		Ready:              r.configurationError == nil && len(unavailable) == 0,
		Status:             status,
		ConfigurationValid: r.configurationError == nil,
		ConfigurationError: configurationErrorText(r.configurationError),
		AvailableCount:     availableCount,
		TotalCount:         len(capabilities),
		Unavailable:        unavailable,
		Capabilities:       capabilities,
	}
}

func configurationErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
	if r.configurationError != nil {
		return PrintResult{}, r.configurationError
	}
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
	if r.configurationError != nil {
		return BarcodeItem{}, r.configurationError
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
	if r.configurationError != nil {
		return r.configurationError
	}
	if r.cashDrawer == nil {
		return ErrAdapterUnavailable
	}
	if err := r.cashDrawer.Kick(ctx, DefaultCashDrawerKickCommand()); err != nil {
		return fmt.Errorf("kick cash drawer: %w", err)
	}
	return nil
}
