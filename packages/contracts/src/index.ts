export type UUID = string;

export interface TenantContext {
  tenantId: UUID;
  branchId?: UUID;
  counterId?: UUID;
  operatorId: UUID;
  username: string;
  roles: string[];
  permissions?: string[];
}

export interface LegacyRight {
  rightCode: string;
  permission?: string;
  allowed: boolean;
  legacyStatus?: string;
  mapping: 'explicit' | 'ambiguous';
}

export interface LegacyScope {
  scopeKind: string;
  scopeKey: string;
  scopeLabel?: string;
  allowed: boolean;
  legacyTable?: string;
}

export interface AccessResponse {
  tenantAdmin: boolean;
  permissions: string[];
  legacyRights: LegacyRight[];
  scopes: Record<string, Record<string, boolean>>;
  scopeRows: LegacyScope[];
  exceptions: string[];
}

export interface SessionResponse {
  authenticated: boolean;
  context: TenantContext & { displayName: string; tenantCode: string } | null;
  expiresAt?: string;
}

export interface TenantSummary {
  id: UUID;
  code: string;
  legalName: string;
  active: boolean;
}

export interface BranchSummary {
  id: UUID;
  code: string;
  name: string;
  timezone: string;
  active: boolean;
}

export interface CounterSummary {
  id: UUID;
  branchId: UUID;
  code: string;
  name: string;
  active: boolean;
}

export interface OperatorSummary {
  id: UUID;
  username: string;
  displayName: string;
  active: boolean;
  roles: string[];
  branchId?: UUID;
  counterId?: UUID;
}

export interface OperatorCreateRequest {
  username: string;
  displayName: string;
  password: string;
  active?: boolean;
  roleCode?: string;
  branchId?: UUID;
  counterId?: UUID;
}

export interface OperatorUpdateRequest {
  username: string;
  displayName: string;
  password?: string;
  active?: boolean;
  roleCode?: string;
  branchId?: UUID;
  counterId?: UUID;
}

export interface RoleSummary {
  id: UUID;
  code: string;
  name: string;
  memberCount: number;
  permissions: string[];
}

export interface RoleRightsResponse {
  roleId: UUID;
  permissions: string[];
  legacyRights: LegacyRight[];
  scopes: LegacyScope[];
}

export interface SyncBatchResult {
  accepted: number;
  duplicates: number;
  conflicts: number;
}

export interface TransactionResult {
  accepted: boolean;
  duplicate: boolean;
  eventId: UUID;
  aggregateId: UUID;
}

export interface SyncEnvelope<TPayload = unknown> {
  eventId: UUID;
  aggregate: string;
  aggregateId: UUID;
  tenantId: UUID;
  branchId: UUID;
  counterId: UUID;
  operatorId: UUID;
  occurredAt: string;
  idempotencyKey: string;
  schemaVersion: number;
  payload: TPayload;
}

export interface ConflictRecord {
  id: UUID;
  entityType: string;
  entityId: UUID;
  tenantId: UUID;
  branchId: UUID;
  status: 'open' | 'resolved' | 'dismissed';
  localValue: unknown;
  serverValue: unknown;
  resolution?: unknown;
  createdAt: string;
}

export interface MasterRecord {
  id: UUID;
  kind: string;
  legacyId?: string;
  code: string;
  name: string;
  payload: Record<string, unknown>;
  active: boolean;
  createdAt: string;
  updatedAt: string;
  suppliers?: ItemSupplier[];
}

export interface ItemAlternateAliasesResponse {
  aliases: string[];
}

export interface ItemImage {
  id?: UUID;
  rowId: number;
  imageDescription: string;
  imageData: string;
  imageType: string;
}

export interface ItemImagesResponse {
  images: ItemImage[];
}

export interface ItemNotesResponse {
  /** Base64-encoded bytes from the captured ItemNotes blob. */
  notesData: string;
}

export interface ItemAssociation {
  id?: UUID;
  legacyItemId: string;
  code?: string;
  name?: string;
}

export interface ItemAssociationsResponse {
  associations: ItemAssociation[];
}

