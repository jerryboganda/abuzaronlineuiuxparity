<script lang="ts">
  import { page } from '$app/stores';
  import { beforeNavigate } from '$app/navigation';
  import { onMount } from 'svelte';
  import type { Document, ItemLookupResult, InventoryAvailableBatch, MasterRecord, SessionResponse, SyncEnvelope, ReportRow, PricingPreviewResponse, PricingPreviewRequest, DocumentCommandForKind } from '@abuzar/contracts';
  import { AbuzarApi, ApiError, OfflineQueue, edgeRequest, newEventId } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';
  import type { MenuAction } from '$lib/legacy-menu';
  import { formatLegacyTitle } from '$lib/legacy-title';
  import { localDateAtNoonUtc, localDateString } from '$lib/calendar-date';

  type SaleAllocation = { batchId: string; quantity: string };
  type SaleRow = { itemId?: string; itemLegacyId?: string; sourceLineId?: string; itemName: string; stock: string; stockError?: string; availabilityLoaded: boolean; availableBatches?: InventoryAvailableBatch[]; allocations?: SaleAllocation[]; purchasePrice: string; salePrice: string; salePrices?: string[]; manufacturer: string; pieces: string; location: string; quantity: string; discountPercent: string; gstRate: string; batchNumber: string; expiryDate: string; unitCost: string; total: string };
  type LookupItem = ItemLookupResult & { stock: string; purchasePrice: string; salePrice: string; salePrices: string[]; manufacturer: string; pieces: string; location: string };
  type SalesWorkflowState = {
    documentNumber: string;
    businessDocumentId: string;
    businessDocumentVersion: number;
    canonicalCommandSignature: string;
    canonicalCommandId: string;
    canonicalIdempotencyKey: string;
    customer: string;
    customerId: string;
    godownId: string;
    sourceDocumentId: string;
    sourceDocumentNumber: string;
    lookupQuery: string;
    lookupResults: ItemLookupResult[];
    reference: string;
    salePriceMode: string;
    transactionDate: string;
    dueDate: string;
    remarks: string;
    activeTab: 'detail' | 'list';
    interactive: boolean;
    history: ReportRow[];
    historyFilter: string;
    documentDiscountPercent: string;
    flatDiscountAmount: string;
    miscAmount: string;
    cashTendered: string;
    itemGstRate: string;
    itemDiscountRate: string;
    attachments: Array<{ name: string; size: number }>;
    pricingPreview: PricingPreviewResponse | null;
    pricingError: string;
    rows: SaleRow[];
    message: string;
    error: string;
  };
  const api = new AbuzarApi();
  const queue = new OfflineQueue();
  const workflowStates = new Map<string, SalesWorkflowState>();
  let session: SessionResponse['context'] = null;
  let documentNumber = '';
  let businessDocumentId = '';
  let businessDocumentVersion = 0;
  let activeWorkflowKind = '';
  let workflowRevision = 0;
  let lookupRequestId = 0;
  let pricingRequestId = 0;
  let historyRequestId = 0;
  let historySelectionRequestId = 0;
  const availabilityRequestIds = new Map<string, number>();
  let canonicalCommandSignature = '';
  let canonicalCommandId = '';
  let canonicalIdempotencyKey = '';
  let customer = 'CASH SALES CUSTOMER';
  let customerId = '';
  let customers: MasterRecord[] = [];
  let customerLoadState: 'idle' | 'loading' | 'loaded' | 'error' = 'idle';
  let godowns: MasterRecord[] = [];
  let godownId = '';
  let sourceDocumentId = '';
  let sourceDocumentNumber = '';
  let lookupQuery = '';
  let lookupBusy = false;
  let reference = '';
  let salePriceMode = 'Sale Price 1';
  let transactionDate = localDateString();
  let dueDate = '';
  let remarks = '';
  let busy = false;
  let message = '';
  let error = '';
  let pending = 0;
  let online = true;
  let activeTab: 'detail' | 'list' = 'detail';
  let interactive = false;
	let history: ReportRow[] = [];
	let historyBusy = false;
	let historyFilter = '';
  let documentDiscountPercent = '0.00';
  let flatDiscountAmount = '0.00';
  let miscAmount = '1.00';
  let cashTendered = '';
  let itemGstRate = '';
  let itemDiscountRate = '';
  let attachmentInput: HTMLInputElement | null = null;
  let attachments: Array<{ name: string; size: number }> = [];
  let pricingPreview: PricingPreviewResponse | null = null;
  let pricingBusy = false;
  let pricingError = '';
  let pricingTimer: ReturnType<typeof setTimeout> | undefined;
  let clock = new Date();
  let lookupResults: ItemLookupResult[] = [];
  let availableLookupItems: LookupItem[] = [];
  let rows: SaleRow[] = [blankRow()];
  $: kind = $page?.url?.searchParams?.get('kind') ?? 'cash';
  $: if (kind !== activeWorkflowKind) switchWorkflow(kind);
  $: workflowTitle = ({ cash: 'Cash Sale', credit: 'Credit Sale', 'cash-return': 'Cash Sale Return', 'credit-return': 'Credit Sale Return', 'open-cash-return': 'Open Cash Sale Return', 'open-credit-return': 'Open Credit Sale Return', quotation: 'Quotation', refused: 'Refused Sales' } as Record<string, string>)[kind] ?? 'Cash Sale';
  $: aggregate = ['cash-return', 'credit-return', 'open-cash-return', 'open-credit-return'].includes(kind) ? 'sale_return' : kind === 'quotation' ? 'quotation' : kind === 'refused' ? 'refused_sale' : 'sale';
  $: if (session && ['credit', 'credit-return', 'open-credit-return'].includes(kind) && customerLoadState === 'idle') void loadCustomers();
  $: availableLookupItems = lookupResults.map((record) => {
    const payload = (record.payload ?? {}) as Record<string, unknown>;
    const value = (...keys: string[]) => keys.map((key) => payload[key]).find((candidate) => candidate !== undefined && candidate !== null && String(candidate).trim() !== '');
    const text = (keys: string[], fallback = '') => String(value(...keys) ?? fallback);
    const salePrices = Array.from({ length: 10 }, (_, index) => {
      const level = index + 1;
      return text([`SalePrice${level}`, `salePrice${level}`, ...(level === 1 ? ['SalePrice', 'salePrice'] : []), 'DefaultSalePrice', 'defaultSalePrice']);
    });
    const salePrice = salePrices[selectedPriceLevel() - 1] ?? '';
    return {
      ...record,
      stock: '',
      purchasePrice: text(['PurchasePrice', 'purchasePrice', 'AvgPrice', 'AveragePrice']),
      salePrice,
      salePrices,
      manufacturer: text(['Manufacturer', 'manufacturer']),
      pieces: text(['Pieces', 'pieces', 'PackUnits', 'packUnits', 'Pcs', 'pcs'], '1'),
      location: text(['Location', 'location', 'ItemLocation', 'itemLocation'])
    };
  });

  async function loadHistory() {
    const requestRevision = workflowRevision;
    const requestId = ++historyRequestId;
    historyBusy = true;
    try {
      const result = await api.transactions(aggregate, transactionDate, transactionDate, historyFilter.trim());
      if (requestRevision !== workflowRevision || requestId !== historyRequestId) return;
      history = result.rows;
    } catch {
      if (requestRevision !== workflowRevision || requestId !== historyRequestId) return;
      history = [];
    } finally {
      if (requestRevision === workflowRevision && requestId === historyRequestId) historyBusy = false;
    }
  }

  function saleRowsFromDocument(document: Document): SaleRow[] {
    const hydrated = document.lines.map((line) => {
      const allocations = (line.allocations ?? []).map((allocation) => ({
        batchId: allocation.batchId ?? '',
        quantity: allocation.quantity || '0'
      }));
      const historicalBatches = (line.allocations ?? [])
        .filter((allocation) => allocation.batchId)
        .map((allocation) => ({
          batchId: allocation.batchId as string,
          batchNumber: allocation.batchNumber || '',
          expiryDate: allocation.expiryDate,
          quantity: allocation.quantity || '0',
          unitCost: allocation.unitCost || line.unitCost || '0.00'
        }));
      const gstRate = line.tax?.lines?.find((tax) => tax.code?.toLowerCase() === 'gst')?.rate || line.tax?.lines?.[0]?.rate || '';
      return {
        ...blankRow(),
        itemId: line.itemId,
        itemLegacyId: line.itemLegacyId || line.itemCode || line.itemId,
        sourceLineId: line.sourceLineId,
        itemName: line.itemName || line.itemCode,
        availableBatches: historicalBatches,
        allocations: allocations.length ? allocations : [blankAllocation(line.quantity || '0')],
        purchasePrice: line.unitCost || historicalBatches[0]?.unitCost || '',
        salePrice: line.price.unitPrice || line.price.netAmount || '0.00',
        salePrices: [line.price.unitPrice || line.price.netAmount || '0.00'],
        quantity: line.quantity || '0',
        discountPercent: line.price.discountPercent || '',
        gstRate,
        batchNumber: line.batchNumber || historicalBatches[0]?.batchNumber || '',
        expiryDate: line.expiryDate || historicalBatches[0]?.expiryDate || '',
        unitCost: line.unitCost || historicalBatches[0]?.unitCost || '',
        total: line.lineTotal || line.price.netAmount || '0.00'
      };
    });
    return hydrated.length ? hydrated : [blankRow()];
  }

  async function applyHistoryRow(row: ReportRow) {
    const requestRevision = workflowRevision;
    const requestId = ++historySelectionRequestId;
    historyBusy = true;
    error = '';
    try {
      if (!row.documentId) {
        documentNumber = row.document || '';
        businessDocumentId = '';
        businessDocumentVersion = 0;
        customerId = '';
        sourceDocumentId = '';
        sourceDocumentNumber = '';
        reference = '';
        dueDate = '';
        remarks = '';
        cashTendered = '';
        pricingPreview = null;
        customer = row.party || customer;
        rows = [{ ...blankRow(), itemName: row.item || '', quantity: row.quantity || '1', salePrice: row.amount || '0.00', total: row.amount || '0.00' }];
        activeTab = 'detail';
        message = `${workflowTitle} ${documentNumber || 'document'} loaded from the compatibility history summary.`;
        return;
      }
      const document = await api.document(row.documentId);
      if (requestRevision !== workflowRevision || requestId !== historySelectionRequestId) return;
      const saleDocumentKinds = ['cash-sale', 'credit-sale', 'cash-return', 'credit-return', 'open-cash-return', 'open-credit-return', 'quotation', 'refused-sale'];
      if (!saleDocumentKinds.includes(document.kind)) throw new Error('The selected history row is not a canonical sales document.');
      if (document.kind !== businessDocumentKind()) throw new Error('The selected history row does not belong to the current sales workflow.');
      documentNumber = document.documentNumber || row.document || '';
      transactionDate = document.occurredAt?.slice(0, 10) || transactionDate;
      customer = document.customer?.name || row.party || customer;
      customerId = document.customer?.id || '';
      godownId = document.godownId || '';
      sourceDocumentId = document.sourceDocumentId || '';
      sourceDocumentNumber = document.sourceDocumentNumber || '';
      reference = document.reference || '';
      dueDate = document.kind === 'credit-sale' ? document.dueDate || '' : '';
      remarks = document.remarks || '';
      salePriceMode = `Sale Price ${document.pricing?.priceLevel ?? 1}`;
      documentDiscountPercent = document.pricing?.documentPercentDiscount ?? '0.00';
      flatDiscountAmount = document.pricing?.flatDiscount ?? '0.00';
      miscAmount = document.pricing?.misc ?? '0.00';
      cashTendered = document.kind === 'cash-sale' ? document.payment?.tendered ?? document.totals.totalAmount ?? '' : '';
      pricingPreview = document.pricing ?? null;
      rows = saleRowsFromDocument(document);
      businessDocumentId = document.id;
      businessDocumentVersion = document.version;
      canonicalCommandSignature = '';
      canonicalCommandId = '';
      canonicalIdempotencyKey = '';
      activeTab = 'detail';
      message = `${workflowTitle} ${document.documentNumber || row.document || 'document'} loaded with canonical lines.`;
    } catch (cause) {
      if (requestRevision !== workflowRevision || requestId !== historySelectionRequestId) return;
      error = cause instanceof Error ? cause.message : 'The selected sales document could not be loaded.';
      message = 'The selected sales document could not be loaded.';
    } finally {
      if (requestRevision === workflowRevision && requestId === historySelectionRequestId) historyBusy = false;
    }
  }

  async function navigateHistory(offset: number) {
    const requestRevision = workflowRevision;
    if (!history.length) await loadHistory();
    if (requestRevision !== workflowRevision) return;
    if (!history.length) { message = 'No persisted documents found for this date.'; return; }
    const current = history.findIndex((row) => row.document === documentNumber);
    const next = current < 0 ? (offset > 0 ? 0 : history.length - 1) : (current + offset + history.length) % history.length;
    void applyHistoryRow(history[next]);
  }

  async function navigateHistoryTo(index: number) {
    const requestRevision = workflowRevision;
    if (!history.length) await loadHistory();
    if (requestRevision !== workflowRevision) return;
    if (!history.length) { message = 'No persisted documents found for this date.'; return; }
    void applyHistoryRow(history[index < 0 ? history.length - 1 : Math.min(index, history.length - 1)]);
  }

  async function printSaleSlip() {
    const slip = {
      header: 'WASEELA ABUZAR',
      store: 'Abuzar Software',
      invoiceNumber: documentNumber || 'UNNUMBERED',
      date: transactionDate,
      customer,
      lines: rows.filter((row) => row.itemName.trim()).map((row) => ({ itemName: row.itemName, quantity: row.quantity, total: row.total })),
      subtotal: totalAmount,
      discount: pricingPreview?.totalDiscount ?? '0.00',
      tax: pricingPreview?.taxes?.reduce((sum, tax) => sum + Number(tax.amount || 0), 0).toFixed(2) ?? '0.00',
      total: effectiveTotal,
      footer: 'Thank you'
    };
    try {
      await edgeRequest('/v1/hardware/print/sale-slip', slip);
      message = 'Sale slip sent to the branch printer.';
    } catch {
      message = 'Print preview is ready.';
      window.print();
    }
  }

  function onAttachmentsSelected(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    attachments = Array.from(input.files ?? []).map((file) => ({ name: file.name, size: file.size }));
    message = attachments.length ? `${attachments.length} document${attachments.length === 1 ? '' : 's'} attached to this ${workflowTitle.toLowerCase()} draft.` : 'No documents selected.';
  }

  $: if (activeTab === 'list' && transactionDate) void loadHistory();

  function enableInteractive(event?: Event) {
    const target = event?.target;
    if (target instanceof Element && target.closest('.legacy-transaction-tabs')) return;
    interactive = true;
  }

  function apiErrorMessage(cause: unknown, fallback: string): string {
    return cause instanceof ApiError ? cause.problem?.detail ?? cause.message : cause instanceof Error ? cause.message : fallback;
  }

  function handleMenuCommand(action: MenuAction): boolean {
    if (busy) {
      message = 'Wait for the active document command to finish.';
      return true;
    }
    switch (action.label) {
      case 'New':
        rows = [blankRow()];
        documentNumber = '';
        businessDocumentId = '';
        businessDocumentVersion = 0;
        customerId = '';
        godownId = '';
        sourceDocumentId = '';
        sourceDocumentNumber = '';
        dueDate = '';
        cashTendered = '';
        pricingPreview = null;
        message = 'New sale ready.';
        return true;
      case 'List':
        activeTab = 'list';
        void loadHistory();
        return true;
      case 'Save':
        void submitSale('draft', 'save');
        return true;
      case 'Post':
        void submitSale('posted', businessDocumentId ? 'post' : 'save-and-post');
        return true;
      case 'Save And Post':
        void submitSale('posted', 'save-and-post');
        return true;
      case 'Void':
        busy = true;
        void submitBusinessVoid().catch((cause) => { error = apiErrorMessage(cause, 'The canonical sale could not be voided.'); }).finally(() => { busy = false; });
        return true;
      case 'Print':
      case 'Sale Slip':
        void printSaleSlip();
        return true;
      case 'First':
        void navigateHistoryTo(0);
        return true;
      case 'Previous':
        void navigateHistory(-1);
        return true;
      case 'Next':
        void navigateHistory(1);
        return true;
      case 'Last':
        void navigateHistoryTo(-1);
        return true;
      case 'Item Sale History':
        activeTab = 'list';
        void loadHistory();
        message = 'Sale history loaded for the current item context.';
        return true;
      case 'Populate Items':
        if (!rows.some((row) => row.itemName.trim()) && availableLookupItems[0]) void chooseLookupItem(availableLookupItems[0]);
        message = availableLookupItems.length ? 'Item lookup populated from active canonical items.' : 'Search for an active canonical item before adding a line.';
        return true;
      case 'Auto Batch Generation': {
        const dateToken = transactionDate.replace(/[^0-9]/g, '').slice(0, 8) || localDateString().replace(/-/g, '');
        let sequence = 0;
        rows = rows.map((row) => {
          if (!row.itemName.trim()) return row;
          sequence += 1;
          return row.batchNumber.trim() ? row : { ...row, batchNumber: `AUTO-SALE-${dateToken}-${String(sequence).padStart(3, '0')}` };
        });
        message = sequence ? `Auto Batch Generation: ${sequence} batch identifier${sequence === 1 ? '' : 's'} generated.` : 'Select at least one item before generating batch identifiers.';
        return true;
      }
      case 'Apply Item GST %': {
        if (!itemGstRate.trim()) {
          message = 'Apply Item GST %: enter a rate in the transaction adjustments first.';
          return true;
        }
        let applied = 0;
        rows = rows.map((row) => {
          if (!row.itemName.trim()) return row;
          applied += 1;
          return { ...row, gstRate: itemGstRate.trim() };
        });
        message = applied ? `Apply Item GST %: ${itemGstRate.trim()}% applied to populated lines.` : 'Apply Item GST %: no populated lines to update.';
        return true;
      }
      case 'Apply Item Discount %': {
        if (!itemDiscountRate.trim()) {
          message = 'Apply Item Discount %: enter a rate in the transaction adjustments first.';
          return true;
        }
        let applied = 0;
        rows = rows.map((row) => {
          if (!row.itemName.trim()) return row;
          applied += 1;
          return { ...row, discountPercent: itemDiscountRate.trim() };
        });
        message = applied ? `Apply Item Discount %: ${itemDiscountRate.trim()}% applied to populated lines.` : 'Apply Item Discount %: no populated lines to update.';
        return true;
      }
      case 'Attach Document(s)':
        attachmentInput?.click();
        return true;
      case 'Show Document Gallery':
        message = attachments.length ? `Document Gallery: ${attachments.length} attachment${attachments.length === 1 ? '' : 's'} selected.` : 'Document Gallery: no attachments selected.';
        return true;
      case 'Import Parent Server Selected Transactions':
        void flushQueue();
        message = 'Import Parent Server Selected Transactions: branch sync requested.';
        return true;
      case 'Sort Items':
        rows = [...rows].sort((left, right) => left.itemName.localeCompare(right.itemName));
        message = 'Items sorted by name.';
        return true;
      case 'Delete Item':
        removeRow(rows.length - 1);
        message = 'Last item row deleted.';
        return true;
      case 'Restore Item':
        addRow();
        message = 'Item row restored.';
        return true;
      case 'View Item Info':
        window.location.assign('/app/master/item');
        return true;
      case 'Customer Info.':
      case 'New Customer':
        window.location.assign('/app/master/customer');
        return true;
      case 'New Item':
        window.location.assign('/app/master/item');
        return true;
      case 'Change User':
        window.location.assign('/login?changeUser=1');
        return true;
      case 'Exit':
        window.location.assign('/');
        return true;
      default:
        return false;
    }
  }

  function blankAllocation(quantity = '0'): SaleAllocation { return { batchId: '', quantity }; }
  function blankRow(): SaleRow { return { itemName: '', stock: '', availabilityLoaded: false, availableBatches: [], allocations: [blankAllocation('1')], purchasePrice: '', salePrice: '', manufacturer: '', pieces: '', location: '', quantity: '1', discountPercent: '', gstRate: '', batchNumber: '', expiryDate: '', unitCost: '', total: '0.00' }; }
  function rowAllocations(row: SaleRow): SaleAllocation[] { return row.allocations?.length ? row.allocations : [blankAllocation(row.quantity || '1')]; }
  function addRow() { rows = [...rows, blankRow()]; }
  function removeRow(index: number) { rows = rows.filter((_, rowIndex) => rowIndex !== index); if (!rows.length) rows = [blankRow()]; }
  function updateRow(index: number, key: keyof SaleRow, value: string) {
    rows = rows.map((row, rowIndex) => {
      if (rowIndex !== index) return row;
      const next = { ...row, [key]: value };
      if (key === 'itemName' && value !== row.itemName) { next.itemId = undefined; next.itemLegacyId = undefined; next.availabilityLoaded = false; next.availableBatches = []; next.allocations = [blankAllocation(next.quantity || '1')]; next.stock = ''; next.stockError = undefined; }
      if (key === 'salePrice') {
        const salePrices = [...(row.salePrices ?? [])];
        salePrices[selectedPriceLevel() - 1] = value;
        next.salePrices = salePrices;
      }
      if (key === 'quantity' && !(row.allocations ?? []).some((allocation) => allocation.batchId)) next.allocations = [blankAllocation(value || '1')];
      if (key === 'quantity' || key === 'salePrice') next.total = ((Number(next.quantity) || 0) * (Number(next.salePrice) || 0)).toFixed(2);
      return next;
    });
    queuePricingPreview();
  }

  function setRowAllocation(index: number, allocationIndex: number, key: keyof SaleAllocation, value: string) {
    rows = rows.map((row, rowIndex) => {
      if (rowIndex !== index) return row;
      const allocations = [...rowAllocations(row)];
      allocations[allocationIndex] = { ...(allocations[allocationIndex] ?? blankAllocation()), [key]: value };
      return { ...row, allocations };
    });
    if (key === 'batchId') message = value ? `Batch selected for line ${index + 1}. Enter the quantity for this allocation.` : `Automatic FIFO batch allocation restored for line ${index + 1}.`;
  }

  function addRowAllocation(index: number) {
    rows = rows.map((row, rowIndex) => rowIndex === index ? {
      ...row,
      allocations: [...rowAllocations(row), blankAllocation()]
    } : row);
  }

  function removeRowAllocation(index: number, allocationIndex: number) {
    rows = rows.map((row, rowIndex) => {
      if (rowIndex !== index) return row;
      const allocations = (row.allocations ?? []).filter((_, candidateIndex) => candidateIndex !== allocationIndex);
      return { ...row, allocations: allocations.length ? allocations : [blankAllocation(row.quantity || '1')] };
    });
  }

  function supportsExplicitBatchAllocation(): boolean {
    return ['cash', 'credit', 'cash-return', 'credit-return'].includes(kind);
  }

  function repriceRowsForSelectedTier() {
    const priceLevel = selectedPriceLevel();
    rows = rows.map((row) => {
      const selectedPrice = row.salePrices?.[priceLevel - 1]?.trim();
      if (!row.itemId || selectedPrice === undefined || selectedPrice === '') return row;
      return { ...row, salePrice: selectedPrice, total: ((Number(row.quantity) || 0) * (Number(selectedPrice) || 0)).toFixed(2) };
    });
    queuePricingPreview();
  }

  function freshWorkflowState(): SalesWorkflowState {
    return {
      documentNumber: '',
      businessDocumentId: '',
      businessDocumentVersion: 0,
      canonicalCommandSignature: '',
      canonicalCommandId: '',
      canonicalIdempotencyKey: '',
      customer: 'CASH SALES CUSTOMER',
      customerId: '',
      godownId: '',
      sourceDocumentId: '',
      sourceDocumentNumber: '',
      lookupQuery: '',
      lookupResults: [],
      reference: '',
      salePriceMode: 'Sale Price 1',
      transactionDate: localDateString(),
      dueDate: '',
      remarks: '',
      activeTab: 'detail',
      interactive: false,
      history: [],
      historyFilter: '',
      documentDiscountPercent: '0.00',
      flatDiscountAmount: '0.00',
      miscAmount: '1.00',
      cashTendered: '',
      itemGstRate: '',
      itemDiscountRate: '',
      attachments: [],
      pricingPreview: null,
      pricingError: '',
      rows: [blankRow()],
      message: '',
      error: ''
    };
  }

  function captureWorkflowState(): SalesWorkflowState {
    return {
      documentNumber,
      businessDocumentId,
      businessDocumentVersion,
      canonicalCommandSignature,
      canonicalCommandId,
      canonicalIdempotencyKey,
      customer,
      customerId,
      godownId,
      sourceDocumentId,
      sourceDocumentNumber,
      lookupQuery,
      lookupResults,
      reference,
      salePriceMode,
      transactionDate,
      dueDate,
      remarks,
      activeTab,
      interactive,
      history,
      historyFilter,
      documentDiscountPercent,
      flatDiscountAmount,
      miscAmount,
      cashTendered,
      itemGstRate,
      itemDiscountRate,
      attachments,
      pricingPreview,
      pricingError,
      rows,
      message,
      error
    };
  }

  function applyWorkflowState(state: SalesWorkflowState) {
    documentNumber = state.documentNumber;
    businessDocumentId = state.businessDocumentId;
    businessDocumentVersion = state.businessDocumentVersion;
    canonicalCommandSignature = state.canonicalCommandSignature;
    canonicalCommandId = state.canonicalCommandId;
    canonicalIdempotencyKey = state.canonicalIdempotencyKey;
    customer = state.customer;
    customerId = state.customerId;
    godownId = state.godownId;
    sourceDocumentId = state.sourceDocumentId;
    sourceDocumentNumber = state.sourceDocumentNumber;
    lookupQuery = state.lookupQuery;
    lookupResults = state.lookupResults;
    reference = state.reference;
    salePriceMode = state.salePriceMode;
    transactionDate = state.transactionDate;
    dueDate = state.dueDate;
    remarks = state.remarks;
    activeTab = state.activeTab;
    interactive = state.interactive;
    history = state.history;
    historyFilter = state.historyFilter;
    documentDiscountPercent = state.documentDiscountPercent;
    flatDiscountAmount = state.flatDiscountAmount;
    miscAmount = state.miscAmount;
    cashTendered = state.cashTendered;
    itemGstRate = state.itemGstRate;
    itemDiscountRate = state.itemDiscountRate;
    attachments = state.attachments;
    pricingPreview = state.pricingPreview;
    pricingError = state.pricingError;
    rows = state.rows;
    message = state.message;
    error = state.error;
  }

  function switchWorkflow(nextKind: string) {
    if (pricingTimer) clearTimeout(pricingTimer);
    lookupRequestId += 1;
    pricingRequestId += 1;
    historyRequestId += 1;
    historySelectionRequestId += 1;
    if (activeWorkflowKind) workflowStates.set(activeWorkflowKind, captureWorkflowState());
    activeWorkflowKind = nextKind;
    workflowRevision += 1;
    historyBusy = false;
    lookupBusy = false;
    pricingBusy = false;
    applyWorkflowState(workflowStates.get(nextKind) ?? freshWorkflowState());
    if (godownId && rows.some((row) => row.itemId && !row.availabilityLoaded)) void refreshAllAvailability();
  }

  async function loadCustomers() {
    const requestRevision = workflowRevision;
    customerLoadState = 'loading';
    try {
      const customerResult = await api.masterRecords('customer');
      customers = customerResult.records.filter((record) => record.active);
      customerLoadState = 'loaded';
    } catch (cause) {
      customers = [];
      customerLoadState = requestRevision === workflowRevision ? 'error' : 'idle';
      if (requestRevision === workflowRevision) error = apiErrorMessage(cause, 'Canonical customers could not be loaded.');
    }
  }

  async function searchItems(value = lookupQuery) {
    const requestRevision = workflowRevision;
    const requestId = ++lookupRequestId;
    lookupQuery = value;
    const query = value.trim();
    if (!query) {
      lookupResults = [];
      lookupBusy = false;
      return;
    }
    lookupBusy = true;
    try {
      const result = await api.itemLookup(query);
      if (requestRevision !== workflowRevision || requestId !== lookupRequestId || query !== lookupQuery.trim()) return;
      lookupResults = result.items.filter((item) => item.active && item.id);
      error = '';
    } catch (cause) {
      if (requestRevision !== workflowRevision || requestId !== lookupRequestId || query !== lookupQuery.trim()) return;
      lookupResults = [];
      error = apiErrorMessage(cause, 'Item lookup could not be loaded.');
    } finally {
      if (requestRevision === workflowRevision && requestId === lookupRequestId) lookupBusy = false;
    }
  }

  async function refreshRowAvailability(index: number) {
    const row = rows[index];
    if (!row?.itemId || !row.itemLegacyId || !godownId) return;
    const requestRevision = workflowRevision;
    const requestItemId = row.itemId;
    const requestGodownId = godownId;
    const requestKey = `${requestRevision}:${index}:${requestItemId}:${requestGodownId}`;
    const requestId = (availabilityRequestIds.get(requestKey) ?? 0) + 1;
    availabilityRequestIds.set(requestKey, requestId);
    rows = rows.map((candidate, rowIndex) => rowIndex === index ? { ...candidate, stock: 'Loading…', availabilityLoaded: false, stockError: undefined } : candidate);
    try {
      const result = await api.inventoryAvailability(row.itemLegacyId, requestGodownId);
      if (requestRevision !== workflowRevision || godownId !== requestGodownId || rows[index]?.itemId !== requestItemId || availabilityRequestIds.get(requestKey) !== requestId) return;
      const batches = result.batches.filter((batch) => Number(batch.quantity || 0) > 0);
      const available = batches.reduce((sum, batch) => sum + Number(batch.quantity || 0), 0).toFixed(4).replace(/\.?0+$/, '');
      rows = rows.map((candidate, rowIndex) => rowIndex === index ? {
        ...candidate,
        stock: available || '0',
        availabilityLoaded: true,
        availableBatches: batches,
        allocations: (() => {
          const allocations = (candidate.allocations ?? []).filter((allocation) => !allocation.batchId || batches.some((batch) => batch.batchId === allocation.batchId));
          return allocations.length ? allocations : [blankAllocation(candidate.quantity || '1')];
        })(),
        stockError: undefined
      } : candidate);
    } catch (cause) {
      if (requestRevision !== workflowRevision || godownId !== requestGodownId || rows[index]?.itemId !== requestItemId || availabilityRequestIds.get(requestKey) !== requestId) return;
      rows = rows.map((candidate, rowIndex) => rowIndex === index ? { ...candidate, stock: 'Unavailable', availabilityLoaded: false, stockError: apiErrorMessage(cause, 'Stock availability could not be loaded.') } : candidate);
    } finally {
      if (availabilityRequestIds.get(requestKey) === requestId) availabilityRequestIds.delete(requestKey);
    }
  }

  async function refreshAllAvailability() {
    await Promise.all(rows.map((_, index) => refreshRowAvailability(index)));
  }

  async function chooseLookupItem(item: LookupItem) {
    if (!item.active || !item.id) {
      error = 'Only active canonical items can be selected.';
      return;
    }
    let index = rows.findIndex((row) => !row.itemName.trim());
    if (index < 0) { rows = [...rows, blankRow()]; index = rows.length - 1; }
    rows = rows.map((row, rowIndex) => rowIndex === index ? { ...row, itemId: item.id, itemLegacyId: item.legacyId, itemName: item.name, stock: godownId ? 'Loading…' : 'Select godown', availabilityLoaded: false, availableBatches: [], allocations: [blankAllocation(row.quantity || '1')], stockError: undefined, purchasePrice: item.purchasePrice, salePrice: item.salePrice, salePrices: [...item.salePrices], manufacturer: item.manufacturer, pieces: item.pieces, location: item.location, quantity: row.quantity || '1', total: '0.00' } : row);
    queuePricingPreview();
    await refreshRowAvailability(index);
  }
  $: totalAmount = rows.reduce((sum, row) => sum + (Number(row.total) || ((Number(row.salePrice) || 0) * (Number(row.quantity) || 0))), 0).toFixed(2);
  $: effectiveTotal = pricingPreview?.total ?? totalAmount;
  $: cashTenderedValue = cashTendered.trim() || effectiveTotal;
  $: cashBack = (() => {
    const tendered = Number(cashTenderedValue);
    const total = Number(effectiveTotal);
    return Number.isFinite(tendered) && Number.isFinite(total) ? Math.max(0, tendered - total).toFixed(2) : '0.00';
  })();
  $: selectedStockTotal = rows.filter((row) => row.itemId && row.availabilityLoaded).reduce((sum, row) => sum + (Number(row.stock) || 0), 0).toFixed(2);
  $: transactionWindowTitle = `${formatLegacyTitle(session?.username, clock)} - [${workflowTitle}]`;

  function selectedPriceLevel(): number {
    const match = salePriceMode.match(/(\d+)/);
    return Math.min(10, Math.max(1, Number(match?.[1] ?? 1)));
  }

  function pricingRequest(): PricingPreviewRequest | null {
    const pricedRows = rows.filter((row) => row.itemName.trim());
    if (!pricedRows.length) return null;
    const priceLevel = selectedPriceLevel();
    return {
      priceLevel,
      lines: pricedRows.map((row, index) => {
        const unitPrice = (row.salePrice || '0').replace(/,/g, '').trim() || '0';
        const prices = Array.from({ length: priceLevel }, (_, tier) => (row.salePrices?.[tier] || unitPrice).replace(/,/g, '').trim() || unitPrice);
        return {
          id: row.itemId || `${row.itemName.trim()}#${index + 1}`,
          quantity: row.quantity.trim() || '0',
          prices,
          ...(row.discountPercent.trim() ? { itemDiscountPercent: row.discountPercent.trim() } : {})
        };
      }),
      documentDiscountPercent: documentDiscountPercent || '0',
      flatDiscountAmount: flatDiscountAmount || '0',
      miscAmount: miscAmount || '0'
    };
  }

  function queuePricingPreview() {
    pricingPreview = null;
    pricingError = '';
    if (pricingTimer) clearTimeout(pricingTimer);
    pricingTimer = setTimeout(() => {
      pricingTimer = undefined;
      void refreshPricingPreview();
    }, 300);
  }

  async function refreshPricingPreview(): Promise<PricingPreviewResponse | null> {
    if (!online || !session) return null;
    const requestRevision = workflowRevision;
    const requestId = ++pricingRequestId;
    const request = pricingRequest();
    if (!request) {
      pricingBusy = false;
      return null;
    }
    pricingBusy = true;
    try {
      const result = await api.previewPricing(request);
      if (requestRevision !== workflowRevision || requestId !== pricingRequestId) return null;
      pricingPreview = result;
      pricingError = '';
      return result;
    } catch (cause) {
      if (requestRevision !== workflowRevision || requestId !== pricingRequestId) return null;
      pricingPreview = null;
      pricingError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : cause instanceof Error ? cause.message : 'Pricing could not be calculated.';
      return null;
    } finally {
      if (requestRevision === workflowRevision && requestId === pricingRequestId) pricingBusy = false;
    }
  }

  onMount(() => {
    const clockTimer = window.setInterval(() => { clock = new Date(); }, 1000);
    online = navigator.onLine;
    const updateOnline = () => (online = navigator.onLine);
    window.addEventListener('online', updateOnline);
    window.addEventListener('offline', updateOnline);
    void (async () => {
      try {
        const response = await api.session();
        if (!response.authenticated || !response.context) { window.location.assign('/login'); return; }
        session = response.context;
        pending = (await queue.pending()).length;
        const godownResult = await api.masterRecords('godown');
        godowns = godownResult.records.filter((record) => record.active);
      } catch (cause) { error = apiErrorMessage(cause, 'The API session or canonical sales context could not be loaded.'); }
    })();
    return () => { window.clearInterval(clockTimer); window.removeEventListener('online', updateOnline); window.removeEventListener('offline', updateOnline); };
  });

  beforeNavigate((navigation) => {
    if (busy) navigation.cancel();
  });

  function makeEvent(calculated?: PricingPreviewResponse | null, status: 'draft' | 'posted' | 'voided' = 'posted'): SyncEnvelope {
    if (!session?.tenantId || !session.branchId || !session.counterId || !session.operatorId) throw new Error('Select a branch and counter before posting a sale.');
    const eventId = newEventId();
    return { eventId, aggregate, aggregateId: eventId, tenantId: session.tenantId, branchId: session.branchId, counterId: session.counterId, operatorId: session.operatorId, occurredAt: localDateAtNoonUtc(transactionDate), idempotencyKey: `${aggregate}:${documentNumber || eventId}:${status}`, schemaVersion: 1, payload: { documentNumber, customer, godownId, reference, salePriceMode, dueDate, remarks, rows, totalAmount: calculated?.total ?? effectiveTotal, pricing: calculated, pricingRequest: calculated ? pricingRequest() : null, ...(kind === 'cash' ? { payment: { mode: 'cash', received: effectiveTotal, tendered: cashTenderedValue, change: cashBack } } : {}), status } };
  }

  function businessDocumentKind(): 'cash-sale' | 'credit-sale' | 'cash-return' | 'credit-return' | 'open-cash-return' | 'open-credit-return' | 'quotation' | 'refused-sale' | undefined {
    if (kind === 'cash') return 'cash-sale';
    if (kind === 'credit') return 'credit-sale';
    if (kind === 'cash-return') return 'cash-return';
    if (kind === 'credit-return') return 'credit-return';
    if (kind === 'open-cash-return') return 'open-cash-return';
    if (kind === 'open-credit-return') return 'open-credit-return';
    if (kind === 'quotation') return 'quotation';
    if (kind === 'refused') return 'refused-sale';
    return undefined;
  }

  function validateExplicitAllocations(row: SaleRow, lineIndex: number) {
    const allocations = rowAllocations(row).filter((allocation) => allocation.batchId);
    if (!allocations.length) return;
    const quantities = allocations.map((allocation) => Number(allocation.quantity.trim()));
    if (allocations.some((allocation, index) => !/^\d+(?:\.\d{1,4})?$/.test(allocation.quantity.trim()) || !Number.isFinite(quantities[index]) || quantities[index] <= 0)) {
      throw new Error(`Enter a positive batch quantity with no more than four decimals for line ${lineIndex + 1}.`);
    }
    const batchIds = allocations.map((allocation) => allocation.batchId);
    if (new Set(batchIds).size !== batchIds.length) throw new Error(`Select each batch only once for line ${lineIndex + 1}.`);
    const allocated = quantities.reduce((sum, quantity) => sum + quantity, 0);
    const requested = Number(row.quantity.trim());
    if (!Number.isFinite(requested) || Math.abs(allocated - requested) > 0.00000001) throw new Error(`Batch allocations for line ${lineIndex + 1} must total the line quantity (${row.quantity || '0'}).`);
  }

  function canonicalValidation(action: 'save' | 'post' | 'save-and-post') {
    const documentKind = businessDocumentKind();
    const returnKind = documentKind === 'cash-return' || documentKind === 'credit-return';
    const openReturnKind = documentKind === 'open-cash-return' || documentKind === 'open-credit-return';
    const anyReturnKind = returnKind || openReturnKind;
    const stockBearing = documentKind === 'cash-sale' || documentKind === 'credit-sale' || anyReturnKind;
    const requiresAvailability = documentKind === 'cash-sale' || documentKind === 'credit-sale';
    const lineRows = rows.filter((row) => row.itemName.trim() || row.itemId);
    if (!lineRows.length) throw new Error('Enter at least one item selected from the active canonical item lookup.');
    if (lineRows.some((row) => !row.itemId || !row.itemLegacyId)) throw new Error('Select an active canonical item from lookup for every sale line.');
    if (stockBearing && (!godownId || !godowns.some((godown) => godown.id === godownId && godown.active))) throw new Error('Select an active godown before saving or posting the sale.');
    if ((kind === 'credit' || kind === 'credit-return' || kind === 'open-credit-return') && (!customerId || !customers.some((party) => party.id === customerId && party.active))) throw new Error(kind === 'credit-return' || kind === 'open-credit-return' ? 'Select an active canonical customer for a sale return.' : 'Select an active canonical customer for a credit sale.');
    if (documentKind === 'cash-sale' && cashTendered.trim()) {
      const tendered = Number(cashTendered);
      const total = Number(effectiveTotal);
      if (!Number.isFinite(tendered) || !Number.isFinite(total) || tendered < total) throw new Error('Cash tendered must be at least the calculated total.');
    }
    if (returnKind && action !== 'save' && !sourceDocumentId.trim()) throw new Error('Enter the canonical source sale document ID before posting a sale return.');
    if (returnKind && action !== 'save' && lineRows.some((row) => !row.sourceLineId?.trim())) throw new Error('Enter the canonical source sale line ID for every posted sale return line.');
    if (requiresAvailability && action !== 'save') {
      for (const row of lineRows) {
        if (!row.availabilityLoaded) throw new Error(`Stock availability is not loaded for ${row.itemName}. Select a godown and refresh availability.`);
        if (Number(row.quantity) > Number(row.stock)) throw new Error(`Insufficient available stock for ${row.itemName}.`);
      }
    }
    if (action !== 'save' && supportsExplicitBatchAllocation()) lineRows.forEach((row, index) => validateExplicitAllocations(row, index));
    if (documentKind !== 'credit-sale' && dueDate.trim()) throw new Error('Due Date is only valid for credit sales.');
    if (documentKind === 'credit-sale' && dueDate.trim() && !/^\d{4}-\d{2}-\d{2}$/.test(dueDate.trim())) throw new Error('Due Date must use the YYYY-MM-DD format.');
    return lineRows;
  }

  function businessDocumentCommand(action: 'save' | 'post' | 'save-and-post'): DocumentCommandForKind<'cash-sale' | 'credit-sale' | 'cash-return' | 'credit-return' | 'open-cash-return' | 'open-credit-return' | 'quotation' | 'refused-sale'> {
    const documentKind = businessDocumentKind();
    if (!documentKind) throw new Error('This sales route is not available in the canonical document lifecycle.');
    const lineRows = canonicalValidation(action);
    const lines = lineRows.map((row, index) => {
      const requestedAllocations = rowAllocations(row)
        .filter((allocation) => allocation.batchId)
        .map((allocation) => ({
          batchId: allocation.batchId,
          batchNumber: row.availableBatches?.find((batch) => batch.batchId === allocation.batchId)?.batchNumber ?? '',
          quantity: allocation.quantity || '0'
        }));
      return {
        lineNumber: index + 1,
        itemId: row.itemId as string,
        ...(row.sourceLineId?.trim() ? { sourceLineId: row.sourceLineId.trim() } : {}),
        quantity: row.quantity || '0',
        unitPrice: (row.salePrice || '0').replace(/,/g, ''),
        discountPercent: row.discountPercent || '0',
        ...(row.gstRate.trim() ? { gstRate: row.gstRate.trim() } : {}),
        ...(supportsExplicitBatchAllocation() && requestedAllocations.length ? { allocations: requestedAllocations } : {}),
        ...(row.batchNumber.trim() ? { batchNumber: row.batchNumber.trim() } : {}),
        ...(row.expiryDate.trim() ? { expiryDate: row.expiryDate.trim() } : {}),
        ...(row.unitCost.trim() ? { unitCost: row.unitCost.trim() } : {})
      };
    });
    const signature = JSON.stringify({
      documentKind,
      action,
      documentId: businessDocumentId,
      version: businessDocumentVersion,
      documentNumber,
      occurredAt: transactionDate,
      sourceDocumentId,
      sourceDocumentNumber,
      godownId,
      customerId,
      reference,
      dueDate,
      remarks,
      priceLevel: selectedPriceLevel(),
      lines,
      documentDiscountPercent,
      flatDiscountAmount,
      miscAmount,
      cashTendered
    });
    if (signature !== canonicalCommandSignature) {
      canonicalCommandSignature = signature;
      canonicalCommandId = newEventId();
      canonicalIdempotencyKey = `canonical:${documentKind}:${newEventId()}`;
    }
    const base = {
      commandId: canonicalCommandId,
      kind: documentKind,
      action,
      idempotencyKey: canonicalIdempotencyKey,
      occurredAt: localDateAtNoonUtc(transactionDate),
      document: {
        ...(businessDocumentId ? { id: businessDocumentId } : {}),
        kind: documentKind,
        documentNumber,
        occurredAt: localDateAtNoonUtc(transactionDate),
        ...(documentKind === 'credit-sale' || documentKind === 'credit-return' || documentKind === 'open-credit-return' ? { customerId } : {}),
        ...(documentKind === 'cash-sale' || documentKind === 'credit-sale' || documentKind === 'cash-return' || documentKind === 'credit-return' || documentKind === 'open-cash-return' || documentKind === 'open-credit-return' ? { godownId } : {}),
        ...(documentKind === 'cash-return' || documentKind === 'credit-return' ? { sourceDocumentId, sourceDocumentNumber } : {}),
        reference,
        remarks,
        ...(documentKind === 'credit-sale' && dueDate.trim() ? { dueDate: dueDate.trim() } : {}),
        lines,
        priceLevel: selectedPriceLevel(),
        flatDiscountAmount,
        miscAmount,
        documentDiscountPercent,
        pricing: { priceLevel: selectedPriceLevel(), documentDiscountPercent, flatDiscountAmount, miscAmount },
        ...(documentKind === 'cash-sale' ? { payment: { mode: 'cash', received: effectiveTotal, tendered: cashTenderedValue, change: cashBack } } : {})
      }
    };
    return businessDocumentVersion > 0 ? { ...base, expectedVersion: businessDocumentVersion } as DocumentCommandForKind<'cash-sale' | 'credit-sale' | 'cash-return' | 'credit-return' | 'open-cash-return' | 'open-credit-return' | 'quotation' | 'refused-sale'> : base as DocumentCommandForKind<'cash-sale' | 'credit-sale' | 'cash-return' | 'credit-return' | 'open-cash-return' | 'open-credit-return' | 'quotation' | 'refused-sale'>;
  }

  async function submitBusinessDocument(action: 'save' | 'post' | 'save-and-post') {
    const requestRevision = workflowRevision;
    const documentKind = businessDocumentKind();
    const command = businessDocumentCommand(action);
    if (!documentKind) throw new Error('Canonical sales document kind is unavailable.');
    const response = await api.documentCommand(documentKind, command);
    if (requestRevision !== workflowRevision || documentKind !== businessDocumentKind()) return;
    if (!response.accepted) throw new Error(response.errors.map((item) => item.message).join('; ') || 'The canonical document command was rejected.');
    if (action === 'save' && response.status !== 'draft') throw new Error(`Canonical save returned status ${response.status}; it was not saved as a draft.`);
    if ((action === 'post' || action === 'save-and-post') && response.status !== 'posted') throw new Error(`Canonical sale was accepted with status ${response.status}; it was not posted.`);
    businessDocumentId = response.document.id;
    businessDocumentVersion = response.document.version;
    documentNumber = response.document.documentNumber ?? documentNumber;
    canonicalCommandSignature = '';
    canonicalCommandId = '';
    canonicalIdempotencyKey = '';
    message = `${workflowTitle} ${action === 'save' ? 'saved as draft' : 'posted'} successfully.`;
  }

  async function submitBusinessVoid() {
    const requestRevision = workflowRevision;
    const documentKind = businessDocumentKind();
    if (!documentKind || !businessDocumentId) throw new Error('Load or save a canonical sale before voiding it.');
    const signature = JSON.stringify({ documentKind, action: 'void', documentId: businessDocumentId, version: businessDocumentVersion, reason: 'Voided from sales workflow' });
    if (signature !== canonicalCommandSignature) {
      canonicalCommandSignature = signature;
      canonicalCommandId = newEventId();
      canonicalIdempotencyKey = `canonical:${documentKind}:void:${businessDocumentId}:${businessDocumentVersion}`;
    }
    const command: DocumentCommandForKind<'cash-sale' | 'credit-sale' | 'cash-return' | 'credit-return' | 'open-cash-return' | 'open-credit-return' | 'quotation' | 'refused-sale'> = {
      commandId: canonicalCommandId,
      kind: documentKind,
      action: 'void',
      idempotencyKey: canonicalIdempotencyKey,
      occurredAt: localDateAtNoonUtc(transactionDate),
      expectedVersion: businessDocumentVersion,
      documentId: businessDocumentId,
      reason: 'Voided from sales workflow'
    };
    const response = await api.documentCommand(documentKind, command);
    if (requestRevision !== workflowRevision || documentKind !== businessDocumentKind()) return;
    if (!response.accepted) throw new Error(response.errors.map((item) => item.message).join('; ') || 'The canonical void command was rejected.');
    if (response.status !== 'void') throw new Error(`Canonical void returned status ${response.status}.`);
    businessDocumentVersion = response.document.version;
    canonicalCommandSignature = '';
    canonicalCommandId = '';
    canonicalIdempotencyKey = '';
    message = `${workflowTitle} voided successfully.`;
  }

  async function submitSale(status: 'draft' | 'posted' = 'posted', action?: 'save' | 'post' | 'save-and-post') {
    const requestRevision = workflowRevision;
    busy = true; message = ''; error = '';
    let event: SyncEnvelope | undefined;
    try {
      if (!businessDocumentKind() && !rows.some((row) => row.itemName.trim())) throw new Error('Enter at least one item before posting.');
      const canonicalAction = action ?? (status === 'draft' ? 'save' : 'save-and-post');
      if (businessDocumentKind()) {
        if (!online) throw new Error('Canonical cash and credit sales require an online API connection; they are not placed in the legacy compatibility queue.');
        canonicalValidation(canonicalAction);
      }
      if (pricingTimer) {
        clearTimeout(pricingTimer);
        pricingTimer = undefined;
      }
      const calculated = await refreshPricingPreview();
      if (requestRevision !== workflowRevision) return;
      if (pricingError && online) throw new Error(pricingError);
      if (businessDocumentKind() && !calculated) throw new Error('Authoritative pricing did not complete. Review the sale and try again.');
      if (businessDocumentKind()) {
        if (action) await submitBusinessDocument(action);
        else await submitBusinessDocument(canonicalAction);
        if (requestRevision !== workflowRevision) return;
        if (aggregate === 'sale' && kind === 'cash') void edgeRequest('/v1/hardware/cash-drawer/kick', {}).catch(() => undefined);
        return;
      }
      event = makeEvent(calculated, status);
      if (!online) { await queue.enqueue(event); pending = (await queue.pending()).length; message = 'Sale saved to the local branch queue.'; return; }
      if (aggregate === 'sale_return') await api.createSaleReturn(event);
      else if (aggregate === 'quotation') await api.createQuotation(event);
      else if (aggregate === 'refused_sale') await api.createRefusedSale(event);
      else await api.createSale(event);
      if (aggregate === 'sale' && kind === 'cash') void edgeRequest('/v1/hardware/cash-drawer/kick', {}).catch(() => undefined);
      message = status === 'draft' ? `${workflowTitle} saved as draft.` : `${workflowTitle} posted successfully.`;
    } catch (cause) {
      if (requestRevision !== workflowRevision) return;
      if (event && (cause instanceof TypeError || !online)) { await queue.enqueue(event); pending = (await queue.pending()).length; message = 'Central API unavailable; sale saved to the legacy compatibility queue.'; }
      else if (cause instanceof ApiError && cause.status === 401) error = 'Your session has expired. Sign in again.';
      else error = cause instanceof Error ? cause.message : 'Sale could not be posted.';
    } finally { busy = false; }
  }

  async function flushQueue() {
    busy = true; message = ''; error = '';
    try {
      const configuredEdgeUrl = window.localStorage.getItem('abuzar.edgeUrl');
      const edgeUrl = configuredEdgeUrl || (['localhost', '127.0.0.1'].includes(window.location.hostname) ? 'http://127.0.0.1:8091' : '');
      if (!edgeUrl) throw new Error('Configure the branch-edge URL before syncing the offline queue.');
      const result = await queue.flush(edgeUrl, 500, window.localStorage.getItem('abuzar.edgeSecret') ?? '');
      pending = (await queue.pending()).length; message = `${result.accepted} queued event(s) accepted; ${result.duplicates} duplicate(s).`;
    } catch (cause) { error = cause instanceof Error ? cause.message : 'Offline queue sync failed.'; }
    finally { busy = false; }
  }
