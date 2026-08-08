// Produced by a one-time reviewed mapping pass (2026-08-08). See
// docs/PHASE_R_SECURITY_RIGHTS_VERIFICATION_2026-08-08.md, section
// "2026-08-08 permission backfill", for methodology, coverage counts, and
// the categories deliberately left unmapped.
package httpapi

// legacyRightPermission maps a legacy SQL Server dbo.Rights.RightCode (as the
// text form found in group_rights.right_code) to the modern permission
// string it should unlock. It was built by cross-referencing all 486
// dbo.Rights rows against the 726 group_rights rows actually migrated for
// the four legacy groups (ADMINISTRATOR, REMOTE, SALES OFFICER, SHIFT
// INCHARGE; tenant eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee), using
// RightName/IndicesString for semantic matching and, wherever possible,
// cross-checking against the actual Go handlers that call
// requirePermission(...) with each permission string (business.go,
// documents.go, history.go, maintenance.go — see the per-entry comments).
//
// This is a static, human-reviewable table by design (not a runtime
// string-matching heuristic): 433 of 486 legacy right codes are mapped here;
// the remaining 53 are deliberately absent because they have no confident
// modern equivalent (e.g. Transactions/Accounting Vouchers, Payroll,
// E-Prescription, Patient/Student modules that don't exist in this app yet,
// or single legacy toggles that are genuinely ambiguous between read and
// write). A missing entry means "leave permission NULL" — never guess a
// mapping for a code not listed here.
//
// group_rights.permission for already-migrated tenants was backfilled from
// exactly this table by db/migrations/044_legacy_rights_permission_backfill.sql,
// which must be regenerated (not hand-edited) if this table changes.
var legacyRightPermission = map[string]string{
	"1":    "reports.read",      // Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"2":    "purchases.write",   // Purchase , Purchases (Pack) -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"3":    "purchases.write",   // Purchase -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"4":    "purchases.write",   // Purchase , Purchase Order -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"6":    "purchases.write",   // Purchase , Opening Purchase -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"9":    "sales.write",       // Sales -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"10":   "sales.write",       // Sales , Cash Sale -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"11":   "sales.write",       // Sales , Credit Sale -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"12":   "sales.write",       // Sales , Sale Return -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"13":   "sales.write",       // Sales , Sale Return , Cash Sale -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"14":   "sales.write",       // Sales , Sale Return , Credit Sale -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"15":   "sales.write",       // Sales , Open Sale Return , Cash Sale -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"16":   "sales.write",       // Sales , Open Sale Return , Credit Sale -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"25":   "reports.read",      // Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"26":   "reports.read",      // Reports , Daily Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"27":   "reports.read",      // Reports , Daily Reports , Sale -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"28":   "reports.read",      // Reports , Daily Reports , Sale , Sale Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"29":   "reports.read",      // Reports , Daily Reports , Sale , Sale Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"30":   "reports.read",      // Reports , Daily Reports , Sale Return -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"31":   "reports.read",      // Reports , Daily Reports , Sale Return , Sale Return Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"32":   "reports.read",      // Reports , Daily Reports , Sale Return , Sale Return Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"33":   "reports.read",      // Reports , Daily Reports , Purchase -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"34":   "reports.read",      // Reports , Daily Reports , Purchase , Purchase Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"35":   "reports.read",      // Reports , Daily Reports , Purchase , Purchase Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"36":   "reports.read",      // Reports , Daily Reports , Purchase Return -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"37":   "reports.read",      // Reports , Daily Reports , Purchase Return , Purchase Return Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"38":   "reports.read",      // Reports , Daily Reports , Purchase Return , Purchase Return Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"43":   "reports.read",      // Reports , Stock Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"44":   "reports.read",      // Reports , Stock Reports , Stock in hand -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"45":   "reports.read",      // Reports , Stock Reports , Expiry Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"46":   "reports.read",      // Reports , Stock Reports , Reorder Level Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"47":   "reports.read",      // Reports , Stock Reports , Stock Register -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"49":   "reports.read",      // Reports , Sales Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"51":   "reports.read",      // Reports , Daily Reports , Sales , Sales Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"52":   "reports.read",      // Reports , Purchase Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"53":   "reports.read",      // Reports , Purchase Reports , Periodic Purchases -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"54":   "reports.read",      // Reports , Accounts Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"58":   "reports.read",      // Reports , Accounts Reports , Ledger Reports , Account Ledger -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"61":   "reports.read",      // Reports , Listing -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"64":   "reports.read",      // Reports , Listing , Supplier List -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"65":   "reports.read",      // Reports , Listing , Items List -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"68":   "reports.read",      // Reports , Listing , Manufacturer List -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"74":   "master.read",       // Basic Data -- Basic Data browse/view screen for a master-record entity or lookup table
	"78":   "master.read",       // Basic Data , Customer .......... -- Basic Data browse/view screen for a master-record entity or lookup table
	"79":   "master.read",       // Basic Data , Supplier .......... , Supplier -- Basic Data browse/view screen for a master-record entity or lookup table
	"80":   "master.read",       // Basic Data , Customer .......... , Customer Category -- Basic Data browse/view screen for a master-record entity or lookup table
	"81":   "master.read",       // Basic Data , Item -- Basic Data browse/view screen for a master-record entity or lookup table
	"82":   "master.read",       // Basic Data , Item , Item Class -- Basic Data browse/view screen for a master-record entity or lookup table
	"84":   "master.read",       // Basic Data , Item , Item Category -- Basic Data browse/view screen for a master-record entity or lookup table
	"85":   "master.read",       // Basic Data , Item , Manufacturer -- Basic Data browse/view screen for a master-record entity or lookup table
	"101":  "manage.groups",     // Manage , Groups -- Manage > Groups is the exact legacy group-management screen
	"102":  "manage.users",      // Manage , Users -- Manage > Users is the exact legacy user-management screen
	"105":  "purchases.write",   // Purchase , Purchase Return -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"106":  "sales.write",       // Sales , Open Sale Return -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"111":  "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Wise Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"112":  "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Wise Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"113":  "reports.read",      // Reports , Sales Reports , Customer Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"116":  "reports.read",      // Reports , Sales Reports , Hourly Sales Graph -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"117":  "reports.read",      // Reports , Sales Reports , Customer Sales, Hourly Graph -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"118":  "reports.read",      // Reports , Sales Reports , User Wise , Invoice Graph -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"120":  "master.write",      // Maintenance , Change Items Price -- Item price/discount-category/basic-data/suppliers/reorder-qty/batch-lock mutation actions on item master records
	"121":  "maintenance.write", // Maintenance , Datbase Utilities -- Corresponds to maintenanceAction kinds handled by maintenancePermission's default branch ("maintenance.write"): database utilities / backup / integrity-check / historical-data import / inplace-initialization
	"123":  "reports.read",      // Reports , Sales Reports , Category Wise , Sale And Return -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"124":  "reports.read",      // Reports , Sales Reports , Manufacturer Wise , Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"128":  "reports.read",      // Reports , Sales Reports , User Wise , Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"129":  "reports.read",      // Reports , Sales Reports , Slow/Fast moving Items -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"130":  "reports.read",      // Reports , Sales Reports , Slow/Fast Moving Items Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"134":  "reports.read",      // Reports , Sales Reports , Customer Sales , Days Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"137":  "purchases.write",   // Purchase , Purchases (Loose) -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"140":  "reports.read",      // Reports , Stock Reports , Stoch in Hand Manufacturer Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"141":  "reports.read",      // Reports , Stock Reports , Stoch in Hand Category Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"148":  "reports.read",      // Reports , Purchase Reports , Purchase Order -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"152":  "reports.read",      // Reports , Stock Reports ,  Stock In Hand , Other Stock Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"168":  "reports.read",      // Reports , Stock Reports , Item Stock Register Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"170":  "reports.read",      // Reports , Purchase Reports , SupplierWise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"171":  "reports.read",      // Reports , Purchase Reports , SupplierWise , SupplierWise Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"172":  "reports.read",      // Reports , Purchase Reports , ManufacturerWise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"173":  "reports.read",      // Reports , Purchase Reports , ManufacturerWise , ManufacturerWise Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"177":  "reports.read",      // Reports , RePrinting -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"178":  "reports.read",      // Reports , RePrinting , RePrinting Sale -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"184":  "reports.read",      // Reports , Purchase Reports , Date Wise Purchase Graph -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"193":  "sales.write",       // Sales , Quotation -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"194":  "reports.read",      // Reports , Stock Reports , Stock Register for Norcotix Items -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"195":  "reports.read",      // Reports , Sales Reports , Net Sale Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"197":  "reports.read",      // Reports , Purchase Reports , Supplier Wise Net Purchase -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"205":  "reports.read",      // Reports , Sales Reports , Customer Sales , Invoice Wise Profit Margin Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"207":  "reports.read",      // Reports , Daily Reports , Sale Summary Inv.Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"208":  "reports.read",      // Reports , Daily Reports , Sale Return Summary Inv.Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"212":  "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Wise Item Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"218":  "reports.read",      // Reports , Sales Reports , User Wise , Category Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"219":  "master.write",      // Maintenance , Change Item Discount Category Wise -- Item price/discount-category/basic-data/suppliers/reorder-qty/batch-lock mutation actions on item master records
	"220":  "reports.read",      // Reports , Sales Reports , Category Wise , Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"221":  "reports.read",      // Reports , Purchase Reports , Category Wise Purchase -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"229":  "sales.write",       // Maintenance , Modify Sale Invoices -- Modifying already-posted sale invoices is a sales-document mutation
	"230":  "master.read",       // Basic Data , Supplier .......... , Supplier Category -- Basic Data browse/view screen for a master-record entity or lookup table
	"237":  "maintenance.write", // Maintenance , Adjustment -- Stock Adjustment (increase/decrease) and InterGodown Transfer correspond to the "inventory" transaction aggregate / stock.go, both gated by maintenance.write
	"238":  "maintenance.write", // Maintenance , Adjustment , Increase -- Stock Adjustment (increase/decrease) and InterGodown Transfer correspond to the "inventory" transaction aggregate / stock.go, both gated by maintenance.write
	"239":  "maintenance.write", // Maintenance , Adjustment , Decrease -- Stock Adjustment (increase/decrease) and InterGodown Transfer correspond to the "inventory" transaction aggregate / stock.go, both gated by maintenance.write
	"242":  "reports.read",      // Reports , Stock Reports ,  Stock In Hand , Class Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"246":  "reports.read",      // Reports , Sales Reports , Customer Sales, Invoice Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"255":  "reports.read",      // Reports , Stock Reports , Stock and Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"256":  "reports.read",      // Reports , Purchase Reports , Day Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"259":  "reports.read",      // Reports , Stock Reports , Stock In Hand , Batch, Priority Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"260":  "reports.read",      // Reports , RePrinting , Purchase -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"261":  "reports.read",      // Reports , RePrinting , Sale (with summary reports) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"263":  "reports.read",      // Reports , Sales Reports , Category Wise , Monthly Sale -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"266":  "reports.read",      // Reports , Daily Reports , Adjustment -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"267":  "reports.read",      // Reports , Daily Reports , Adjustment, Adjustment Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"268":  "reports.read",      // Reports , Daily Reports , Adjustment, Adjustment Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"269":  "reports.read",      // Reports , Sales Reports , User Wise , Discount Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"271":  "reports.read",      // Reports , Sales Reports , Category Wise , Gross Profit -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"272":  "reports.read",      // Reports , Stock Reports , Optimum Level Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"275":  "reports.read",      // Reports , Item Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"276":  "reports.read",      // Reports , Item Reports , History -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"277":  "reports.read",      // Reports , Item Reports , History , Sale Price Difference -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"281":  "reports.read",      // Reports , Sales Reports , Category Wise , Net Sale -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"285":  "reports.read",      // Reports , Sales Reports , Category Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"286":  "reports.read",      // Reports , Sales Reports , Manufacturer Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"287":  "reports.read",      // Reports , Sales Reports , Class Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"289":  "reports.read",      // Reports , Sales Reports , User Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"295":  "reports.read",      // Reports , Purchase Return Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"296":  "reports.read",      // Reports , Purchase Return Reports, Supplier Purchase Returns -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"297":  "reports.read",      // Reports , Purchase Return Reports, Supplier Purchase Returns, Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"298":  "reports.read",      // Reports , Purchase Return Reports, Supplier Purchase Returns, Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"303":  "reports.read",      // Reports , Stock Reports , Item Activity -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"307":  "reports.read",      // Reports , Daily Reports , Quotation -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"308":  "reports.read",      // Reports , Daily Reports , Quotation , Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"309":  "reports.read",      // Reports , Daily Reports , Quotation , Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"322":  "reports.read",      // Reports , Sales Reports, Item Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"326":  "reports.read",      // Reports , Stock Reports , Stock In Hand , Back Date -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"329":  "reports.read",      // Reports , Daily Reports , Purchase , Purchase Summary(Format2) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"344":  "reports.read",      // Reports , Daily Reports , Sale Return, Sale Return Detail Inv.Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"345":  "master.read",       // Basic Data , Item , Item -- Basic Data browse/view screen for a master-record entity or lookup table
	"353":  "master.read",       // Basic Data , Customer .......... , Customer -- Basic Data browse/view screen for a master-record entity or lookup table
	"355":  "master.read",       // Basic Data , Supplier .......... -- Basic Data browse/view screen for a master-record entity or lookup table
	"358":  "reports.read",      // Reports , Accounts Reports , Ledger Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"379":  "reports.read",      // Reports , Stock Reports , Stock Register(Narcotics Format2) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"387":  "reports.read",      // Reports , Daily Reports , Sale , Sale Summary Inv.Wise, Cust Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"388":  "reports.read",      // Reports , Stock Reports , Expiry Report(Classwise) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"395":  "reports.read",      // Reports , Sales Reports , Manufacturer Wise , Sales Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"411":  "reports.read",      // Reports , Item Reports , Stock Adjustments -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"412":  "reports.read",      // Reports , Item Reports , Stock Adjustments , Stock Adjustments Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"415":  "reports.read",      // Reports , Sales Reports , Category Wise , Item Wise Sale Discounts Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"419":  "reports.read",      // Reports , Item Reports , History , Item Basic Data Changes -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"440":  "reports.read",      // Reports , Daily Reports , Sale , Detail Inv. Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"444":  "reports.read",      // Reports , Daily Reports , Adjustment , Adjustment Summary Inv.Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"445":  "reports.read",      // Reports , Daily Reports , Adjustment , Adjustment Detail Inv.Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"448":  "reports.read",      // Reports , Sales Reports , Manufacturer Wise , Net Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"464":  "reports.read",      // Reports , Stock Reports , Minimum Level Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"485":  "reports.read",      // Reports , Item Reports , History , Item Sale Price Changes -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"487":  "reports.read",      // Reports , RePrinting , Sale(format2) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"490":  "maintenance.write", // Maintenance , Adjustment , Stock Adjustment -- Stock Adjustment (increase/decrease) and InterGodown Transfer correspond to the "inventory" transaction aggregate / stock.go, both gated by maintenance.write
	"500":  "sales.write",       // Sales , Rights , Cash Sale Posting -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"501":  "sales.write",       // Sales , Rights , Credit Sale Posting -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"502":  "sales.write",       // Sales , Rights , Cash Sale Modify -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"503":  "sales.write",       // Sales , Rights , Credit Sale Modify -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"504":  "purchases.write",   // Purchase , Purchases (Pack) , Posting -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"505":  "purchases.write",   // Purchase , Purchases (Loose) , Posting -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"506":  "purchases.write",   // Purchase , Opening Purchase , Posting -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"507":  "purchases.write",   // Purchase , Purchases (Pack) , Modify -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"508":  "purchases.write",   // Purchase , Purchases (Loose) , Modify -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"509":  "purchases.write",   // Purchase , Opening Purchase , Modify -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"510":  "master.write",      // Basic Data , Item , Item , Modify Item Basic Data -- Explicit item/customer/supplier basic-data modification right
	"511":  "sales.write",       // Sales , Rights , Modify Sale Price Downward -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"512":  "master.write",      // Basic Data , Item , Item , Modify Item Activeness -- Explicit item/customer/supplier basic-data modification right
	"513":  "sales.read",        // Sales , Rights , Display Price List In Sale -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"514":  "master.write",      // Basic Data , Item , Item , Modify Item Restriction -- Explicit item/customer/supplier basic-data modification right
	"515":  "master.write",      // Basic Data , Item , Item , Assign Restricted Items -- Explicit item/customer/supplier basic-data modification right
	"516":  "sales.write",       // Sales , Rights , Modify Sale Item Discount% -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"517":  "sales.write",       // Sales , Rights , Modify Sale Item FlatDiscount -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"518":  "sales.write",       // Sales , Rights , Modify Sale Return Item Discount% -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"519":  "sales.write",       // Sales , Rights , Modify Sale Return SalePrice -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"523":  "sales.read",        // Sales , Rights , Display (PurchasePrice, RecentPurchasePrice, AvgPrice) -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"524":  "master.write",      // Basic Data , Item , Item , Modify Item Alias Name -- Explicit item/customer/supplier basic-data modification right
	"525":  "master.write",      // Basic Data , Customer .......... , Customer , Modify Customer Alias Name -- Explicit item/customer/supplier basic-data modification right
	"526":  "master.write",      // Basic Data , Supplier .......... , Supplier , Modify Supplier Alias Name -- Explicit item/customer/supplier basic-data modification right
	"528":  "purchases.write",   // Purchase , Rights , Save and Posting -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"531":  "sales.write",       // Sales , Rights , Save Invoice(Ctrl + S) -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"535":  "sales.read",        // Sales , Rights , View Stock In Sale/Return Module Footer -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"538":  "sales.read",        // Sales , Rights , Show Invoices In List Window -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"539":  "sales.read",        // Sales , Rights , Show Net Amount Column In List Window -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"549":  "purchases.write",   // Purchase , Purchase Order , Rights , Modify Purchase Order -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"551":  "sales.write",       // Sales , Rights , Save as Normal Sale -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"552":  "purchases.write",   // Purchase , Purchase Return , Rights , Modify Purchase Return -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"553":  "maintenance.write", // Maintenance , Adjustment , Decrease , Modify Price -- Stock Adjustment (increase/decrease) and InterGodown Transfer correspond to the "inventory" transaction aggregate / stock.go, both gated by maintenance.write
	"554":  "maintenance.write", // Maintenance , Adjustment , Increase , Modify Price -- Stock Adjustment (increase/decrease) and InterGodown Transfer correspond to the "inventory" transaction aggregate / stock.go, both gated by maintenance.write
	"564":  "purchases.read",    // Purchase , Rights , Show Invoice List -- Purchase-category right; classified read by Show/View/Display/Preview keyword heuristic
	"566":  "master.read",       // Basic Data , Item , Item , Show Item List -- Basic Data browse/view screen for a master-record entity or lookup table
	"567":  "sales.read",        // Sales , Sale Return , Rights , Show Invoices In List -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"568":  "purchases.read",    // Purchase , Purchase Return , Rights , Show Invoices In List -- Purchase-category right; classified read by Show/View/Display/Preview keyword heuristic
	"574":  "sales.read",        // Sales , Rights , Preview Sale Invoice Margin -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"575":  "sales.read",        // Sales , Rights , Item Sale History -- Item Sale History / Previous Invoice Info are read-only lookups surfaced inside the sale screen
	"576":  "sales.read",        // Sales , Rights , Previous Invoice Info -- Item Sale History / Previous Invoice Info are read-only lookups surfaced inside the sale screen
	"587":  "master.read",       // Item Search Window , Show Purchase Price -- Item Search Window price display is a read-only master-data lookup
	"589":  "sales.read",        // Sales , Rights , View Invoice Level Flat Discount in Sale Module Footer -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"590":  "sales.read",        // Sales , Rights , View Invoice Level Misc. Charges in Sale Module Footer -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"591":  "sales.read",        // Sales , Rights , View Invoice Level Discount(%) in Sale Module Footer -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"595":  "sales.write",       // Sales , Sale Return , Rights , Modify Sale Return Price -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"602":  "sales.write",       // Sales , Quotation , Modify Quotation -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"603":  "sales.read",        // Sales , Rights , Show Refused Sale Entry Form -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"611":  "sales.write",       // Sales , Rights , Override Customer Credit Limit -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"613":  "maintenance.write", // Maintenance , Adjustment , Stock Adjustment , Save and Posting -- Stock Adjustment (increase/decrease) and InterGodown Transfer correspond to the "inventory" transaction aggregate / stock.go, both gated by maintenance.write
	"614":  "purchases.write",   // Purchase , Purchase Order , Rights , Change Calculated Required Packs -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"617":  "purchases.write",   // Purchase , Purchase Order , Rights , Modify Rate -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"623":  "sales.write",       // Sales , Sale Return , Rights , Save and Post -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"632":  "purchases.write",   // Purchase , Purchase Order , Rights , Edit Minimum Qty. -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"633":  "purchases.write",   // Purchase , Purchase Order , Rights , Edit Reorder Qty. -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"634":  "purchases.write",   // Purchase , Purchase Order , Rights , Edit Optimum Qty. -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"637":  "reports.read",      // Reports , Rights , Save As -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"638":  "reports.read",      // Reports , Rights , Save As Excel -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"644":  "sales.write",       // Sales , Rights , Cash Sales Retrieve -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"645":  "sales.write",       // Sales , Rights , Credit Sales Retrieve -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"649":  "master.write",      // Basic Data , Customer .......... , Customer , Modify Customer Basic Data -- Explicit item/customer/supplier basic-data modification right
	"650":  "master.write",      // Basic Data , Supplier .......... , Supplier , Modify Supplier Basic Data -- Explicit item/customer/supplier basic-data modification right
	"673":  "master.write",      // Basic Data , Customer .......... , Customer , Modify Customer Lisc. Expiry -- Explicit item/customer/supplier basic-data modification right
	"676":  "sales.write",       // Sales , Rights , Edit Quotation -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"687":  "purchases.write",   // Purchase , Purchase Order , Rights , Apply Customer Associated Quotation(Alt+F8) -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"691":  "purchases.write",   // Purchase , Purchase Order , Rights , Modify Required Pack(s) Qty -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"700":  "sales.write",       // Sales , Sale Return , Rights , Save Invoice [Ctrl + S] -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"701":  "sales.write",       // Sales , Rights , Modify Sale Price Upward -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"704":  "purchases.read",    // Purchase , Rights , Show Price/Values in Purchase -- Purchase-category right; classified read by Show/View/Display/Preview keyword heuristic
	"705":  "master.read",       // Basic Data , Item , Item , Show Purchase Price -- Basic Data browse/view screen for a master-record entity or lookup table
	"706":  "master.read",       // Basic Data , Item , Item , Show Avg. Price -- Basic Data browse/view screen for a master-record entity or lookup table
	"707":  "master.read",       // Basic Data , Item , Item , Show Recent Purchase Price -- Basic Data browse/view screen for a master-record entity or lookup table
	"708":  "sales.write",       // Sales , Rights , Save and Post (Ctrl + Q) -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"720":  "master.read",       // Basic Data , Supplier .......... , Supplier , Show Account Ledger -- Basic Data browse/view screen for a master-record entity or lookup table
	"802":  "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Category Wise Net Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"817":  "reports.read",      // Reports , Sales Reports , Item Wise , Item Sale and Return Activity -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"820":  "reports.read",      // Reports , Sales Reports , Category Wise , Customer Category Wise Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"826":  "reports.read",      // Reports , Stock Reports , Reorder/Optimum Level Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"829":  "master.write",      // Maintenance , Update Item Basic Data -- Item price/discount-category/basic-data/suppliers/reorder-qty/batch-lock mutation actions on item master records
	"832":  "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Category Wise Sales , Customer Category Wise Sales Summary Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"833":  "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Category Wise Sales , Customer Category Wise Sales Detail Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"860":  "reports.read",      // Reports , RePrinting , Sale Format(3) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"896":  "reports.read",      // Reports , Stock Reports , Stoch in Hand, Manufacturer Wise (Format2) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"897":  "reports.read",      // Reports , Sales Reports , Item Sales/Discount -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"913":  "reports.read",      // Reports , Reprinting , Sale Format(4) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"914":  "reports.read",      // Reports , Sales Reports , Sale/Return Summary Inv. Type Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"943":  "reports.read",      // Reports , RePrinting , Sale (with header wise summaries) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"947":  "reports.read",      // Reports , Purchase Reports , Purchase Order Manf. Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"950":  "master.write",      // Maintenance , Update Item Suppliers -- Item price/discount-category/basic-data/suppliers/reorder-qty/batch-lock mutation actions on item master records
	"951":  "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Wise Category Net Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"957":  "master.read",       // Basic Data , Item , Generic Item -- Basic Data browse/view screen for a master-record entity or lookup table
	"970":  "reports.read",      // Reports , Listing , Group Rights List -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"972":  "reports.read",      // Reports , Daily Reports , Sale , Sale Detail (Format 2) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"997":  "master.write",      // Maintenance , Change Item Reorder Qty -- Item price/discount-category/basic-data/suppliers/reorder-qty/batch-lock mutation actions on item master records
	"1008": "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Category Wise Sales , Net Sales and Volume -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1017": "reports.read",      // Reports , Sales Reports , Category Wise , Item Category Wise Monthly Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1032": "reports.read",      // Reports , Stock Reports , Daily Stock IN/OUT -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1040": "reports.read",      // Reports , Daily Reports , Sale , Refused Sales Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1042": "master.read",       // Basic Data , Item , Item Basic Data -- Basic Data browse/view screen for a master-record entity or lookup table
	"1044": "reports.read",      // Reports , Item Reports , History , New Item(s) Created/Defined -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1057": "reports.read",      // Reports , Listing , GroupWise Users List -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1059": "reports.read",      // Reports , Listing , Customers List , Category Wise Sale History -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1068": "reports.read",      // Reports , Stock Reports , Stock in Hand , Supplier Manufacturer Association -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1075": "reports.read",      // Reports , Sales Reports , Customer Sales , Monthly Net Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1076": "reports.read",      // Reports , Daily Reports , Header Wise Transaction Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1078": "reports.read",      // Reports , Daily Reports , Purchase Order -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1079": "reports.read",      // Reports , Daily Reports , Purchase Order , Purchase Order Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1082": "reports.read",      // Reports , Purchase Reports , Net Purchase Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1116": "master.read",       // Basic Data , Item , Price Policy -- Basic Data browse/view screen for a master-record entity or lookup table
	"1120": "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Category Wise Sales , Customer Wise Sales Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1121": "reports.read",      // Reports , Daily Reports , Adjustment , Adjustment Summary/Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1125": "reports.read",      // Reports , Daily Reports , Purchase Order , P/O Based Purchase Disparity -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1131": "reports.read",      // Reports , Stock Reports , Stock IN/OUT(Date Wise) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1132": "maintenance.write", // Maintenance , Import Historical Data -- Corresponds to maintenanceAction kinds handled by maintenancePermission's default branch ("maintenance.write"): database utilities / backup / integrity-check / historical-data import / inplace-initialization
	"1133": "maintenance.write", // Maintenance , Import Historical Data , Import Previous Sales -- Corresponds to maintenanceAction kinds handled by maintenancePermission's default branch ("maintenance.write"): database utilities / backup / integrity-check / historical-data import / inplace-initialization
	"1135": "reports.read",      // Reports , Listing , Customers List , Customer List Category Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1140": "reports.read",      // Reports , Sales Reports , Daily Sale Summary with Profit(Day wise grouping -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1142": "reports.read",      // Reports , Stock Reports , Stock In Hand , Stock Quantity Format -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1168": "reports.read",      // Reports , Daily Reports , Sale , Sale Detail Inv. Wise(with diff.col.) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1172": "reports.read",      // Reports , Sales Reports , Item Wise , Item Wise Net Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1176": "reports.read",      // Reports , Daily Reports , Sale , Sale Summary - Invoice Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1210": "reports.read",      // Reports , Sales Reports , Manufacturer Wise , Manufacturer Wise Sales And Return Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1212": "master.read",       // Basic Data , Item , Item Alert -- Basic Data browse/view screen for a master-record entity or lookup table
	"1217": "maintenance.write", // Maintenance , Inplace Initialization -- Corresponds to maintenanceAction kinds handled by maintenancePermission's default branch ("maintenance.write"): database utilities / backup / integrity-check / historical-data import / inplace-initialization
	"1263": "reports.read",      // Reports , Daily Reports , Adjustment , Item Wise Adjustment Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1269": "reports.read",      // Reports , Stock Reports , Stock in Hand , Stock in Hand - Audit Purpose -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1278": "sales.write",       // Sales , Refused Sales -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"1291": "reports.read",      // Reports , Sales Reports , Monthly Net Sales Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1295": "reports.read",      // Reports , Purchase Reports , Supplier Category Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1308": "maintenance.write", // Maintenance , Datbase Utilities , BackUp Database -- Corresponds to maintenanceAction kinds handled by maintenancePermission's default branch ("maintenance.write"): database utilities / backup / integrity-check / historical-data import / inplace-initialization
	"1309": "maintenance.write", // Maintenance , Datbase Utilities , Check Database Integrity -- Corresponds to maintenanceAction kinds handled by maintenancePermission's default branch ("maintenance.write"): database utilities / backup / integrity-check / historical-data import / inplace-initialization
	"1321": "reports.read",      // Reports , Sales Reports , User Wise , Net Cash -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1333": "reports.read",      // Reports , Sales Reports , User Wise , Sales Commission -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1344": "sales.read",        // Manage , Cashier Management -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"1345": "sales.read",        // Manage , Cashier Management , Cashier Job -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"1346": "sales.read",        // Manage , Cashier Management , Cashier Activity Window -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"1367": "reports.read",      // Reports , Sales Reports , User Wise , User Wise Sales Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1435": "reports.read",      // Reports , Sales Reports , Dead Item List -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1518": "reports.read",      // Reports , Item Reports , Deleted Sale Items Log -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1625": "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Category Wise Sales , Customer Category Wise Net Sales Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1630": "reports.read",      // Reports , Daily Reports , Sale , Sale Summary Machine and Invoice Range Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1670": "reports.read",      // Reports , Stock Reports , Stock Management Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1671": "reports.read",      // Reports , Purchase Reports , Withholding Tax Deduction -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1672": "reports.read",      // Reports , Listing , Sale Person Scope -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1673": "reports.read",      // Reports , Listing , Sale Person Scope , Manufacturer/Sub Area Wise Sales Person Conflict -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1676": "reports.read",      // Reports , Sales Reports , Manufacturer Wise , CNIC/NTN Registered Customers -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1677": "reports.read",      // Reports , Godown Reports , Godown Wise Stock in Hand (Audit Format2) -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1678": "reports.read",      // Reports , Sales Reports , Area/Region/Zone ... Wise , Area Wise Monthly Sales Comparison -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1679": "reports.read",      // Reports , Accounts Reports , Purchase Invoice Based Accounting , Detail Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1680": "reports.read",      // Reports , Accounts Reports , Purchase Invoice Based Accounting , Detail Reports , Purchase Invoice Accounting Detail -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1681": "master.read",       // Basic Data , Item , Sales Tax Schedule -- Basic Data browse/view screen for a master-record entity or lookup table
	"1682": "master.read",       // Basic Data , Customer .......... , Sale Promotion -- Basic Data browse/view screen for a master-record entity or lookup table
	"1683": "master.read",       // Basic Data , Item , PCT Codes -- Basic Data browse/view screen for a master-record entity or lookup table
	"1684": "master.read",       // Basic Data , Item , Generic Item Type -- Basic Data browse/view screen for a master-record entity or lookup table
	"1685": "reports.read",      // Reports , Sales Order Reports , Sale Order-Supplier Wise Order Estimates -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1686": "reports.read",      // Reports , Sales Order Reports , Sale Order based Sales Disparity -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1687": "reports.read",      // Reports , Sales Order Reports , Sale Order Items Not Sold -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1689": "reports.read",      // Reports , Stock Reports , Norcotics Stock Register-Generic Type Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1690": "reports.read",      // Reports , Student Reports , Grade/Branch Wise Student Strength -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1691": "reports.read",      // Reports , Issue Reports , Recipient Wise , Receipt/Issue Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1708": "master.read",       // Basic Data , Item , Item Thickness -- Basic Data browse/view screen for a master-record entity or lookup table
	"1709": "reports.read",      // Reports , Production Reports , Production Note Reports , Wastage Summary Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1710": "reports.read",      // Reports , Special Reports , Data Export Utility-Global Pharma -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1711": "reports.read",      // Reports , Godown Reports , Allowed Godown Wise Stock in Hand -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1712": "reports.read",      // Reports , Sales Reports , Customer Sales , Claimable for Allowed Customers -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1713": "reports.read",      // Reports , Patient Reports , Doctor Wise Patient Reports , Doctor/Patient Wise Net Service Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1714": "reports.read",      // Reports , Purchase Reports , Supplier/Manufacturer Wise G/P -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1715": "reports.read",      // Reports , Special Reports , Data Export Utility-Pharma Link -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1716": "reports.read",      // Reports , Sales Reports , Customer Sales , Customer NTN Wise Sales Tax Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1717": "reports.read",      // Reports , Special Reports , Data Export Utility-Next Pharma -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1718": "master.read",       // Basic Data , Customer .......... , Customer Sector -- Basic Data browse/view screen for a master-record entity or lookup table
	"1719": "reports.read",      // Reports , Special Reports , Data Export Utility-Bosch/Linz -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1720": "reports.read",      // Reports , Accounts Reports , Aging Analysis, Aging Analysis Summary - Customer Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1722": "reports.read",      // Reports , Special Reports , Data Export Utility-Sci Life Pharma -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1723": "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Category Wise Sales , Output Sales Tax Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1724": "reports.read",      // Reports , Purchase Reports , Supplier Category Wise , Input Sales Tax Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1725": "reports.read",      // Reports , Sales Reports , Sales Tax Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1726": "reports.read",      // Reports , RePrinting , Selected Sales and Summaries -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1727": "master.read",       // Basic Data , Item , Lock Reason -- Basic Data browse/view screen for a master-record entity or lookup table
	"1728": "reports.read",      // Reports , Special Reports , Data Export Utility-Clinix -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1729": "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Wise Advance Tax -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1730": "reports.read",      // Reports , Purchase Reports , Supplier Wise , Advance Income Tax -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1731": "reports.read",      // Reports , Special Reports , Data Export Utility-Otsuka -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1732": "reports.read",      // Reports , Sales Reports , Category Wise , Category Wise Day Net Sale -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1733": "reports.read",      // Reports , Godown Reports , Godown Wise Stock-With Batch Expiry Details -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1734": "reports.read",      // Reports , Godown Reports , Customer Associated Godown Stock -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1736": "reports.read",      // Reports , CRS Reports , CRS Stock Reports , CRS Stock and Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1737": "reports.read",      // Reports , CRS Reports , CRS Stock Reports , CRS Stock and Sales , Manufacturer and Category Wise -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1738": "reports.read",      // Reports , CRS Reports , CRS Stock Reports , Consolidated Stock Position -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1739": "reports.read",      // Reports , Special Reports , Data Export Utility-Libra -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1740": "reports.read",      // Reports , Service Reports , Purchase Of Service Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1741": "reports.read",      // Reports , Service Reports , Purchase Of Service Reports , Supplier Wise Purchase of Services -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1743": "reports.read",      // Reports , Special Reports , Data Export Utility-Racket -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1744": "reports.read",      // Reports , Sales Reports , Sales Man Wise , Sales Person Wise Customer/Item Net Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1745": "reports.read",      // Reports , Purchase Reports , ManufacturerWise , Monthly Stock Movement -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1746": "reports.read",      // Reports , Sales Reports , Sales Man Wise , Sales Person Wise Monthly Net Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1747": "reports.read",      // Reports , Sales Reports , Sales Man Wise , Sales Person/Customer Wise Manufacturer Net Sales -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1759": "reports.read",      // Reports , Patient Reports , Patient Transaction Status -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1760": "reports.read",      // Reports , Sales Reports , Customer Sales , Customer Category Wise Sales , Customer Wise Gross Profit -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1761": "reports.read",      // Reports , Sales Reports , Sales Summary Reports , Month Wise Gross Profit -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1784": "master.read",       // Basic Data , Item , Category Segment -- Basic Data browse/view screen for a master-record entity or lookup table
	"1785": "reports.read",      // Reports , CRS Reports , CRS Invoice Search -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1786": "reports.read",      // Reports , Special Reports , Data Export Utility-Masood Homoeo -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1787": "reports.read",      // Reports , Issue Reports , Issue Summary , Issue Based Receipts -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1788": "reports.read",      // Reports , Stock Reports , Stock in Hand , Batch, Priority Wise - Audit Purposes -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1789": "reports.read",      // Reports , Student Reports , Data Export Utility-Bank Format -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1790": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1791": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports , CRS Ledger Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1792": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports , CRS Ledger Reports , CRS Customer Ledger -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1793": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports , CRS Ledger Reports , CRS Supplier Ledger -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1794": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports , CRS Ledger Reports , CRS Accounts Ledger -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1795": "reports.read",      // Reports , Item Reports , History , Item Name Changes -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1796": "master.read",       // Basic Data , Item , Item , Show Allow Sale Price Below AvgPrice -- Basic Data browse/view screen for a master-record entity or lookup table
	"1797": "sales.write",       // Sales , Rights , Allow Sale Price Below AvgPrice -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"1798": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports , CRS Balance Reports -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1799": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports , CRS Balance Reports , CRS Customer Balances -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1800": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports , CRS Balance Reports , CRS Supplier Balances -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1801": "reports.read",      // Reports , CRS Reports , CRS Accounting Reports , CRS Balance Reports , CRS Account Balances -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1802": "reports.read",      // Reports , Special Reports , Data Export Utility-Neutro Pharma -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1810": "master.read",       // Basic Data , Item , Manufacturer Type -- Basic Data browse/view screen for a master-record entity or lookup table
	"1818": "master.read",       // Basic Data , State/Province -- Basic Data browse/view screen for a master-record entity or lookup table
	"1820": "sales.read",        // Sales , Rights , Show Pre Disc. % -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"1821": "sales.write",       // Sales , Rights , Digitalize Sale Invoice {CTRL+SHIFT+D} -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"1822": "sales.write",       // Sales , Rights , Update Digital Invoice Info {CTRL+SHIFT+M} -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"1825": "sales.write",       // Sales , Rights , Generate Sales From Pending Quotations [CTRL+SHIFT+G] -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"1826": "maintenance.write", // Maintenance , Godown Preferences , InterGodown Transfer , Modify Transfer -- Stock Adjustment (increase/decrease) and InterGodown Transfer correspond to the "inventory" transaction aggregate / stock.go, both gated by maintenance.write
	"1827": "reports.read",      // Reports , CRS Reports , CRS Stock Reports , CRS Item Stock Register -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1828": "reports.read",      // Reports , Employee Reports , Payroll Reports , Pay Slip Re-Printing -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1829": "reports.read",      // Reports , CRS Reports , CRS Sales Reports , CRS - Day Wise Gross Profit Summary -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"1831": "reports.read",      // Reports , CRS Reports , CRS Stock Reports , CRS Stock Management -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"5027": "sales.read",        // Manage , Cashier Management , Cashier Activity Window , Rights , Show Print Preview -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"5056": "sales.read",        // Sales , Rights , Show Customer Wise Sale Detail Report -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5064": "sales.read",        // Manage , Cashier Management , Cashier Activity Window , Rights , Supervise All [F9] -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"5065": "sales.read",        // Manage , Cashier Management , Cashier Activity Window , Rights , Supervise Selected [F8] -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"5066": "sales.read",        // Manage , Cashier Management , Cashier Activity Window , Rights , Show Cash Tendered Window [F6] -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"5067": "sales.read",        // Manage , Cashier Management , Cashier Activity Window , Rights , Supervise Current [F7] -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"5068": "sales.read",        // Manage , Cashier Management , Cashier Activity Window , Rights , Change Category [F10] -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"5091": "purchases.read",    // Purchase , Rights , Show Item Purchase History [Ctrl+H] -- Purchase-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5101": "master.write",      // Basic Data , Item , Item , Add New Item -- Explicit item/customer/supplier basic-data modification right
	"5121": "master.write",      // Purchase , Rights , Create New Item -- "Create New Item" from the Purchase screen creates a master item record, not a purchase document
	"5192": "purchases.write",   // Purchase , Purchase Return , Rights , Modify Price -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5193": "purchases.write",   // Purchase , Purchase Return , Rights , Modify Disc. % -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5207": "purchases.write",   // Purchase , Purchase Return , Rights , Post Purchase Return -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5213": "sales.read",        // Sales , Sales Return , Rights , View Invoice Level Discount(%) in Footer -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5214": "sales.read",        // Sales , Sales Return , Rights , View Invoice Level Flat Discount in Footer -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5215": "sales.read",        // Sales , Sales Return , Rights , View Invoice Level GST% in Footer -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5216": "sales.read",        // Sales , Sales Return , Rights , View Invoice Level Misc. Charges in Footer -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5217": "reports.read",      // Reports , Rights , Print Report -- Reports-category right; modern app has a single reports.read gate for all report viewing
	"5229": "purchases.write",   // Purchase , Rights , Allow Deviation From Previous Margin On Posting -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5236": "master.write",      // Maintenance , Lock Item Batches -- Item price/discount-category/basic-data/suppliers/reorder-qty/batch-lock mutation actions on item master records
	"5237": "sales.write",       // Sales , Rights , Fiscalize Sale Invoice -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5238": "sales.write",       // Sales , Sales Return , Rights , Fiscalize S/R Invoice -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5241": "sales.write",       // Sales , Rights , Fiscalize Sale Invoices -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5242": "sales.write",       // Sales , Sales Return , Rights , Fiscalize S/R Invoices -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5243": "sales.write",       // Sales , Rights , Attach Document(s) -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5244": "sales.read",        // Sales , Rights , Show Document Gallery -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5245": "purchases.write",   // Purchase , Rights , Attach Document(s) -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5246": "purchases.read",    // Purchase , Rights , Show Document Gallery -- Purchase-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5247": "sales.write",       // Sales , Rights , Apply Customized GST % in Unit S/Tax -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5248": "master.read",       // Basic Data , Tax Category -- Basic Data browse/view screen for a master-record entity or lookup table
	"5250": "sales.read",        // Manage , Cashier Management , Cashier Activity Window , Rights , Show Totals -- maintenancePermission("manage-cashier-job") in maintenance.go returns sales.read; this is the Cashier Management/Cashier Job/Cashier Activity Window cluster that action gates
	"5253": "sales.read",        // Sales , Rights , Show Branch Wise Item Stock Position -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5256": "master.read",       // Basic Data , Item , Item , Show Sale Price -- Basic Data browse/view screen for a master-record entity or lookup table
	"5257": "master.read",       // Basic Data , Item , Item , Show Sale Discount % -- Basic Data browse/view screen for a master-record entity or lookup table
	"5258": "master.read",       // Basic Data , Item , Item , Show Flat Discount -- Basic Data browse/view screen for a master-record entity or lookup table
	"5259": "sales.read",        // Sales , Rights , Show Godown Wise Stock [F6 on Qty] -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5260": "sales.read",        // Sales , Rights , Show Unit Qty Calculator [F7 on Qty] -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5261": "sales.read",        // Sales , Rights , Show Batch Sale Price Selection [F8 on Qty] -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5262": "sales.read",        // Sales , Rights , Show Qty/Rate/Value Calculator [F9 on Qty] -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5267": "purchases.write",   // Purchase , Rights , Fetch Purchase Invoice From Other Sources -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5282": "sales.read",        // Sales , Rights , Show Item Purchase History -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5283": "purchases.read",    // Purchase , Purchase Return , Rights , View Invoice Level Discount(%) -- Purchase-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5284": "purchases.read",    // Purchase , Purchase Return , Rights , View Invoice Level Flat Discount -- Purchase-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5285": "purchases.read",    // Purchase , Purchase Return , Rights , View Invoice Level Misc. Charges -- Purchase-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5286": "sales.read",        // Sales , Services , Rights , Show Invoices In List Window -- Sales-category right; classified read by Show/View/Display/Preview keyword heuristic
	"5289": "sales.write",       // Sales , POS Sale , Rights , Save as Credit Invoice -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5290": "purchases.write",   // Purchase , Rights , Modify Price/Values in Purchase -- Purchase-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5302": "sales.write",       // Sales , Services , Rights , Fiscalize Service Invoice [CTRL+ALT+Z] -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5303": "sales.write",       // Sales , Services , Rights , Fiscalize Service Invoice(s) [CTRL+ALT+F] -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5304": "sales.write",       // Sales , Service Return , Rights , Fiscalize Service Return [CTRL+ALT+Z] -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
	"5305": "sales.write",       // Sales , Service Return , Rights , Fiscalize Service Return(s) [CTRL+ALT+F] -- Sales-category right; classified write by Show/View/Display/Preview keyword heuristic
}