export interface ItemAuthor {
  id?: UUID;
  authorCode: number;
  priority: number;
  rowId: number;
}

export interface ItemAuthorsResponse {
  authors: ItemAuthor[];
}

export interface ItemModel {
  id?: UUID;
  modelCode: number;
}

export interface ItemModelsResponse {
  models: ItemModel[];
}

export interface ItemPricePolicy {
  policyCode: string;
  name: string;
  legacyItemId: string;
}

export interface ItemPricePolicyTier {
  id?: UUID;
  quantityLimit: number;
  price: Decimal;
  expiryDate: string;
  flatDiscount: Decimal;
  discountPercent: Decimal;
}

export interface ItemPricePolicyResponse {
  policy: ItemPricePolicy | null;
  tiers: ItemPricePolicyTier[];
}

export interface ItemRegistrationRequest {
  id: UUID;
  requestCode: number;
  legacyItemId: string;
  requestedAt: string;
  serverName: string;
  machineName: string;
  sent: 'Y' | 'N';
  sentOn: string;
  sentBy?: number;
  serverRequestCode?: number;
  payload: Record<string, unknown>;
}

export interface ItemRegistrationRequestResponse {
  request: ItemRegistrationRequest | null;
}

export interface ItemUnpostedTransaction {
  id: UUID;
  kind: string;
  documentNumber: string;
  status: 'draft';
  occurredAt: string;
  lineNumber: number;
  itemLegacyId: string;
  itemName: string;
  quantity: Decimal;
  unitPrice: Decimal;
  lineTotal: Decimal;
}

export interface ItemUnpostedTransactionsResponse {
  itemId: UUID;
  transactions: ItemUnpostedTransaction[];
  truncated: boolean;
}

export interface ItemSupplier {
  id: UUID;
  legacySupplierId: string;
  supplierId?: UUID;
  priority?: number;
  rate?: Decimal;
  discountPercent?: Decimal;
  quantity?: Decimal;
  bonus?: Decimal;
  days?: number;
}

export interface ItemLookupResult {
  id: UUID;
  legacyId: string;
  code: string;
  name: string;
  payload: Record<string, unknown>;
  active: boolean;
  aliases: string[];
}

export type EdgeHardwareCapabilityName =
  | 'thermal_printer'
  | 'barcode_scanner'
  | 'cash_drawer'
  | 'biometric_reader'
  | 'sms'
  | 'email';

export interface EdgeHardwareCapability {
  name: EdgeHardwareCapabilityName;
  available: boolean;
  provider: string;
  reason?: string;
}

export interface EdgeHardwareCapabilitiesResponse {
  capabilities: EdgeHardwareCapability[];
}

export type EdgeHardwareReadinessStatus = 'ready' | 'degraded' | 'unavailable' | 'invalid_configuration';

export interface EdgeHardwareReadiness {
  ready: boolean;
  status: EdgeHardwareReadinessStatus;
  configurationValid: boolean;
  configurationError?: string;
  availableCount: number;
  totalCount: number;
  unavailable: EdgeHardwareCapabilityName[];
  capabilities: EdgeHardwareCapability[];
}

export interface EdgeSaleSlipLine {
  itemName: string;
  quantity?: string;
  total?: string;
}

export interface EdgeSaleSlip {
  header?: string;
  store?: string;
  invoiceNumber: string;
  date?: string;
  customer?: string;
  lines?: EdgeSaleSlipLine[];
  subtotal?: string;
  discount?: string;
  tax?: string;
  total: string;
  footer?: string;
}

export interface EdgePurchaseLabel {
  itemName: string;
  batch?: string;
  expiry?: string;
  mrp?: string;
  quantity?: string;
}

export interface EdgePurchaseLabelBatch {
  labels: EdgePurchaseLabel[];
  cutAfter?: boolean;
}

export interface EdgePrintResult {
  printed: true;
  bytes: number;
  provider: string;
}

export interface EdgeBarcodeItem {
  code: string;
  itemId: string;
  name: string;
}

export type TaxKind = 'gst' | 'pct' | 'advance';

