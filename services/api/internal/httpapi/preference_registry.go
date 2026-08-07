package httpapi

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// preferenceValueType is deliberately small. The legacy grid stores text, but
// keeping the reviewed type and validation contract here prevents a malformed
// value from becoming an implicit application setting.
type preferenceValueType string

const (
	preferenceText    preferenceValueType = "text"
	preferenceBoolean preferenceValueType = "boolean"
	preferenceInteger preferenceValueType = "integer"
	preferenceDecimal preferenceValueType = "decimal"
	preferenceDate    preferenceValueType = "date"
	preferenceTime    preferenceValueType = "time"
	preferenceSecret  preferenceValueType = "secret"
	preferenceEnum    preferenceValueType = "enum"
)

type preferenceDefinition struct {
	Category       string
	Caption        string
	FieldKey       string
	StorageCaption string
	Type           preferenceValueType
	Default        string
	Allowed        []string
	Minimum        *float64
	Maximum        *float64
	Behavior       string
	RuntimeStatus  string
	Position       int
}

type preferenceDivergence struct {
	Category string `json:"category"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
}

var (
	registryOnce sync.Once
	registry     []preferenceDefinition
)

// This is the reviewed inventory from the 17 captured legacy tabs. Section
// headings are presentation-only and are intentionally not persisted.
var preferenceCatalog = map[string]string{
	"General": `
Inventory System:
Inventory Movement Method:
Default Batch:
Enable Alias Name:
Search Item Code if Alias Name Not Found:
Default Expiry:
Prompt Before Printing:
Ask No. of copies in print dialog:
Max. Allowed Days:
Business Short Name:
Application Title:
Allow Login A User Multiple Times:
Auto Responsive Search With Alternate Alias Name:
Item Name:
Alias Name:
Local Item Name:
Manufacturer:
Sale Price:
Packing:
Pack Units:
Stock:
Pack Stock
Generic Item Name
Packing Description:
Item Location:
Transit Stock:
Item Alert:
In Sale/Return:
In Basic Data:
In Accounts Module:
In Purchase/Purchase Return:
In Purchase/Purchase Return Services:
In Accounts:
In Basic Data:
In Basic Data:
In Sale/Return:
Enable Quick Search:
Quick Search Type:
Check Manual Backup Health At StartUp:
Age of Item Changes For Data Carry (in Days):`,
	"Sale": `
Cash Sale Initial Focus:
Credit Sale Initial Focus:
Fiscalization Machine IP:
Allow Zero Retail Price:
Customer Balance:
Reference No. 2:
Reference No. 3:
Reference No. 4:
Sales Person:
Account For:
Ask Header:
Price # in Cash Sale:
Price # in Credit Sale:
Item Disc. %:
Message:
Doctor:
Show Agency:
Show Vehicle:
Show Ship To:
Show Associated Purchase Inv. Code:
Show Supplier Inv. Code:
Show GRN:
Show Guarantee Person:
Show Sale Type:
Show Item Image/Photo:
Loyalty Points:
Currency:
Motor Vehicle:
Alternate Alias Name:
Disc. % On Cash Sale:
Pre-Discount %age:
Item Flat Discount:
Bonus Qty:
Item Unit Sales Tax:
Disc. % On Credit Sale:
Claimable Disc.%:
Claimable Item:
Item GST %:
Location:
Item Packing:
Packing Description:
Extra Tax %:
Item Description:
Batch No:
Purchase Price:
Item Weight Per Unit:
Packing Factor Per Unit:
Print Warranted Invoice:`,
	"Sale Return": `
Show Account for Sale Return:
Header On Sale Return:
Page Size:
Alternate Alias Name:
Packing Description:
Batch:
Expiry:
Item Description:
Total Pieces:
Sale Return Page Size:
Thermal Print Format:
Sale Return Account For:
Ask Amount Paid on S/Return Saving:
Payment Mode Amt. Paid in S/Return:
Payment A/C for Amt. Paid in S/Return:
Fetch Latest Item Sales Info.:
Update Avg. Price:
Auto Locate Referenced Sale Return:
Auto Post Sale Return Allocation:
Ask User/Password in Sale Return:
Round Item Total (decimal places):
Show Sale Return in Activity Monitor:
Copy Remarks in Item Description:
Allow Empty Sale Inv # in Unposted S/R Module:
Show Master Cashier Window in Cash S/R:
Show Master Cashier Window in Credit S/R:
Show Master Cashier Window in In-Patient S/R:
Show Master Cashier Window in Buffer S/R:
Show S/R in Cash Activity Window:
Allow Same Batch Return Multiple Times:
Fetch FBR POS Fee For Reference S/R:`,
	"Purchase": `
Ask Header:
Ask Purchase Type:
Ask Purchase Order:
Account For:
Supplier Balance:
Sale Return Basis:
Show Agency:
Show Vehicle:
Show Ship To:
Show Associated Sale Inv. Code:
Show Dept:
Show Supplier Date:
Show Supplier Amount:
Show GRN No.:
Ask New Purchase Order No.:
Show Supplier Inv.#:
Ask L. C. No.:
Show Item Image/Photo:
Ask Credit Days:
Alternate Alias Name:
Pack Units:
Batch Number:
Mfg. Date:
Expiry Date:
Pre-Disc. Price:
Flat Discount:
Purchase Price:
Disc. %:
Packing Description:
P/O Qty:
Bonus Quantity:
Item Description:
Sales Tax:
Pur. Tax:
Item Level GST %:
Item Location:
Show/Update Retail Price:
Show Net Rate:
Total Pieces:
Mark-up:
Margin%:
Net Margin %:
Manufacturer:
Show Unit Weight:
Show Total Weight:
Extra Tax %:
Batch Sale Price:
Sale Disc. %:
Sales Tax Schedule:
PCT Code:`,
	"Purchase Return": `
Header:
Account For:
Ask Pur. Invoice No.:
Alternate Alias Name:
Packing Description:
Item Description:
Total Pieces:
Show Expiry:
Show Pack Qty.:
Show Purchase Batch:
Auto Post:
Ask Amount Received on P/Return Saving:
Payment Mode Amt. Received in P/Return:
Payment A/C for Amt. Received in P/Return:
Price in Purchase Return:
Round Item Total (decimal places):
Ask No. of Copies to Print:
Show P/Return in Activity Monitor:
Allow Empty Pur. Invoice No.:
Copy Remarks in Item Description:
Update Avg. Price On P/R:
Allow P/R Below Avg. Price:`,
	"Report": `
Refresh Time(Minutes):
Default Header On Report:
Apply Default Date for Report Arg. Window?
Default Start Date:
Allow Print Setup:
Show Account in Reports:
Report Term 1:
Report Term 2:
Report Term 3:
Report Term 4:
Report Term 5:
Report Term 6:`,
	"BasicData": `
Manufacturer:
Item Category:
Item Class:
Item Packing:
Associated Godown:
Lock Item Disc. (%):
Lock Item Sale Price:
Allow Due Default Value in Item Basic Data:
Ask User/Password In Item:
Ask User/Password In Patient Registration:
Sales Person Scope For Sub Area:`,
	"Quotation": `
Ask Header:
Delivery Days:
Validity Days:
Payment To:
Remote Net Sale Qty:
Remote Stock in Hand:
Remote Re-Order Qty:
Remote Optimum Qty:
Remote Min Qty:
Remote P/O Qty:
Manufacturer Name:
Pack Units:
Show P/O Rate:
Show P/O Discount (%):
Show Special Rate:
Show Ref. No.:
Claimable Discount %:
Color:
Quantity Denomination:
Show Quotation in Activity Monitor:
Allow Duplicate Items:
Allow Quotation On Zero Price:
Fetch Latest Sale Price:
Allow Quotation Below Avg. Price:
Allow Quotation Below RPP:
Allow Quotation Above Avg. Price:
Drop Box - Auto Fetch Prices Of Branch P/O:
Drop Box - Auto Fetch Discounts Of Branch P/O:
Line1:
Line2:
Line3:
Line4:
Line5:
Line6:
Line7:
Line8:`,
	"Schedule": `
Activate:
Once At:
Hour:
Minute:
Second:
Every:
Activate:
Post Cash Sale Invoices:
Minutes old:
Occurs Every:
Activate:
Post Credit Sale Invoices:
Minutes old:
Occurs Every:
Activate:
Once At:
Hour:
Minute:
Second:
Every:
Activate:
Once At:
Hour:
Minute:
Second:
Every:
Activate:
Once a day at:`,
	"Adjustment": `
Show Header:
Show Update Avg. Price Column:
Adjustment Qty:
Alternate Alias Name:
Show Batch:
Show Expiry:
Update Avg. Price:
Ask User/Password in Adjustment:
In Adj. Buffer, Pending Due Effect:
Show Adjustments in Activity Monitor:`,
	"Purchase Order": `
Show Supplier Reference:
Show Delivery Place:
Show Usage Palace:
Show Required Date:
Show Remarks 2:
Show Purchase Type:
Show Remarks 3:
Show Item Remarks/Description:
Show Item Sale Tax:
Show Item GST %:
Show Item Flat Discount:
Show Disc. Perc. 2.:
Item Alert:
Show Invoice Discount (%):
Show Misc.Charges:
Show Invoice GST (%):
Show Invoice Flat Discount:
Show Grand Total:
Required Packs Fraction Treatment:
Auto Update Transit Stock at Posting:
Update Re-Order Qty at Saving:
Page Size for print out:
Apply Supplier Items Only Option
Default Purchase Order Category
Update Minimum Qty at Saving:
Apply Associated Quotation On Save
Update Optimum Qty at Saving:
Show Pur. Order in Activity Monitor:
Show Un-Posted P/Ret. on P/O Saving:
Copy Remarks in Item Description
Enforce Supplier/Item Association On P/O saving:
Fetch Last Purchase Info. in P/O:
P/O Supplier Consideration:
Apply Supplier Associated Quotation:
Consider Issue Qty:
Consider Receipt Qty:
Line1:
Line2:
Line3:
Line4:
Line5:
Line6:
Line7:
Line8:
Purchase Order Footer:`,
	"Others": `
Preferred Printer for Activity Monitor:
SMS Expiry (in Hours):
Refresh Time (Seconds):
Activity Period (Minutes):
Mfg. Distributor Code:
Distributor Code:
Retail Price(%) for Sale Inv.:
Output File(s) Location :
Default Location for [Save As PDF]:
Synchronize Parent Server Stock:
Keep Child Server Invoice No Intact:
Copy Child Server Invoice No in Ref # 1:
Prepare Purchase Dump Before Data Migration:
Prepare Sales Dump Before Data Migration:
Save Sale History Before Data Migration:
Auto Purge Posted Sale Time(Minutes)
Select Image Scan Tool:`,
	"Point of Sale": `
Invoice Size:
Sales Person:
Loyalty Points:
Delivered By:
Item Discount %:
Item Flat Discount:
Item GST %:
Item Unit Sales Tax:
Invoice GST %:
Misc. Charges:
Invoice Discount %:
Invoice Flat Discount:
Prompt For Zero Stock:
Sale Sets/Deals In POS:
Auto Production In POS:
Ask Associate Sale Invoices on Saving:
Check Due Date on Saving:
Show Cashier Window:
Show Master Cashier Window In POS
Print Store Summary With POS Sale Inv.:
Ask User/Password In POS
Reset Inv. Balance Field in Footer On Saving
POS List View Retrieval Limit (days):
POS List View Max. Retrieval Limit (days):
Show Sale in Cashier Activity Window:
Lock Sale Qty/Bonus:
Lock Item Name:
Allow CTRL+D :
Must Save Invoice on Exit:
Default LCD Config:
Use LCD Display:
LCD Display Manufacturer/Model:
Default Cash Drawer COM Port
Use Cash Drawer:
Use Cash Drawer With Printer:
Cash Drawer Printer Name:
Use BarCode Printer:
BarCode Printer Name/Model:`,
	"Cashier Job Activity": `
Date:
Header Inv. No.:
Remarks:
User:
Machine Name:
Posted:
GL Voucher Code:
Type/Module:
Invoice Number:
Invoice Amount:
Cash Tendered:
Cash Charged:
Balance/Cash Back:
Cash Account:
Category:
Account Title:
Supervised:
Allow Selection:
Refresh Time (in seconds):
Mode:
Auto Fill Cash Charged:
Show Empty Doc. Code by Default:
Keep Cash Window Always Open:
Show Saving/Supervised Messages:`,
	"Email": `
SMTP Server:
SMTP Port:
From/Sender Name:
Email User ID:
Email Password:
SMTP Server Requires Authentication (User/Password):
SMTP Encryption Type:
Email Subject:
Email Body:`,
	"SMS": `
SMS Method:
Web SMS Provider:
Web SMS User ID:
Web SMS Password:
Web SMS Mask:
Web SMS API Key:`,
	"Dashboard": `
Summary Analysis Days Lim.:
Sales Analysis Days Lim.:
Inventory Breakup On:
Purchase Analysis Days Lim.:
Receipt Analysis Days Lim.:
Issue Analysis Days Lim.:
Adjustment Analysis Days Lim.:
Godown Transfer Analysis Days Lim.:
Accounts Analysis Days Lim.:
Service Analysis Days Lim.:`,
}

var explicitPreferenceDefaults = map[string]string{
	"General\x00Inventory System:":                                "Periodic Inventory System",
	"General\x00Inventory Movement Method:":                       "Equal Priority / Shortest Expiry First",
	"General\x00Default Batch:":                                   ".",
	"General\x00Default Expiry:":                                  "2030-12-12 00:00:00",
	"General\x00Max. Allowed Days:":                               "30",
	"General\x00Application Title:":                               "ABUZAR V3 01.01.2025",
	"General\x00Quick Search Type:":                               "X",
	"General\x00Check Manual Backup Health At StartUp:":           "Yes",
	"General\x00Age of Item Changes For Data Carry (in Days):":    "999",
	"Report\x00Refresh Time(Minutes):":                            "1",
	"Report\x00Default Start Date:":                               "2005-07-12 15:37:01",
	"Point of Sale\x00POS List View Retrieval Limit (days):":      "1",
	"Point of Sale\x00POS List View Max. Retrieval Limit (days):": "5",
	"Point of Sale\x00Default Cash Drawer COM Port":               "1",
	"Cashier Job Activity\x00Refresh Time (in seconds):":          "15",
	"Others\x00SMS Expiry (in Hours):":                            "24",
	"Others\x00Refresh Time (Seconds):":                           "15",
	"Others\x00Activity Period (Minutes):":                        "30",
	"Others\x00Distributor Code:":                                 "90",
	"Others\x00Retail Price(%) for Sale Inv.:":                    "0.00",
	"Others\x00Auto Purge Posted Sale Time(Minutes)":              "60",
	"Dashboard\x00Summary Analysis Days Lim.:":                    "30",
	"Dashboard\x00Sales Analysis Days Lim.:":                      "30",
	"Dashboard\x00Purchase Analysis Days Lim.:":                   "30",
	"Dashboard\x00Receipt Analysis Days Lim.:":                    "30",
	"Dashboard\x00Issue Analysis Days Lim.:":                      "30",
	"Dashboard\x00Adjustment Analysis Days Lim.:":                 "30",
	"Dashboard\x00Godown Transfer Analysis Days Lim.:":            "30",
	"Dashboard\x00Accounts Analysis Days Lim.:":                   "30",
	"Dashboard\x00Service Analysis Days Lim.:":                    "30",
}

func reviewedPreferenceRegistry() []preferenceDefinition {
	registryOnce.Do(func() {
		for _, category := range []string{
			"General", "Sale", "Sale Return", "Purchase", "Purchase Return",
			"Report", "BasicData", "Quotation", "Schedule", "Adjustment",
			"Purchase Order", "Others", "Point of Sale", "Cashier Job Activity",
			"Email", "SMS", "Dashboard",
		} {
			position := 0
			lines := make([]string, 0)
			counts := make(map[string]int)
			for _, line := range strings.Split(preferenceCatalog[category], "\n") {
				caption := strings.TrimSpace(line)
				if caption == "" {
					continue
				}
				lines = append(lines, caption)
				counts[caption]++
			}
			occurrences := make(map[string]int)
			for _, caption := range lines {
				occurrences[caption]++
				registry = append(registry, reviewedPreference(category, caption, position, occurrences[caption], counts[caption]))
				position++
			}
		}
	})
	return registry
}

func reviewedPreference(category, caption string, position, occurrence, count int) preferenceDefinition {
	key := category + "\x00" + caption
	defaultValue := explicitPreferenceDefaults[key]
	fieldKey := preferenceFieldKey(category, caption, occurrence, count)
	storageCaption := caption
	if count > 1 {
		storageCaption = caption + " [" + fieldKey + "]"
	}
	definition := preferenceDefinition{
		Category: category, Caption: caption, FieldKey: fieldKey, StorageCaption: storageCaption, Type: preferenceText,
		Default: defaultValue, Position: position, RuntimeStatus: "stored_only",
		Behavior: "No backend behavior is currently dependent on this value.",
	}
	lower := strings.ToLower(caption)
	if strings.Contains(lower, "password") || strings.Contains(lower, "api key") {
		definition.Type, definition.RuntimeStatus = preferenceSecret, "stored_only"
		definition.Behavior = "Stored encrypted-by-deployment policy; never returned to audit payloads."
	} else if isPreferenceBooleanCaption(lower) {
		definition.Type, definition.Allowed = preferenceBoolean, []string{"Yes", "No"}
		if defaultValue == "" {
			defaultValue = "No"
			definition.Default = defaultValue
		}
	} else if strings.Contains(lower, "date") && !strings.Contains(lower, "update") {
		definition.Type = preferenceDate
	} else if strings.Contains(lower, "time") || lower == "once at:" || lower == "hour:" || lower == "minute:" || lower == "second:" {
		definition.Type = preferenceTime
	} else if isPreferenceIntegerCaption(lower) {
		definition.Type = preferenceInteger
		minimum, maximum := 0.0, 1000000.0
		definition.Minimum, definition.Maximum = &minimum, &maximum
	} else if strings.Contains(lower, "%") || strings.Contains(lower, "amount") || strings.Contains(lower, "rate") {
		definition.Type = preferenceDecimal
		minimum, maximum := 0.0, 1000000.0
		definition.Minimum, definition.Maximum = &minimum, &maximum
	}
	if enumValues, ok := preferenceEnumValues(category, caption); ok {
		definition.Type, definition.Allowed = preferenceEnum, enumValues
	}
	definition.Behavior, definition.RuntimeStatus = preferenceBehavior(category, caption)
	return definition
}

func preferenceFieldKey(category, caption string, occurrence, count int) string {
	base := preferenceKeyPart(category) + "." + preferenceKeyPart(caption)
	if count > 1 {
		return fmt.Sprintf("%s.%d", base, occurrence)
	}
	return base
}

func preferenceKeyPart(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, character := range strings.ToLower(value) {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			builder.WriteRune(character)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func isPreferenceBooleanCaption(caption string) bool {
	for _, prefix := range []string{"enable ", "allow ", "ask ", "show ", "update ", "auto ", "lock ", "use ", "prompt ", "check ", "apply ", "keep ", "copy ", "prepare ", "synchronize ", "must ", "fetch ", "reset ", "consider ", "enforce ", "in ", "header", "date:", "posted:", "supervised:", "active:"} {
		if strings.HasPrefix(caption, prefix) {
			return true
		}
	}
	if strings.Contains(caption, "%") &&
		!strings.Contains(caption, "retail price") &&
		!strings.Contains(caption, "round item total") {
		return true
	}
	return strings.HasSuffix(caption, " option") || strings.Contains(caption, " requires authentication")
}

func isPreferenceIntegerCaption(caption string) bool {
	if strings.Contains(caption, "round item total") {
		return false
	}
	for _, token := range []string{"days", "minutes", "seconds", "hour", "minute", "second", "port", "qty", "quantity denomination", "copies", "com port", "decimal places"} {
		if strings.Contains(caption, token) {
			return true
		}
	}
	return false
}

func preferenceEnumValues(category, caption string) ([]string, bool) {
	switch category + "\x00" + caption {
	case "General\x00Inventory System:":
		return []string{"Periodic Inventory System", "Perpetual Inventory System"}, true
	case "General\x00Inventory Movement Method:":
		return []string{"Equal Priority / Shortest Expiry First", "FIFO", "FEFO"}, true
	case "Email\x00SMTP Encryption Type:":
		return []string{"None", "SSL", "TLS"}, true
	case "Dashboard\x00Inventory Breakup On:":
		return []string{"Category", "Manufacturer", "Godown"}, true
	case "Cashier Job Activity\x00Mode:":
		return []string{"Active Entry", "Posted Entry", "All"}, true
	default:
		return nil, false
	}
}

func preferenceBehavior(category, caption string) (string, string) {
	lower := strings.ToLower(caption)
	switch {
	case category == "Schedule":
		return "Captured legacy SQL-Agent/msdb schedule; PostgreSQL scheduler adapter is not configured.", "not_configured"
	case category == "Report" && strings.Contains(lower, "default header"):
		return "Mapped to the report definition letterhead name used by the existing report loader.", "wired"
	case strings.Contains(lower, "price #") || strings.Contains(lower, "retail price"):
		return "Stored for the captured preference contract; the current pricing API does not implicitly read this setting.", "stored_only"
	case strings.Contains(lower, "gst") || strings.Contains(lower, "sales tax") || strings.Contains(lower, "pct code") || strings.Contains(lower, "extra tax"):
		return "Stored for captured UI parity; tax rates and assignments are controlled by the existing tax API.", "stored_only"
	case strings.Contains(lower, "expiry") || strings.Contains(lower, "batch") || strings.Contains(lower, "stock") || strings.Contains(lower, "godown"):
		return "Stored for captured UI parity; stock posting independently validates branch, batch, and expiry data.", "stored_only"
	case category == "Point of Sale" && (strings.Contains(lower, "cashier") || strings.Contains(lower, "cash drawer") || strings.Contains(lower, "cash charged")):
		return "Stored for captured UI parity; cashier activity is branch scoped and physical drawer signaling remains an edge adapter concern.", "stored_only"
	default:
		return "No backend behavior is currently dependent on this value.", "stored_only"
	}
}

func preferenceValidationError(definition preferenceDefinition, value string) error {
	value = strings.TrimSpace(value)
	switch definition.Type {
	case preferenceBoolean:
		if value != "Yes" && value != "No" {
			return fmt.Errorf("must be Yes or No")
		}
	case preferenceInteger:
		number, err := strconv.Atoi(value)
		if err != nil || number < int(*definition.Minimum) || number > int(*definition.Maximum) {
			return fmt.Errorf("must be an integer between %d and %d", int(*definition.Minimum), int(*definition.Maximum))
		}
	case preferenceDecimal:
		number, err := strconv.ParseFloat(value, 64)
		if err != nil || number < *definition.Minimum || number > *definition.Maximum {
			return fmt.Errorf("must be a decimal between %.0f and %.0f", *definition.Minimum, *definition.Maximum)
		}
	case preferenceEnum:
		for _, allowed := range definition.Allowed {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("must be one of %s", strings.Join(definition.Allowed, ", "))
	case preferenceDate:
		if value != "" {
			valid := false
			for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05"} {
				if _, err := time.Parse(layout, value); err == nil {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("must be a date or timestamp")
			}
		}
	case preferenceTime:
		if value != "" {
			valid := false
			for _, layout := range []string{"15:04", "15:04:05"} {
				if _, err := time.Parse(layout, value); err == nil {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("must be a time in HH:MM[:SS] format")
			}
		}
	}
	if len(value) > 4096 {
		return fmt.Errorf("may not exceed 4096 characters")
	}
	return nil
}