</script>

<svelte:window onkeydown={enableInteractive} />

<svelte:head><title>WASEELA · ABUZAR V3 · {workflowTitle}</title></svelte:head>

<main class:legacy-sales-cash-page={kind === 'cash'} class:legacy-sales-cash-baseline={kind === 'cash' && activeTab === 'detail' && !interactive} class:legacy-sales-list-page={activeTab === 'list'} class="legacy-transaction-page" onpointerdown={enableInteractive} onfocusin={enableInteractive}>
  <section class="legacy-transaction-window" aria-label={workflowTitle}>
<header class="legacy-transaction-titlebar"><a href="/app/legacy" aria-label="Back to main window">←</a><h1 aria-label={kind === 'cash' ? 'New sale' : workflowTitle}>{transactionWindowTitle}</h1></header>
    <LegacyMenuBar context="cash-sale" windowId={'sales-' + kind} windowLabel={workflowTitle} windowHref={'/app/sales?kind=' + kind} navigationBlocked={busy} onCommand={handleMenuCommand} />
    <div class="legacy-transaction-toolbar" role="toolbar" aria-label="Sale toolbar">
      <button type="button" aria-label="New sale" onclick={() => { rows = [blankRow()]; documentNumber = ''; businessDocumentId = ''; businessDocumentVersion = 0; dueDate = ''; cashTendered = ''; pricingPreview = null; message = 'New sale ready.'; }} disabled={busy} title="New">▱</button>
      <button type="button" aria-label="Post sale" onclick={() => { void submitSale('posted', businessDocumentId ? 'post' : 'save-and-post'); }} disabled={busy} title="Post sale">▣</button>
      <button type="button" aria-label="Void sale" onclick={() => { busy = true; error = ''; void submitBusinessVoid().catch((cause) => { error = apiErrorMessage(cause, 'The canonical sale could not be voided.'); }).finally(() => { busy = false; }); }} disabled={busy || !businessDocumentId} title="Void sale">⊘</button>
      <button type="button" aria-label="Print sale" onclick={() => { void printSaleSlip(); }} title="Print">▤</button>
      <span class="legacy-toolbar-separator"></span><button type="button" aria-label="Previous sale" onclick={() => { void navigateHistory(-1); }} disabled={busy} title="Previous">◀</button><button type="button" aria-label="Next sale" onclick={() => { void navigateHistory(1); }} disabled={busy} title="Next">▶</button>
      <span class="legacy-toolbar-caption">{online ? 'Online' : 'Offline'} · {workflowTitle}</span>
    </div>
    <div class="legacy-transaction-tabs"><button aria-label="Detail" aria-pressed={activeTab === 'detail'} class:active={activeTab === 'detail'} type="button" onclick={() => { interactive = true; activeTab = 'detail'; }} disabled={busy}>▦ Detail</button><button data-testid="sales-list-tab" aria-label="List" aria-pressed={activeTab === 'list'} class:active={activeTab === 'list'} type="button" onclick={() => { interactive = true; activeTab = 'list'; void loadHistory(); }} disabled={busy}>▦ List</button></div>
    <div class="legacy-transaction-detail" inert={busy} aria-busy={busy}>
      <div class="legacy-sale-fields">
        <label>Inv. No:<input bind:value={documentNumber} /></label><label>Date:<input type="date" bind:value={transactionDate} /></label>
        {#if kind === 'credit'}<label>Due Date:<input aria-label="Due date" type="date" bind:value={dueDate} /></label>{/if}
        <label>User:<input value={session?.username ?? ''} readonly /></label><label>Godown:<select aria-label="Godown" bind:value={godownId} onchange={() => void refreshAllAvailability()}><option value="">Select active godown</option>{#each godowns as godown}<option value={godown.id}>{godown.name}</option>{/each}</select></label>
        <label>Alias Name:<input aria-label="Item lookup query" bind:value={lookupQuery} oninput={(event) => void searchItems((event.currentTarget as HTMLInputElement).value)} onkeydown={(event) => { if (event.key === 'Enter') void searchItems((event.currentTarget as HTMLInputElement).value); }} /></label><label>Customer:{#if kind === 'credit' || kind === 'credit-return' || kind === 'open-credit-return'}<select aria-label="Customer" bind:value={customerId} onchange={() => { const selected = customers.find((party) => party.id === customerId); customer = selected?.name ?? ''; }}><option value="">Select active customer</option>{#each customers as party}<option value={party.id}>{party.name}</option>{/each}</select>{:else}<input bind:value={customer} readonly />{/if}</label>
        {#if kind === 'cash-return' || kind === 'credit-return'}<label>Source Inv. ID:<input aria-label="Source document ID" bind:value={sourceDocumentId} /></label><label>Source Inv. No.:<input aria-label="Source document number" bind:value={sourceDocumentNumber} /></label>{/if}
        <label>Ref.:<input bind:value={reference} /></label><label>Remarks:<input bind:value={remarks} /></label>
        <label>SalePrice:#<select aria-label="Sale price tier" bind:value={salePriceMode} onchange={repriceRowsForSelectedTier}>{#each Array(10) as _, index}<option>Sale Price {index + 1}</option>{/each}</select></label>
      </div>
      <div class="legacy-sale-lookup" aria-label="Item lookup list">
        <table><thead><tr><th>Name</th><th>Stock</th><th>Purchase Price</th><th>Sale Price</th><th>Manufacturer</th><th>P/Pcs.</th><th>Location</th></tr></thead><tbody>
          {#if lookupBusy}<tr><td colspan="7">Looking up active canonical items…</td></tr>{:else if availableLookupItems.length === 0}<tr><td colspan="7">Search by item name, alias, barcode, or code. No demo items are available.</td></tr>{:else}{#each availableLookupItems as item}<tr><td><button type="button" onclick={() => chooseLookupItem(item)}>{item.name}</button></td><td>{item.stock || 'Select godown'}</td><td>{item.purchasePrice}</td><td>{item.salePrice}</td><td>{item.manufacturer}</td><td>{item.pieces}</td><td>{item.location}</td></tr>{/each}{/if}
        </tbody></table>
      </div>
      <div class="legacy-sale-grid-wrap"><table class="legacy-sale-grid"><thead><tr><th>No.</th><th>Item Name</th>{#if kind === 'cash-return' || kind === 'credit-return'}<th>Source Sale Line ID</th>{/if}{#if kind === 'open-cash-return' || kind === 'open-credit-return'}<th>Batch</th><th>Expiry</th><th>Unit Cost</th>{/if}<th>Stock</th><th>Purchase Price</th><th>Sale Price</th><th>Manufacturer</th><th>P/Pcs.</th><th>Location</th><th>Qty</th><th>Total</th><th></th></tr></thead><tbody>
        {#each rows as row, index}<tr><td>{index + 1}</td><td><input aria-label={`Item name ${index + 1}`} value={row.itemName} readonly={Boolean(row.itemId)} oninput={(event) => updateRow(index, 'itemName', event.currentTarget.value)} /></td>{#if kind === 'cash-return' || kind === 'credit-return'}<td><input aria-label={`Source sale line ID ${index + 1}`} value={row.sourceLineId ?? ''} oninput={(event) => updateRow(index, 'sourceLineId', event.currentTarget.value)} /></td>{/if}{#if kind === 'open-cash-return' || kind === 'open-credit-return'}<td><input aria-label={`Batch ${index + 1}`} value={row.batchNumber} oninput={(event) => updateRow(index, 'batchNumber', event.currentTarget.value)} /></td><td><input aria-label={`Expiry ${index + 1}`} type="date" value={row.expiryDate} oninput={(event) => updateRow(index, 'expiryDate', event.currentTarget.value)} /></td><td><input aria-label={`Unit cost ${index + 1}`} value={row.unitCost} oninput={(event) => updateRow(index, 'unitCost', event.currentTarget.value)} /></td>{/if}<td>{row.stock}{#if row.stockError}<small class="error">{row.stockError}</small>{/if}</td><td>{row.purchasePrice}</td><td><input aria-label={`Sale price ${index + 1}`} value={row.salePrice} oninput={(event) => updateRow(index, 'salePrice', event.currentTarget.value)} /></td><td>{row.manufacturer}</td><td>{row.pieces}</td><td>{row.location}</td><td><input aria-label={`Quantity ${index + 1}`} value={row.quantity} oninput={(event) => updateRow(index, 'quantity', event.currentTarget.value)} /></td><td>{row.total}</td><td><button type="button" aria-label={`Remove row ${index + 1}`} onclick={() => removeRow(index)}>×</button></td></tr>{/each}
      </tbody></table></div>
      <div class="legacy-sale-lines"><table><thead><tr><th>No.</th><th>Item Name</th></tr></thead><tbody>{#each rows as row, index}<tr><td>{index + 1}</td><td><input aria-label={`Item name summary ${index + 1}`} value={row.itemName} oninput={(event) => updateRow(index, 'itemName', event.currentTarget.value)} /></td></tr>{/each}</tbody></table></div>
	  {#if activeTab === 'list'}<div class="legacy-history-filter" role="search"><label>Filter:<input aria-label="Sales history filter" bind:value={historyFilter} onkeydown={(event) => { if (event.key === 'Enter') void loadHistory(); }} /></label><button type="button" onclick={() => void loadHistory()}>Filter / Retrieve</button></div><div class="legacy-sale-list"><table><thead><tr><th>Document</th><th>Date</th><th>Customer</th><th>Item</th><th>Qty</th><th>Total</th></tr></thead><tbody>
        {#if historyBusy}<tr><td colspan="6">Loading transaction history...</td></tr>
        {:else if history.length === 0}<tr><td colspan="6">No transactions found for this date.</td></tr>
        {:else}{#each history as row}<tr><td><button type="button" onclick={() => { void applyHistoryRow(row); }}>{row.document || ''}</button></td><td>{row.occurredAt || ''}</td><td>{row.party || ''}</td><td>{row.item || ''}</td><td>{row.quantity || ''}</td><td>{row.amount || ''}</td></tr>{/each}{/if}
      </tbody></table></div>{/if}
      <div class="legacy-sale-batch-panel" aria-label="Batch allocation">
        {#each rows as row, index}
          {#if row.availableBatches?.length && supportsExplicitBatchAllocation()}
            <fieldset class="sale-batch-allocation">
              <legend>Line {index + 1} batch allocation</legend>
              {#each rowAllocations(row) as allocation, allocationIndex}
                <div class="sale-batch-allocation-row">
                  <label>{allocationIndex === 0 ? 'Batch' : `Batch ${allocationIndex + 1}`}:
                    <select class="sale-batch-select" aria-label={allocationIndex === 0 ? `Batch ${index + 1}` : `Batch ${index + 1}-${allocationIndex + 1}`} value={allocation.batchId} onchange={(event) => setRowAllocation(index, allocationIndex, 'batchId', (event.currentTarget as HTMLSelectElement).value)}>
                      <option value="">Automatic FIFO</option>
                      {#each row.availableBatches as batch}
                        <option value={batch.batchId} disabled={rowAllocations(row).some((candidate, candidateIndex) => candidateIndex !== allocationIndex && candidate.batchId === batch.batchId)}>{batch.batchNumber}{batch.expiryDate ? ` · ${batch.expiryDate}` : ''} · {batch.quantity}</option>
                      {/each}
                    </select>
                  </label>
                  {#if allocation.batchId}
                    <label>Qty:
                      <input class="sale-batch-quantity" aria-label={`Batch quantity ${index + 1}-${allocationIndex + 1}`} type="number" min="0" step="any" value={allocation.quantity} oninput={(event) => setRowAllocation(index, allocationIndex, 'quantity', (event.currentTarget as HTMLInputElement).value)} />
                    </label>
                  {/if}
                  {#if rowAllocations(row).length > 1}
                    <button type="button" aria-label={`Remove batch allocation ${index + 1}-${allocationIndex + 1}`} onclick={() => removeRowAllocation(index, allocationIndex)}>Remove</button>
                  {/if}
                </div>
              {/each}
              <button type="button" aria-label={`Add batch allocation ${index + 1}`} onclick={() => addRowAllocation(index)}>Add batch</button>
              <small>Leave blank for Automatic FIFO. Explicit quantities must total the line quantity.</small>
            </fieldset>
          {/if}
        {/each}
      </div>
      <button class="legacy-add-row" type="button" onclick={addRow}>Add item row</button>
      <div class="legacy-purchase-adjustments legacy-sale-adjustments" aria-label="Sale adjustments">
        <label>Item GST %<input aria-label="Item GST percent" bind:value={itemGstRate} /></label>
        <label>Item Discount %<input aria-label="Item discount percent" bind:value={itemDiscountRate} /></label>
        <button type="button" onclick={() => attachmentInput?.click()}>Attach Document(s)</button>
        <button type="button" onclick={() => { message = attachments.length ? `Document Gallery: ${attachments.length} attachment${attachments.length === 1 ? '' : 's'} selected.` : 'Document Gallery: no attachments selected.'; }}>Document Gallery</button>
      </div>
      <input class="legacy-hidden-file-input" type="file" multiple bind:this={attachmentInput} onchange={onAttachmentsSelected} aria-label="Attach sale documents" />
      <div class="legacy-sale-total-line"><strong>Total:</strong><span>{rows.length}</span><span>{totalAmount}</span></div>
      <div class="legacy-sale-bottom">
        <label class="sale-bottom-disc-percent">Disc(%):<input aria-label="Discount percent" bind:value={documentDiscountPercent} oninput={queuePricingPreview} /></label>
        <label class="sale-bottom-flat-discount">Flat Disc.(-):<input aria-label="Flat discount" bind:value={flatDiscountAmount} oninput={queuePricingPreview} /></label>
        <label class="sale-bottom-misc">Misc(+):<input aria-label="Miscellaneous amount" bind:value={miscAmount} oninput={queuePricingPreview} /></label>
        <strong class="sale-bottom-sales">Sales: <em>{effectiveTotal}</em></strong>
        <label class="sale-bottom-total">Total:<input value={effectiveTotal} readonly aria-label="Sale total" /></label>
        {#if kind === 'cash'}<label class="sale-bottom-cash-tendered">Cash Tendered:<input aria-label="Cash tendered" inputmode="decimal" bind:value={cashTendered} /></label><label class="sale-bottom-cash-back">Cash Back:<input value={cashBack} readonly aria-label="Cash back" /></label>{/if}
        <label class="sale-bottom-stock">Stock:<input value={selectedStockTotal} readonly aria-label="Stock" /></label>
        <label class="sale-bottom-discount-value">Discount%:<input value={pricingPreview?.documentPercentDiscount ?? documentDiscountPercent} readonly aria-label="Discount value percent" /></label>
        <label class="sale-bottom-disc-value">Disc. Value:<input value={pricingPreview?.totalDiscount ?? '0.00'} readonly aria-label="Discount value" /></label>
      </div>
    </div>
    <div class="legacy-transaction-footer">{#if error}<span class="error" role="alert">{error}</span>{:else if pricingError}<span class="error" role="alert">{pricingError}</span>{:else if message}<span role="status">{message}</span>{:else}<span>{pricingBusy ? 'Calculating totals...' : 'Ready'}</span>{/if}<button type="button" class="legacy-sync-button" onclick={flushQueue} disabled={busy || pending === 0}>Sync queue ({pending})</button><a href="/app/legacy">Back to main window</a></div>
  </section>
</main>