export interface TaxRate {
  id: UUID;
  tenantId: UUID;
  branchId: UUID;
  taxKind: TaxKind;
  code: string;
  name: string;
  rate: Decimal;
  inclusive: boolean;
  effectiveFrom: string;
  effectiveTo?: string;
  sourceTable?: string;
  sourceLegacyId?: string;
  active: boolean;
}

export interface TaxAssignment {
  id: UUID;
  targetKind: 'item' | 'party';
  itemId?: UUID;
  partyId?: UUID;
  taxRate: TaxRate;
  effectiveFrom: string;
  effectiveTo?: string;
  sourceTable?: string;
  sourceLegacyId?: string;
}

export interface ApplyItemGSTRequest {
  rateId?: UUID;
  rate?: Decimal;
  inclusive?: boolean;
  effectiveFrom: string;
  effectiveTo?: string;
  itemIds?: UUID[];
  sourceTable?: string;
  sourceLegacyId?: string;
}

export interface ReportRow {
  documentId?: UUID;
  document: string;
  occurredAt: string;
  party: string;
  item: string;
  quantity: string;
  amount: string;
  alias?: string;
  itemDescription?: string;
  salePrice?: string;
  discountPercent?: string;
  discountValue?: string;
  itemDiscount?: string;
  salesTaxValue?: string;
  purchasePrice?: string;
  averagePrice?: string;
  recentPurchasePrice?: string;
  packUnits?: string;
  documentType?: string;
  alternateAccountCode?: string;
  userLegacyId?: string;
  invoiceCode?: string;
  expiryDate?: string;
  batchNumber?: string;
  grossProfit?: string;
  marginPercent?: string;
  reorderQuantity?: string;
  optimumQuantity?: string;
  minimumQuantity?: string;
  orderedQuantity?: string;
  receivedQuantity?: string;
  disparityQuantity?: string;
  orderedAmount?: string;
  receivedAmount?: string;
  disparityAmount?: string;
}

export interface ReportColumn {
  key: string;
  label: string;
  dataType: 'text' | 'date' | 'number' | 'currency';
  sortable: boolean;
}

export interface ReportFormat {
  id: string;
  name: string;
  source: 'default' | 'database';
}

export interface ReportRetrieval {
  title: string;
  areas: string[];
  supportsCashCredit: boolean;
  supportsDateRange: boolean;
  supportsTextFilter: boolean;
  scope: string;
}

export interface ReportLetterhead {
  name: string;
  line2: string;
  line3: string;
  phone: string;
  fax: string;
  source: 'default' | 'database';
}

export type ReportExportFormat = 'csv' | 'pdf' | 'excel';

export interface ReportExportHook {
  format: ReportExportFormat;
  status: 'available' | 'not_implemented';
  label: string;
  message: string;
}

export interface ReportDefinition {
  kind: string;
  title: string;
  projectionStatus: 'real' | 'event-ledger' | 'generic-fallback';
  projectionNote?: string;
  columns: ReportColumn[];
  formats: ReportFormat[];
  selectedFormat?: string;
  retrieval: ReportRetrieval;
  letterhead: ReportLetterhead;
  exports: ReportExportHook[];
}

export interface ReportResponse {
  kind: string;
  rows: ReportRow[];
  definition: ReportDefinition;
  page: number;
  pageSize: number;
  hasMore: boolean;
}

export interface FinanceLedgerEntry {
  id: string;
  documentId: string;
  debit: Decimal;
  credit: Decimal;
  balanceAfter?: Decimal;
  occurredAt: string;
  description: string;
  sourceType?: 'party-ledger' | 'payment-allocation' | 'receivable-adjustment' | 'return-allocation';
}

export interface FinanceLedgerResponse {
  partyId: UUID;
  balance: Decimal;
  entries: FinanceLedgerEntry[];
}

export interface ProblemDetails {
  type: string;
  title: string;
  status: number;
  detail?: string;
  code?: string;
  traceId?: string;
}

export interface HealthResponse {
  status: 'ok' | 'degraded';
  service: 'api' | 'edge';
  database: 'ok' | 'not_configured' | 'error';
  version: string;
  time: string;
}

/**
 * Financial values cross the wire as decimal strings so clients do not
 * introduce floating-point rounding into document totals.
 */
