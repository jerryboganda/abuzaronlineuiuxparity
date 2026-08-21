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
Check Cr Limit In Cr Sales:
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
	"Sale\x00Check Cr Limit In Cr Sales:":                         "Yes",
	"Sale\x00Price # in Cash Sale:":                               "1",
	"Sale\x00Price # in Credit Sale:":                             "1",
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
	case category == "Sale" && strings.Contains(lower, "check cr limit"):
		return "When Yes, credit-sale posting is blocked if the customer's CrLimit would be exceeded. When No, the limit is observational only.", "wired"
	case category == "Sale" && (strings.Contains(lower, "price # in cash sale") || strings.Contains(lower, "price # in credit sale")):
		return "Sets the default sale price level (1-10) when the document does not specify priceLevel.", "wired"
	case category == "Sale" && strings.Contains(lower, "allow zero retail price"):
		return "When No, cash/credit sale posting rejects lines whose resolved retail price is zero.", "wired"
	case category == "Quotation" && strings.Contains(lower, "allow quotation on zero price"):
		return "When No, quotation posting rejects lines whose resolved retail price is zero.", "wired"
	case category == "Quotation" && (strings.Contains(lower, "show p/o rate") || strings.Contains(lower, "show p/o discount") || strings.Contains(lower, "show special rate")):
		return "When Yes, the matching quotation header field is shown. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Sale Return" && strings.Contains(lower, "show account for sale return"):
		return "When Yes, the sale-return account field is shown. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Sale Return" && strings.Contains(lower, "ask amount paid on s/return"):
		return "When Yes, saving a sale return asks for the amount paid before the document is posted.", "wired"
	case category == "Purchase Return" && strings.Contains(lower, "ask amount received on p/return"):
		return "When Yes, saving a purchase return asks for the amount received before the document is posted.", "wired"
	case category == "Purchase" && (strings.Contains(lower, "ask purchase order") || strings.Contains(lower, "ask new purchase order") || strings.Contains(lower, "ask credit days") || strings.Contains(lower, "ask l. c. no") || strings.Contains(lower, "ask purchase type") || lower == "ask header:"):
		return "When Yes, purchase save/post asks for the matching header value before the document is posted.", "wired"
	case category == "Sale" && lower == "ask header:":
		return "When Yes, cash/credit sale save/post asks for the header/remarks value.", "wired"
	case category == "Quotation" && lower == "ask header:":
		return "When Yes, quotation save/post asks for the header/remarks value.", "wired"
	case category == "Sale" && strings.Contains(lower, "customer balance"):
		return "Seeds the overlay-hidden Customer Balance sale header field.", "wired"
	case category == "Quotation" && strings.Contains(lower, "show ref. no"):
		return "When Yes, the quotation shows an extra Ref. No. field. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Purchase Return" && lower == "account for:":
		return "Seeds the overlay-hidden purchase-return Account For header field.", "wired"
	case category == "Purchase" && strings.Contains(lower, "supplier balance"):
		return "Seeds the overlay-hidden Supplier Balance purchase header field.", "wired"
	case category == "Purchase Return" && lower == "header:":
		return "When Yes, purchase-return save/post asks for the header/remarks value.", "wired"
	case category == "Sale Return" && strings.Contains(lower, "header on sale return"):
		return "When Yes, sale-return save/post asks for the header/remarks value.", "wired"
	case category == "Quotation" && (strings.Contains(lower, "manufacturer name") || lower == "color:" || strings.Contains(lower, "quantity denomination") || lower == "pack units:"):
		return "Seeds overlay-hidden quotation Manufacturer / Color / Quantity Denomination / Pack Units fields.", "wired"
	case category == "Sale Return" && (strings.Contains(lower, "payment mode amt. paid") || strings.Contains(lower, "payment a/c for amt. paid")):
		return "Seeds overlay-hidden sale-return Payment Mode and Payment A/C fields used with amount-paid.", "wired"
	case category == "Purchase Return" && (strings.Contains(lower, "payment mode amt. received") || strings.Contains(lower, "payment a/c for amt. received")):
		return "Seeds overlay-hidden purchase-return Payment Mode and Payment A/C fields used with amount-received.", "wired"
	case (category == "Sale Return" || category == "Purchase Return") && strings.Contains(lower, "round item total"):
		return "When set, displayed return line totals are rounded to this many decimal places.", "wired"
	case category == "Purchase Return" && strings.Contains(lower, "show pack qty"):
		return "When Yes, the purchase-return grid shows a Pack Qty column. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Purchase Order" && strings.Contains(lower, "default purchase order category"):
		return "Seeds the purchase-order Purchase Type header field.", "wired"
	case category == "Sale" && strings.Contains(lower, "print warranted invoice"):
		return "Seeds the overlay-hidden Print Warranted Invoice sale header field.", "wired"
	case category == "Sale" && strings.Contains(lower, "fiscalization machine ip"):
		return "Seeds the overlay-hidden Fiscalization Machine IP sale header field.", "wired"
	case category == "Sale" && (strings.Contains(lower, "item disc. %") || lower == "item gst %:" || strings.Contains(lower, "extra tax %") || strings.Contains(lower, "disc. % on cash sale") || strings.Contains(lower, "disc. % on credit sale") || strings.Contains(lower, "pre-discount") || strings.Contains(lower, "claimable disc")):
		return "When Yes, the matching sale grid column is shown. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Purchase" && (lower == "disc. %:" || strings.Contains(lower, "item level gst") || strings.Contains(lower, "extra tax %") || strings.Contains(lower, "sale disc. %") || lower == "margin%:" || strings.Contains(lower, "net margin")):
		return "When Yes, the matching purchase grid column is shown. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Purchase" && (strings.Contains(lower, "pct code") || strings.Contains(lower, "sales tax schedule")):
		return "Seeds overlay-hidden purchase PCT Code and Sales Tax Schedule header fields.", "wired"
	case category == "Quotation" && strings.Contains(lower, "claimable discount"):
		return "When Yes, the quotation grid shows a Claimable Disc. column. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Sale" && (strings.Contains(lower, "alternate alias name") || strings.Contains(lower, "item flat discount") || strings.Contains(lower, "bonus qty") || strings.Contains(lower, "item unit sales tax") || strings.Contains(lower, "claimable item") || strings.Contains(lower, "item packing") || strings.Contains(lower, "packing description") || strings.Contains(lower, "item description") || strings.Contains(lower, "batch no") || strings.Contains(lower, "item weight per unit") || strings.Contains(lower, "packing factor per unit")):
		return "Seeds overlay-hidden sale header extras captured from the legacy column/header contract.", "wired"
	case category == "Purchase" && (strings.Contains(lower, "alternate alias name") || strings.Contains(lower, "pre-disc. price") || strings.Contains(lower, "flat discount") || strings.Contains(lower, "packing description") || strings.Contains(lower, "p/o qty") || strings.Contains(lower, "bonus quantity") || strings.Contains(lower, "item description") || lower == "sales tax:" || strings.Contains(lower, "pur. tax") || strings.Contains(lower, "total pieces") || strings.Contains(lower, "mark-up") || lower == "manufacturer:" || strings.Contains(lower, "sale return basis")):
		return "Seeds overlay-hidden purchase header extras captured from the legacy column/header contract.", "wired"
	case category == "Sale Return" && (strings.Contains(lower, "page size") || strings.Contains(lower, "alternate alias name") || strings.Contains(lower, "packing description") || strings.Contains(lower, "item description") || strings.Contains(lower, "total pieces") || strings.Contains(lower, "thermal print format")):
		return "Seeds overlay-hidden sale-return header extras captured from the legacy column/header contract.", "wired"
	case category == "Purchase Return" && (strings.Contains(lower, "alternate alias name") || strings.Contains(lower, "packing description") || strings.Contains(lower, "item description") || strings.Contains(lower, "total pieces") || strings.Contains(lower, "price in purchase return")):
		return "Seeds overlay-hidden purchase-return header extras captured from the legacy column/header contract.", "wired"
	case category == "Quotation" && strings.Contains(lower, "remote "):
		return "Seeds overlay-hidden quotation remote quantity fields (net sale, stock, reorder, optimum, min, P/O).", "wired"
	case category == "Purchase Order" && (strings.Contains(lower, "item alert") || strings.Contains(lower, "required packs fraction") || strings.Contains(lower, "page size for print out") || strings.Contains(lower, "p/o supplier consideration")):
		return "Seeds overlay-hidden purchase-order Item Alert / packs fraction / page size / supplier consideration fields.", "wired"
	case category == "General" && strings.Contains(lower, "business short name"):
		return "Falls back the browser document title when Application Title is empty.", "wired"
	case category == "BasicData" && (lower == "manufacturer:" || strings.Contains(lower, "item category") || strings.Contains(lower, "item class")):
		return "Seeds Manufacturer / Category / Class on a new item-master record.", "wired"
	case category == "Others" && strings.Contains(lower, "retail price(%) for sale inv"):
		return "When Yes, the sale grid shows a Retail Price % column. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Others" && (strings.Contains(lower, "mfg. distributor code") || lower == "distributor code:"):
		return "Seeds overlay-hidden manufacturer/distributor code sale header fields.", "wired"
	case category == "Adjustment" && (strings.Contains(lower, "show header") || strings.Contains(lower, "show update avg. price column")):
		return "When Yes, the stock-adjustment workflow shows the matching extra field. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Adjustment" && (strings.Contains(lower, "alternate alias") || strings.Contains(lower, "adjustment qty")):
		return "Seeds overlay-hidden adjustment Alternate Alias and default Quantity.", "wired"
	case category == "Cashier Job Activity":
		return "Seeds overlay-hidden cashier activity columns/fields and optional auto-refresh of the shift table.", "wired"
	case category == "Point of Sale" && (strings.Contains(lower, "invoice size") || strings.Contains(lower, "delivered by") || strings.Contains(lower, "default lcd config") || strings.Contains(lower, "lcd display manufacturer") || strings.Contains(lower, "cash drawer printer name") || strings.Contains(lower, "barcode printer name")):
		return "Seeds overlay-hidden POS invoice/delivery/device-name header fields. Use LCD/drawer/barcode adapters remain env-backed.", "wired"
	case category == "Point of Sale" && (strings.Contains(lower, "list view retrieval limit") || strings.Contains(lower, "list view max. retrieval limit")):
		return "Seeds overlay-hidden POS list-view retrieval day limits.", "wired"
	case category == "Point of Sale" && (strings.Contains(lower, "show cashier window") || strings.Contains(lower, "show master cashier window in pos") || strings.Contains(lower, "print store summary") || strings.Contains(lower, "show sale in cashier activity") || strings.Contains(lower, "allow ctrl+d")):
		return "When Yes, the matching POS extra is shown on cash/credit sale. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Others" && (strings.Contains(lower, "preferred printer for activity monitor") || strings.Contains(lower, "sms expiry") || strings.Contains(lower, "refresh time (seconds)") || strings.Contains(lower, "activity period") || strings.Contains(lower, "output file") || strings.Contains(lower, "save as pdf") || strings.Contains(lower, "image scan tool")):
		return "Seeds overlay-hidden report extras for printer/output locations, SMS expiry, refresh, and scan tool.", "wired"
	case category == "Report" && strings.Contains(lower, "allow print setup"):
		return "When Yes, the report argument pane shows Allow Print Setup. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "BasicData" && (strings.Contains(lower, "item packing") || strings.Contains(lower, "associated godown")):
		return "Seeds overlay-hidden item-master Item Packing and Associated Godown fields.", "wired"
	case category == "Sale Return" && (strings.Contains(lower, "show master cashier window") || strings.Contains(lower, "show s/r in cash activity")):
		return "When Yes, the matching sale-return extra is shown. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Purchase Return" && strings.Contains(lower, "ask pur. invoice"):
		return "When Yes, purchase-return save/post asks for the source purchase invoice number.", "wired"
	case category == "Purchase" && strings.Contains(lower, "show net rate"):
		return "When Yes, the purchase grid shows a Net Rate column. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Purchase" && (strings.Contains(lower, "show unit weight") || strings.Contains(lower, "show total weight")):
		return "When Yes, the purchase grid shows Weight/Unit or Total Weight. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Report" && strings.Contains(lower, "apply default date"):
		return "When Yes, the report argument window seeds Start Date from Default Start Date.", "wired"
	case category == "Report" && strings.Contains(lower, "default start date"):
		return "Source date used when Apply Default Date for Report Arg. Window is Yes.", "wired"
	case category == "Report" && strings.Contains(lower, "show account in reports"):
		return "When Yes, report grids show an Account column. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Report" && strings.Contains(lower, "refresh time"):
		return "Auto-retrieves the open report on this interval after the first Retrieve (default 1 minute).", "wired"
	case category == "Report" && strings.Contains(lower, "report term"):
		return "Non-empty Report Term 1-6 values are shown on the report print letterhead (overlay-hidden at 1936x1048).", "wired"
	case category == "BasicData" && (strings.Contains(lower, "lock item sale price") || strings.Contains(lower, "lock item disc")):
		return "When Yes, the matching item master Sales Price or Sale Disc(%) field is read-only.", "wired"
	case category == "Quotation" && strings.HasPrefix(lower, "line"):
		return "Non-empty Line1-8 values are joined into the quotation print footer.", "wired"
	case category == "Purchase Order" && (strings.HasPrefix(lower, "line") || strings.Contains(lower, "purchase order footer")):
		return "Non-empty Line1-8 and Purchase Order Footer values are included in purchase-order print preview.", "wired"
	case category == "Quotation" && (strings.Contains(lower, "delivery days") || strings.Contains(lower, "validity days") || strings.Contains(lower, "payment to")):
		return "Seeds the quotation Delivery Days / Validity Days / Payment To header fields (overlay-hidden at 1936x1048).", "wired"
	case category == "Sale" && (strings.Contains(lower, "reference no. 2") || strings.Contains(lower, "reference no. 3") || strings.Contains(lower, "reference no. 4") || strings.Contains(lower, "sales person") || lower == "account for:" || lower == "message:" || lower == "doctor:" || strings.Contains(lower, "loyalty points") || lower == "currency:" || strings.Contains(lower, "motor vehicle")):
		return "Seeds overlay-hidden sale header fields (Ref 2/3/4, Sales Person, Doctor, Message, Account For, Loyalty, Currency, Motor Vehicle).", "wired"
	case category == "Sale Return" && strings.Contains(lower, "sale return account for"):
		return "Seeds the overlay-hidden sale-return Account field.", "wired"
	case category == "Purchase" && lower == "account for:":
		return "Seeds the overlay-hidden purchase Account For header field.", "wired"
	case (category == "Sale Return" || category == "Purchase Return") && strings.Contains(lower, "copy remarks in item description"):
		return "When Yes, empty document line notes are filled from header remarks on save/post.", "wired"
	case category == "Purchase Order" && strings.Contains(lower, "copy remarks in item description"):
		return "When Yes, empty purchase-order line notes are filled from header remarks on save/post.", "wired"
	case category == "Purchase Order" && (strings.Contains(lower, "show supplier reference") || strings.Contains(lower, "show delivery place") || strings.Contains(lower, "show usage palace") || strings.Contains(lower, "show required date") || strings.Contains(lower, "show remarks 2") || strings.Contains(lower, "show purchase type") || strings.Contains(lower, "show remarks 3") || strings.Contains(lower, "show misc.charges") || strings.Contains(lower, "show invoice discount") || strings.Contains(lower, "show invoice gst") || strings.Contains(lower, "show invoice flat discount") || strings.Contains(lower, "show grand total") || strings.Contains(lower, "show item remarks") || strings.Contains(lower, "show item sale tax") || strings.Contains(lower, "show item gst") || strings.Contains(lower, "show item flat discount") || strings.Contains(lower, "show disc. perc. 2")):
		return "When Yes, the matching purchase-order header or remarks field is shown. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "General" && strings.Contains(lower, "default batch"):
		return "Empty purchase receipt batches are filled from this value (legacy default '.').", "wired"
	case category == "General" && strings.Contains(lower, "default expiry"):
		return "Empty purchase receipt expiry dates are filled from this value (legacy default 2030-12-12).", "wired"
	case category == "General" && strings.Contains(lower, "max. allowed days"):
		return "Credit-sale dueDate may not be more than this many calendar days after the document date (legacy default 30).", "wired"
	case category == "General" && strings.Contains(lower, "prompt before printing"):
		return "When Yes, sale and purchase Print asks for confirmation before sending the slip or opening preview.", "wired"
	case category == "General" && strings.Contains(lower, "ask no. of copies"):
		return "When Yes, sale and purchase Print asks how many copies before preview.", "wired"
	case category == "Sale" && strings.HasPrefix(lower, "show "):
		return "When Yes, the matching cash/credit sale header field is shown. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "Purchase" && (strings.Contains(lower, "show agency") || strings.Contains(lower, "show vehicle") || strings.Contains(lower, "show ship to") || strings.Contains(lower, "show associated sale inv") || strings.Contains(lower, "show dept") || strings.Contains(lower, "show supplier date") || strings.Contains(lower, "show supplier amount") || strings.Contains(lower, "show grn no") || strings.Contains(lower, "show item image")):
		return "When Yes, the matching purchase header field is shown. Default No keeps the 1936x1048 overlay clean.", "wired"
	case category == "General" && strings.Contains(lower, "application title"):
		return "Sets the browser document title to the captured legacy application title (default ABUZAR V3 01.01.2025).", "wired"
	case category == "Purchase Return" && strings.Contains(lower, "ask no. of copies"):
		return "When Yes, purchase-return Print asks how many copies before preview.", "wired"
	case strings.Contains(lower, "price #") || strings.Contains(lower, "retail price"):
		return "Stored for the captured preference contract; the current pricing API does not implicitly read this setting.", "stored_only"
	case strings.Contains(lower, "gst") || strings.Contains(lower, "sales tax") || strings.Contains(lower, "pct code") || strings.Contains(lower, "extra tax"):
		return "Stored for captured UI parity; tax rates and assignments are controlled by the existing tax API.", "stored_only"
	case strings.Contains(lower, "expiry") || strings.Contains(lower, "batch") || strings.Contains(lower, "stock") || strings.Contains(lower, "godown"):
		return "Stored for captured UI parity; stock posting independently validates branch, batch, and expiry data.", "stored_only"
	case category == "Point of Sale" && (strings.Contains(lower, "cashier") || strings.Contains(lower, "cash drawer") || strings.Contains(lower, "cash charged")):
		return "Stored for captured UI parity; cashier activity is branch scoped and physical drawer signaling remains an edge adapter concern.", "stored_only"
	case category == "Email":
		return "Captured for legacy-tab parity only. A real SMTP client adapter now exists (services/edge/internal/hardware/smtp.go) and central maintenance test-email/send-email actions will attempt a real send when the deployment is configured, but this specific saved value is not read by that adapter: SMTP host/port/user/password/from/encryption are configured via SMTP_* environment variables on the branch-edge process, and the API only reaches that edge instance when ABUZAR_EDGE_CHANNEL_URL is set. Nothing currently copies this preference value into either configuration surface.", "stored_only"
	case category == "SMS":
		return "Captured for legacy-tab parity only. A real Web-SMS-gateway HTTP client adapter now exists (services/edge/internal/hardware/sms.go) and central maintenance test-sms/send-sms actions will attempt a real send when the deployment is configured, but this specific saved value is not read by that adapter: the gateway URL template, credentials, and mask are configured via SMS_GATEWAY_* environment variables on the branch-edge process, and the API only reaches that edge instance when ABUZAR_EDGE_CHANNEL_URL is set. Nothing currently copies this preference value into either configuration surface.", "stored_only"
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
