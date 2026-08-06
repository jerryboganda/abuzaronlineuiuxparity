package hardware

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidPrintJob = errors.New("invalid print job")

type SaleSlipLine struct {
	ItemName string `json:"itemName"`
	Quantity string `json:"quantity"`
	Total    string `json:"total"`
}

type SaleSlip struct {
	Header        string         `json:"header"`
	Store         string         `json:"store"`
	InvoiceNumber string         `json:"invoiceNumber"`
	Date          string         `json:"date"`
	Customer      string         `json:"customer"`
	Lines         []SaleSlipLine `json:"lines"`
	Subtotal      string         `json:"subtotal"`
	Discount      string         `json:"discount"`
	Tax           string         `json:"tax"`
	Total         string         `json:"total"`
	Footer        string         `json:"footer"`
}

type PurchaseLabel struct {
	ItemName string `json:"itemName"`
	Batch    string `json:"batch"`
	Expiry   string `json:"expiry"`
	MRP      string `json:"mrp"`
	Quantity string `json:"quantity"`
}

type PurchaseLabelBatch struct {
	Labels   []PurchaseLabel `json:"labels"`
	CutAfter bool            `json:"cutAfter"`
}

const (
	escInit        = "\x1b\x40"
	escAlignLeft   = "\x1b\x61\x00"
	escAlignCenter = "\x1b\x61\x01"
	escBoldOn      = "\x1b\x45\x01"
	escBoldOff     = "\x1b\x45\x00"
	escCutPartial  = "\x1d\x56\x01"
)

// RenderSaleSlip emits a small, deterministic ESC/POS document. It performs
// formatting only; totals and tax values must be supplied by the caller's
// business workflow. The renderer never talks to a printer.
func RenderSaleSlip(slip SaleSlip) ([]byte, error) {
	if strings.TrimSpace(slip.InvoiceNumber) == "" || strings.TrimSpace(slip.Total) == "" {
		return nil, fmt.Errorf("%w: invoiceNumber and total are required", ErrInvalidPrintJob)
	}
	for _, line := range slip.Lines {
		if strings.TrimSpace(line.ItemName) == "" {
			return nil, fmt.Errorf("%w: every sale line needs an itemName", ErrInvalidPrintJob)
		}
	}

	var output bytes.Buffer
	output.WriteString(escInit)
	output.WriteString(escAlignCenter)
	if slip.Header != "" {
		output.WriteString(escBoldOn)
		writeLine(&output, slip.Header)
		output.WriteString(escBoldOff)
	}
	if slip.Store != "" {
		writeLine(&output, slip.Store)
	}
	output.WriteString(escAlignLeft)
	writeLine(&output, "Invoice: "+slip.InvoiceNumber)
	if slip.Date != "" {
		writeLine(&output, "Date: "+slip.Date)
	}
	if slip.Customer != "" {
		writeLine(&output, "Customer: "+slip.Customer)
	}
	writeLine(&output, strings.Repeat("-", 42))
	for _, line := range slip.Lines {
		writeLine(&output, line.Quantity+" x "+line.ItemName+" "+line.Total)
	}
	writeLine(&output, strings.Repeat("-", 42))
	writeAmountLine(&output, "Subtotal", slip.Subtotal)
	writeAmountLine(&output, "Discount", slip.Discount)
	writeAmountLine(&output, "Tax", slip.Tax)
	writeAmountLine(&output, "Total", slip.Total)
	if slip.Footer != "" {
		output.WriteString(escAlignCenter)
		writeLine(&output, slip.Footer)
	}
	output.WriteString(escCutPartial)
	return output.Bytes(), nil
}

// RenderPurchaseLabels emits one deterministic label block per label. A
// caller explicitly opts into a cut command because some label printers
// advance/cut differently from receipt printers.
func RenderPurchaseLabels(batch PurchaseLabelBatch) ([]byte, error) {
	if len(batch.Labels) == 0 {
		return nil, fmt.Errorf("%w: at least one purchase label is required", ErrInvalidPrintJob)
	}
	for _, label := range batch.Labels {
		if strings.TrimSpace(label.ItemName) == "" {
			return nil, fmt.Errorf("%w: every purchase label needs an itemName", ErrInvalidPrintJob)
		}
	}

	var output bytes.Buffer
	output.WriteString(escInit)
	for _, label := range batch.Labels {
		output.WriteString(escAlignCenter)
		output.WriteString(escBoldOn)
		writeLine(&output, label.ItemName)
		output.WriteString(escBoldOff)
		output.WriteString(escAlignLeft)
		writeAmountLine(&output, "Batch", label.Batch)
		writeAmountLine(&output, "Expiry", label.Expiry)
		writeAmountLine(&output, "MRP", label.MRP)
		writeAmountLine(&output, "Qty", label.Quantity)
		writeLine(&output, "")
	}
	if batch.CutAfter {
		output.WriteString(escCutPartial)
	}
	return output.Bytes(), nil
}

func writeAmountLine(output *bytes.Buffer, label, value string) {
	if value != "" {
		writeLine(output, label+": "+value)
	}
}

func writeLine(output *bytes.Buffer, value string) {
	output.WriteString(safeText(value))
	output.WriteByte('\n')
}

func safeText(value string) string {
	var output strings.Builder
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			output.WriteByte(' ')
			continue
		}
		output.WriteRune(character)
	}
	return output.String()
}