export type Decimal = string;

/** Deterministic, read-only pricing calculation used while a legacy document is edited. */
export interface PricingPreviewSupplierScheme {
  discountPercent?: Decimal;
  qualifyingQuantity?: Decimal;
  bonusQuantity?: Decimal;
}

export interface PricingPreviewLineInput {
  id: string;
  quantity: Decimal;
  prices?: Decimal[];
  unitPrice?: Decimal;
  itemDiscountPercent?: Decimal;
  supplierScheme?: PricingPreviewSupplierScheme;
}

export interface PricingPreviewTaxInput {
  rate: Decimal;
  inclusive?: boolean;
}

export interface PricingPreviewRequest {
  priceLevel?: number;
  lines: PricingPreviewLineInput[];
  groupDiscountPercent?: Decimal;
  customerDiscountPercent?: Decimal;
  overrideDiscountPercent?: Decimal;
  documentDiscountPercent?: Decimal;
  flatDiscountAmount?: Decimal;
  miscAmount?: Decimal;
  taxes?: {
    gst?: PricingPreviewTaxInput;
    pct?: PricingPreviewTaxInput;
    advanceTax?: PricingPreviewTaxInput;
  };
}

export interface PricingPreviewLineResult {
  id: string;
  priceLevel: number;
  resolvedUnitPrice: Decimal;
  supplierDiscount: Decimal;
  supplierBonusQuantity: Decimal;
  lineGross: Decimal;
  itemDiscount: Decimal;
  customerDiscount: Decimal;
  customerDiscountRate: Decimal;
  customerDiscountSource: 'group' | 'customer' | 'override';
  net: Decimal;
}

export interface PricingPreviewTaxResult {
  kind: 'gst' | 'pct' | 'advance_tax' | 'unknown';
  rate: Decimal;
  inclusive: boolean;
  base: Decimal;
  amount: Decimal;
}

export interface PricingPreviewResponse {
  tenantId: UUID;
  branchId?: UUID;
  priceLevel: number;
  lines: PricingPreviewLineResult[];
  subtotal: Decimal;
  lineDiscountTotal: Decimal;
  documentPercentDiscount: Decimal;
  flatDiscount: Decimal;
  documentDiscountTotal: Decimal;
  misc: Decimal;
  taxableBase: Decimal;
  taxes: PricingPreviewTaxResult[];
  totalDiscount: Decimal;
  total: Decimal;
}

export interface InventoryBalanceResponse {
  tenantId: UUID;
  branchId: UUID;
  itemLegacyId: string;
  available: Decimal;
}

export interface InventoryAvailableBatch {
  batchId: UUID;
  batchNumber: string;
  expiryDate?: string;
  quantity: Decimal;
  unitCost: Decimal;
}

export interface InventoryAvailabilityResponse {
  tenantId: UUID;
  branchId: UUID;
  itemLegacyId: string;
  godownId: UUID;
  batches: InventoryAvailableBatch[];
}

export type DocumentStatus = 'draft' | 'posted' | 'void';

export type SaleDocumentKind =
  | 'cash-sale'
  | 'credit-sale'
  | 'cash-return'
  | 'credit-return'
  | 'open-cash-return'
  | 'open-credit-return'
  | 'quotation'
  | 'refused-sale';

export type PurchaseDocumentKind =
  | 'pack-purchase'
  | 'loose-purchase'
  | 'opening-purchase'
  | 'purchase-return'
  | 'purchase-order';

export type DocumentKind = SaleDocumentKind | PurchaseDocumentKind;

export type SaleReturnDocumentKind =
  | 'cash-return'
  | 'credit-return'
  | 'open-cash-return'
  | 'open-credit-return';

export type DocumentAction = 'save' | 'post' | 'save-and-post' | 'void' | 'delete';

export interface DocumentPartyReference {
  id: UUID;
  code?: string;
  name?: string;
}

export interface DocumentPrice {
  priceTier?: number;
  unitPrice: Decimal;
  grossAmount: Decimal;
  discountPercent: Decimal;
  discountAmount: Decimal;
  netAmount: Decimal;
}

export interface DocumentTax {
  code?: string;
  rate: Decimal;
  taxableAmount: Decimal;
  amount: Decimal;
}

export interface DocumentTaxSummary {
  taxableAmount: Decimal;
  amount: Decimal;
  lines: DocumentTax[];
}

export interface BatchAllocation {
  batchId?: UUID;
  batchNumber: string;
  expiryDate?: string;
  godownId?: UUID;
  quantity: Decimal;
  unitCost?: Decimal;
}

export type StockDirection = 'in' | 'out' | 'none';

export interface DocumentStockSummary {
  direction: StockDirection;
  quantity: Decimal;
  availableBefore?: Decimal;
  availableAfter?: Decimal;
}

export interface DocumentLineDraft {
  lineNumber: number;
  itemId: UUID;
  /** Canonical source sale or purchase line required for posted closed returns. */
  sourceLineId?: UUID;
  quantity: Decimal;
  unitOfMeasure?: string;
  unitPrice?: Decimal;
  discountPercent?: Decimal;
  supplierScheme?: PricingPreviewSupplierScheme;
  /** API wire name for explicit purchase-return batch selection. */
  allocations?: BatchAllocation[];
  batchNumber?: string;
  expiryDate?: string;
  unitCost?: Decimal;
  gstRate?: Decimal;
  pctRate?: Decimal;
  advanceTaxRate?: Decimal;
  notes?: string;
}

export interface DocumentLine {
  id: UUID;
  lineNumber: number;
  itemId: UUID;
  sourceLineId?: UUID;
  itemLegacyId?: string;
  itemCode: string;
  itemName: string;
  quantity: Decimal;
  unitOfMeasure?: string;
  price: DocumentPrice;
  tax: DocumentTaxSummary;
  allocations: BatchAllocation[];
  batchNumber?: string;
  expiryDate?: string;
  unitCost?: Decimal;
  taxAmount?: Decimal;
  stock: DocumentStockSummary;
  lineTotal: Decimal;
  notes?: string;
}

export interface DocumentTotals {
  subtotal: Decimal;
  discountAmount: Decimal;
  miscAmount: Decimal;
  taxAmount: Decimal;
  totalAmount: Decimal;
  paidAmount: Decimal;
  balanceAmount: Decimal;
}

export interface DocumentPayment {
  mode: string;
  received: Decimal;
  tendered: Decimal;
  change: Decimal;
  accountCode?: string;
}

export interface DocumentGLPosting {
  accountId: UUID;
  accountCode?: string;
  accountName?: string;
  debit: Decimal;
  credit: Decimal;
  memo?: string;
}

export interface DocumentGLSummary {
  postings: DocumentGLPosting[];
  totalDebit: Decimal;
  totalCredit: Decimal;
}

export interface DocumentFinanceSummary {
  journalId: UUID;
  debitTotal: Decimal;
  creditTotal: Decimal;
  balanced: boolean;
  partyLedgerEntryId?: UUID;
  partyBalanceAfter?: Decimal;
}

export interface DocumentDraft<K extends DocumentKind = DocumentKind> {
  id?: UUID;
  kind: K;
  documentNumber?: string;
  occurredAt: string;
  /** Canonical party identity required for credit sales. */
  customerId?: UUID;
  customer?: DocumentPartyReference;
  supplierId?: UUID;
  supplier?: DocumentPartyReference;
  sourceDocumentId?: UUID;
  sourceDocumentNumber?: string;
  godownId?: UUID;
  reference?: string;
  remarks?: string;
  /** Source-compatible supplier credit term in whole days. */
  creditDays?: Decimal;
  /** Retained SaleLedger due date for new credit-sale aging. */
  dueDate?: string;
  voidReason?: string;
  lines: DocumentLineDraft[];
  priceLevel?: number;
  flatDiscountAmount?: Decimal;
  miscAmount?: Decimal;
  documentDiscountPercent?: Decimal;
  pricing?: Omit<PricingPreviewRequest, 'lines'>;
  payment?: DocumentPayment;
}

export interface DocumentBase<K extends DocumentKind = DocumentKind> {
  id: UUID;
  kind: K;
  status: DocumentStatus;
  documentNumber?: string;
  tenantId: UUID;
  branchId: UUID;
  counterId?: UUID;
  operatorId: UUID;
  occurredAt: string;
  customer?: DocumentPartyReference;
  supplierId?: UUID;
  supplier?: DocumentPartyReference;
  sourceDocumentId?: UUID;
  sourceDocumentNumber?: string;
  godownId?: UUID;
  reference?: string;
  remarks?: string;
  /** Retained supplier credit term for purchase aging. */
  creditDays?: Decimal;
  /** Retained SaleLedger due date for receivables aging. */
  dueDate?: string;
  lines: DocumentLine[];
  totals: DocumentTotals;
  payment?: DocumentPayment;
  /** Server-calculated pricing snapshot; absence does not imply stock/GL posting. */
  pricing?: PricingPreviewResponse;
  stock?: DocumentStockSummary;
  gl?: DocumentGLSummary;
  finance?: DocumentFinanceSummary;
  /** Soft-delete timestamp for an audited draft discard. */
  deletedAt?: string;
  createdAt: string;
  updatedAt: string;
  version: number;
}

/** A discriminated union by `kind`, suitable for exhaustive workflow handling. */
export type Document = {
  [K in DocumentKind]: DocumentBase<K>;
}[DocumentKind];

export interface DocumentCommandError {
  code: string;
  message: string;
  field?: string;
}

export interface DocumentCommandBase<
  K extends DocumentKind = DocumentKind,
  A extends DocumentAction = DocumentAction
> {
  commandId: UUID;
  kind: K;
  action: A;
  idempotencyKey: string;
  occurredAt: string;
  expectedVersion?: number;
}

export interface SaveDocumentCommand<K extends DocumentKind = DocumentKind>
  extends DocumentCommandBase<K, 'save'> {
  action: 'save';
  document: DocumentDraft<K>;
}

export interface PostDocumentCommand<K extends DocumentKind = DocumentKind>
  extends DocumentCommandBase<K, 'post'> {
  action: 'post';
  document: DocumentDraft<K>;
}

export interface SaveAndPostDocumentCommand<K extends DocumentKind = DocumentKind>
  extends DocumentCommandBase<K, 'save-and-post'> {
  action: 'save-and-post';
  document: DocumentDraft<K>;
}

export interface VoidDocumentCommand<K extends DocumentKind = DocumentKind>
  extends DocumentCommandBase<K, 'void'> {
  action: 'void';
  documentId: UUID;
  reason: string;
}

export interface DeleteDocumentCommand<K extends DocumentKind = DocumentKind>
  extends DocumentCommandBase<K, 'delete'> {
  action: 'delete';
  documentId: UUID;
  reason: string;
}

export type DocumentCommandForKind<K extends DocumentKind> =
  | SaveDocumentCommand<K>
  | PostDocumentCommand<K>
  | SaveAndPostDocumentCommand<K>
  | VoidDocumentCommand<K>
  | DeleteDocumentCommand<K>;

/** The command union keeps the command kind and nested document kind aligned. */
export type DocumentCommand = {
  [K in DocumentKind]: DocumentCommandForKind<K>;
}[DocumentKind];

export interface DocumentCommandAcceptedResult<
  K extends DocumentKind = DocumentKind,
  A extends DocumentAction = DocumentAction
> extends TransactionResult {
  /** Immutable sync_events identity; the command-receipt identity is internal. */
  accepted: true;
  kind: K;
  action: A;
  status: DocumentStatus;
  document: DocumentBase<K>;
}

export interface DocumentCommandRejectedResult<
  K extends DocumentKind = DocumentKind,
  A extends DocumentAction = DocumentAction
> {
  accepted: false;
  duplicate: boolean;
  eventId?: UUID;
  aggregateId?: UUID;
  kind: K;
  action: A;
  status: DocumentStatus;
  errors: DocumentCommandError[];
}

export type DocumentCommandResult<
  K extends DocumentKind = DocumentKind,
  A extends DocumentAction = DocumentAction
> =
  | DocumentCommandAcceptedResult<K, A>
  | DocumentCommandRejectedResult<K, A>;
