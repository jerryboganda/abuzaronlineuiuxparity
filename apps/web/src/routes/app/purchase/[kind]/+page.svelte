<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import type { Document, DocumentCommandForKind, InventoryAvailableBatch, ItemLookupResult, MasterRecord, PurchaseDocumentKind, SessionResponse, SyncEnvelope, ReportRow } from '@abuzar/contracts';
  import { AbuzarApi, ApiError, OfflineQueue, edgeRequest, newEventId } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';
  import type { MenuAction } from '$lib/legacy-menu';
  import { formatLegacyTitle } from '$lib/legacy-title';
  import { localDateAtNoonUtc, localDateString } from '$lib/calendar-date';

  type PurchaseAllocation = { batchId: string; batchNumber: string; quantity: string };
  type PurchaseRow = {
    quickSearch: string;
    itemId?: string;
    itemLegacyId: string;
    itemName: string;
    packUnits: string;
    packing: string;
    location: string;
    godown: string;
    batch: string;
    mfgDate: string;
    expiry: string;
    batchSalePrice: string;
    quantity: string;
    purchasePrice: string;
    discountPercent: string;
    gstRate: string;
    sourceBatchId: string;
    sourceLineId?: string;
    total: string;
  };

  const api = new AbuzarApi();
  const queue = new OfflineQueue();
  const supportedPurchaseRouteKinds = ['pack', 'loose', 'opening', 'return', 'order'];
  const titles: Record<string, string> = { pack: 'Pack Purchase', return: 'Purchase Return', opening: 'Opening Purchase', loose: 'Purchases (Loose)', order: 'Purchase Order' };
  let session: SessionResponse['context'] = null;
  let invoiceNumber = '';
  let supplier = '';
  let supplierInvoice = '';
  let orderCode = '';
  let sourceDocumentId = '';
  let sourceDocumentNumber = '';
  let transactionDate = localDateString();
  let remarks = '';
  let itemRecords: ItemLookupResult[] = [];
  let supplierRecords: MasterRecord[] = [];
  let godownRecords: MasterRecord[] = [];
  let supplierId = '';
  let godownId = '';
  let availableBatches: Record<number, InventoryAvailableBatch[]> = {};
  let returnAllocations: Record<number, PurchaseAllocation[]> = {};
  let businessDocumentId = '';
  let businessDocumentVersion = 0;
  let itemLookupBusy = false;
  let itemLookupGeneration = 0;
  let busy = false;
  let online = true;
  let pending = 0;
  let message = '';
  let error = '';
  let rows: PurchaseRow[] = [blankRow()];
  let activeTab: 'detail' | 'list' = 'detail';
  let interactive = false;
	let history: ReportRow[] = [];
	let historyBusy = false;
	let historyFilter = '';
  type HistoryMode = 'browse' | 'populate-invoice' | 'populate-return';
  let historyMode: HistoryMode = 'browse';
  let historyQueryKind = '';
  let saleTemplates: MasterRecord[] = [];
  let saleTemplateBusy = false;
  let showSaleTemplatePicker = false;
  let clock = new Date();
  let authenticatedUsername = 'ADMIN';
  let canonicalCommandSignature = '';
  let canonicalCommandId = '';
  let canonicalIdempotencyKey = '';
  let itemGstRate = '';
  let itemDiscountRate = '';
  let miscAmount = '0';
  let creditDays = '';
  let showExpenses = false;
  let attachmentInput: HTMLInputElement | null = null;
  let attachments: Array<{ name: string; size: number }> = [];

  $: kind = $page?.params?.kind ?? 'pack';
  $: title = titles[kind] ?? 'Purchase';
  $: historyKind = kind === 'return' ? 'purchase-return' : kind === 'order' ? 'purchase-order' : kind === 'loose' ? 'loose-purchase' : kind === 'opening' ? 'opening-purchase' : 'pack-purchase';
  $: historyRequestKind = historyQueryKind || historyKind;
  $: transactionWindowTitle = `${formatLegacyTitle(authenticatedUsername, clock)} - [${title}]`;
  function isCanonicalPurchaseKind(): boolean {
    return supportedPurchaseRouteKinds.includes(kind);
  }

  async function loadHistory(requestedKind = historyRequestKind) {
    historyBusy = true;
    try {
		history = (await api.transactions(requestedKind, transactionDate, transactionDate, historyFilter.trim())).rows;
    } catch {
      history = [];
    } finally {
      historyBusy = false;
    }
  }

  function purchaseRowsFromDocument(document: Document, useDocumentLineIdsAsSource = false): PurchaseRow[] {
    const hydrated = document.lines.map((line) => {
      const allocation = line.allocations?.[0];
      const batch = line.batchNumber || allocation?.batchNumber || '';
      return {
        ...blankRow(),
        quickSearch: line.itemName || line.itemCode || '',
        itemId: line.itemId,
        itemLegacyId: line.itemLegacyId || line.itemCode || line.itemId,
        itemName: line.itemName,
        batch,
        expiry: line.expiryDate || allocation?.expiryDate || '',
        quantity: line.quantity || '0',
        purchasePrice: line.unitCost || line.price.unitPrice || '0.00',
        discountPercent: line.price.discountPercent || '',
        gstRate: line.tax?.lines?.[0]?.rate || '',
        sourceBatchId: allocation?.batchId || '',
        sourceLineId: useDocumentLineIdsAsSource ? line.id : line.sourceLineId || '',
        total: line.lineTotal || line.price.netAmount || '0.00'
      };
    });
    returnAllocations = Object.fromEntries(document.lines.map((line, index) => [
      index,
      (line.allocations ?? []).map((allocation) => ({
        batchId: allocation.batchId ?? '',
        batchNumber: allocation.batchNumber ?? '',
        quantity: allocation.quantity || '0'
      }))
    ]).filter(([, allocations]) => (allocations as PurchaseAllocation[]).length > 0));
    return hydrated.length ? hydrated : [blankRow()];
  }

  async function applyHistoryRow(row: ReportRow) {
    historyBusy = true;
    error = '';
    try {
      if (!row.documentId) {
        invoiceNumber = row.document || '';
        supplier = row.party || supplier;
        availableBatches = {};
        returnAllocations = {};
        creditDays = '';
        rows = [{ ...blankRow(), itemName: row.item || '', quantity: row.quantity || '1', total: row.amount || '0.00' }];
        activeTab = 'detail';
        message = `${title} ${invoiceNumber || 'document'} loaded from the compatibility history summary.`;
        return;
      }
      const document = await api.document(row.documentId);
      const isPurchaseDocument = ['pack-purchase', 'loose-purchase', 'opening-purchase', 'purchase-return', 'purchase-order'].includes(document.kind);
      if (!isPurchaseDocument) throw new Error('The selected history row is not a canonical purchase document.');
      const populating = historyMode !== 'browse';
      const sourceDocument = historyMode === 'populate-invoice' || historyMode === 'populate-return' ? document : undefined;
      invoiceNumber = populating ? '' : document.documentNumber || row.document || '';
      transactionDate = document.occurredAt?.slice(0, 10) || transactionDate;
      supplier = document.supplier?.name || row.party || supplier;
      supplierId = document.supplierId || supplierId;
      godownId = document.godownId || godownId;
      supplierInvoice = document.reference || '';
      remarks = document.remarks || '';
      sourceDocumentId = sourceDocument?.id || document.sourceDocumentId || '';
      sourceDocumentNumber = sourceDocument?.documentNumber || document.sourceDocumentNumber || '';
      creditDays = sourceDocument?.creditDays ?? document.creditDays ?? '';
      rows = purchaseRowsFromDocument(document, historyMode === 'populate-return');
      if (historyMode === 'populate-return') await prepareReturnSourceBatches(document);
      businessDocumentId = populating ? '' : document.id;
      businessDocumentVersion = populating ? 0 : document.version;
      canonicalCommandSignature = '';
      canonicalCommandId = '';
      canonicalIdempotencyKey = '';
      activeTab = 'detail';
      message = populating
        ? `${title}: ${document.documentNumber || row.document || 'document'} lines populated from canonical history.`
        : `${title} ${document.documentNumber || row.document || 'document'} loaded with canonical lines.`;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'The selected purchase document could not be loaded.';
      message = 'The selected purchase document could not be loaded.';
    } finally {
      historyBusy = false;
    }
  }

  async function navigateHistory(offset: number) {
    if (!history.length) await loadHistory();
    if (!history.length) { message = 'No persisted documents found for this date.'; return; }
    const current = history.findIndex((row) => row.document === invoiceNumber);
    const next = current < 0 ? (offset > 0 ? 0 : history.length - 1) : (current + offset + history.length) % history.length;
    void applyHistoryRow(history[next]);
  }

  async function navigateHistoryTo(index: number) {
    if (!history.length) await loadHistory();
    if (!history.length) { message = 'No persisted documents found for this date.'; return; }
    void applyHistoryRow(history[index < 0 ? history.length - 1 : Math.min(index, history.length - 1)]);
  }

  function autoGenerateBatches() {
    error = '';
    const dateToken = transactionDate.replace(/[^0-9]/g, '').slice(0, 8) || localDateString().replace(/-/g, '');
    let sequence = 0;
    rows = rows.map((row) => {
      if (!row.itemName.trim() && !row.quickSearch.trim()) return row;
      sequence += 1;
      return row.batch.trim() ? row : { ...row, batch: `AUTO-${dateToken}-${String(sequence).padStart(3, '0')}` };
    });
    message = sequence
      ? `Auto Batch Generation: ${sequence} batch identifier${sequence === 1 ? '' : 's'} generated.`
      : 'Select at least one item before generating batch identifiers.';
  }

  async function printPurchaseLabels() {
    const labels = rows.filter((row) => row.itemName.trim()).map((row) => ({
      itemName: row.itemName,
      batch: row.batch || '',
      expiry: row.expiry || '',
      mrp: row.batchSalePrice || '',
      quantity: row.quantity || '0'
    }));
    if (!labels.length) {
      message = 'Print Purchase Labels: enter at least one item first.';
      return;
    }
    try {
      await edgeRequest('/v1/hardware/print/purchase-labels', { labels, cutAfter: true });
      message = `Print Purchase Labels: ${labels.length} label${labels.length === 1 ? '' : 's'} sent to the branch printer.`;
    } catch {
      message = `Print Purchase Labels: preview ready for ${labels.length} item${labels.length === 1 ? '' : 's'}.`;
      window.print();
    }
  }

  function onAttachmentsSelected(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    attachments = Array.from(input.files ?? []).map((file) => ({ name: file.name, size: file.size }));
    message = attachments.length ? `${attachments.length} document${attachments.length === 1 ? '' : 's'} attached to this draft.` : 'No documents selected.';
  }

  $: if (activeTab === 'list' && historyRequestKind && transactionDate) void loadHistory();

  function openHistory(mode: HistoryMode = 'browse', requestedKind = historyKind) {
    historyMode = mode;
    historyQueryKind = requestedKind === historyKind ? '' : requestedKind;
    activeTab = 'list';
    void loadHistory(requestedKind);
  }

  function itemHistoryFilter(): string {
    const row = rows.find((candidate) => candidate.itemLegacyId.trim() || candidate.itemName.trim() || candidate.quickSearch.trim());
    return row?.itemLegacyId.trim() || row?.itemName.trim() || row?.quickSearch.trim() || '';
  }

  function enableInteractive(event?: Event) {
    const target = event?.target;
    if (target instanceof Element && target.closest('.legacy-transaction-tabs, .legacy-menu-bar, .legacy-mdi-tabs')) return;
    interactive = true;
  }

  function handleMenuCommand(action: MenuAction): boolean {
    switch (action.label) {
      case 'New':
        newDocument();
        return true;
      case 'List':
        openHistory();
        return true;
      case 'Save':
        void savePurchase('save');
        return true;
      case 'Post':
        void savePurchase(businessDocumentId ? 'post' : 'save-and-post');
        return true;
      case 'Save And Post':
        void savePurchase('save-and-post');
        return true;
      case 'Print':
      case 'Purchase Slip':
        message = `${action.label}: print preview ready.`;
        window.print();
        return true;
      case 'Print Purchase Labels':
        void printPurchaseLabels();
        return true;
      case 'Populate Items':
        void populatePurchaseItems();
        return true;
      case 'Populate Purchase Invoice':
        openHistory('populate-invoice', 'purchase-order');
        message = 'Populate Purchase Invoice: select a purchase order to copy its canonical lines.';
        return true;
      case 'Populate Purchase Return Invoice':
        openHistory('populate-return', 'pack-purchase');
        message = 'Populate Purchase Return Invoice: select the posted purchase whose batches should be returned.';
        return true;
      case 'Populate Sales Order':
      case 'Populate Pending Due Item(s)':
        openHistory('browse');
        message = `${action.label}: persisted purchase history is ready for selection.`;
        return true;
      case 'Populate From Sale Template':
        void openSaleTemplatePicker();
        return true;
      case 'Fetch Purchase Invoice From Other Sources':
        openHistory('populate-invoice', 'purchase-order');
        message = 'Fetch Purchase Invoice From Other Sources: select a canonical purchase order to populate a new invoice.';
        return true;
      case 'Import Parent Server Selected Transactions':
        void flushQueue();
        message = 'Import Parent Server Selected Transactions: branch sync requested.';
        return true;
      case 'Show Purchase Expenses Window':
        showExpenses = true;
        return true;
      case 'Attach Document(s)':
        attachmentInput?.click();
        return true;
      case 'Show Document Gallery':
        message = attachments.length ? `Document Gallery: ${attachments.length} attachment${attachments.length === 1 ? '' : 's'} selected.` : 'Document Gallery: no attachments selected.';
        return true;
      case 'Apply Item GST %':
        if (!itemGstRate.trim()) {
          message = 'Apply Item GST %: enter a rate in the transaction adjustments first.';
          return true;
        }
        let gstApplied = 0;
        rows = rows.map((row) => {
          if (!row.itemName.trim() && !row.quickSearch.trim()) return row;
          gstApplied += 1;
          return { ...row, gstRate: itemGstRate.trim() };
        });
        message = gstApplied ? `Apply Item GST %: ${itemGstRate.trim()}% applied to populated lines.` : 'Apply Item GST %: no populated lines to update.';
        return true;
      case 'Apply Item Discount %':
        if (!itemDiscountRate.trim()) {
          message = 'Apply Item Discount %: enter a rate in the transaction adjustments first.';
          return true;
        }
        let discountApplied = 0;
        rows = rows.map((row) => {
          if (!row.itemName.trim() && !row.quickSearch.trim()) return row;
          discountApplied += 1;
          return { ...row, discountPercent: itemDiscountRate.trim() };
        });
        message = discountApplied ? `Apply Item Discount %: ${itemDiscountRate.trim()}% applied to populated lines.` : 'Apply Item Discount %: no populated lines to update.';
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
      case 'Auto Batch Generation':
        autoGenerateBatches();
        return true;
      case 'Item Purchase History':
        {
          const itemFilter = itemHistoryFilter();
          if (!itemFilter) {
            message = 'Item Purchase History: select or populate an item row first.';
            return true;
          }
          historyFilter = itemFilter;
        }
        openHistory();
        message = `Item Purchase History: filtered transaction list ready for ${historyFilter}.`;
        return true;
      case 'Sort Items':
        {
          const sorted = rows.map((row, index) => ({ row, index })).sort((left, right) => left.row.itemName.localeCompare(right.row.itemName));
          rows = sorted.map((entry) => entry.row);
          returnAllocations = Object.fromEntries(sorted
            .map((entry, index) => [index, returnAllocations[entry.index]])
            .filter(([, allocations]) => Boolean(allocations)) as Array<[number, PurchaseAllocation[]]>);
          availableBatches = Object.fromEntries(sorted
            .map((entry, index) => [index, availableBatches[entry.index]])
            .filter(([, batches]) => Boolean(batches)) as Array<[number, InventoryAvailableBatch[]]>);
        }
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
        {
          const item = rows.find((candidate) => candidate.itemId && candidate.itemLegacyId.trim());
          if (!item) {
            message = 'View Item Info: select an active canonical item row first.';
            return true;
          }
          window.location.assign(`/app/master/item?legacyId=${encodeURIComponent(item.itemLegacyId.trim())}`);
        }
        return true;
      case 'Supplier Info.':
        {
          const supplierRecord = supplierRecords.find((record) => record.id === supplierId && record.active)
            ?? supplierRecords.find((record) => record.name.trim().toLowerCase() === supplier.trim().toLowerCase() || record.code.trim().toLowerCase() === supplier.trim().toLowerCase() || record.legacyId?.trim().toLowerCase() === supplier.trim().toLowerCase());
          const supplierLegacyId = supplierRecord?.legacyId?.trim() || supplierRecord?.code?.trim();
          if (!supplierRecord || !supplierLegacyId) {
            message = 'Supplier Info.: select an active canonical supplier first.';
            return true;
          }
          window.location.assign(`/app/master/supplier?legacyId=${encodeURIComponent(supplierLegacyId)}`);
        }
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
      case 'Void':
        void voidPurchase();
        return true;
      default:
        return false;
    }
  }

  function blankRow(): PurchaseRow {
    return { quickSearch: '', itemLegacyId: '', itemName: '', packUnits: '', packing: '', location: '', godown: '', batch: '', mfgDate: '', expiry: '', batchSalePrice: '', quantity: '1', purchasePrice: '', discountPercent: '', gstRate: '', sourceBatchId: '', total: '0.00' };
  }

  async function lookupItems(value: string): Promise<ItemLookupResult[]> {
    const query = value.trim();
    const generation = ++itemLookupGeneration;
    if (!query) {
      itemRecords = [];
      return [];
    }
    itemLookupBusy = true;
    try {
      const records = (await api.itemLookup(query)).items.filter((record) => record.active && Boolean(record.id));
      if (generation !== itemLookupGeneration) return [];
      itemRecords = records;
      error = '';
      return records;
    } catch (cause) {
      if (generation !== itemLookupGeneration) return [];
      itemRecords = [];
      if (!(cause instanceof ApiError && cause.status === 401)) error = cause instanceof Error ? cause.message : 'Canonical item lookup could not be loaded.';
      return [];
    } finally {
      if (generation === itemLookupGeneration) itemLookupBusy = false;
    }
  }

  async function loadCanonicalContext() {
    const [supplierResult, godownResult] = await Promise.allSettled([api.masterRecords('supplier'), api.masterRecords('godown')]);
    if (supplierResult.status === 'fulfilled') {
      supplierRecords = supplierResult.value.records.filter((record) => record.active);
    } else if (!(supplierResult.reason instanceof ApiError && supplierResult.reason.status === 401)) {
      error = supplierResult.reason instanceof Error ? supplierResult.reason.message : 'Canonical supplier context could not be loaded.';
    }
    if (godownResult.status === 'fulfilled') {
      godownRecords = godownResult.value.records.filter((record) => record.active);
    } else if (!(godownResult.reason instanceof ApiError && godownResult.reason.status === 401)) {
      error = godownResult.reason instanceof Error ? godownResult.reason.message : 'Canonical godown context could not be loaded.';
    }
    const normalizedSupplier = supplier.trim().toLowerCase();
    const supplierMatch = supplierRecords.find((record) => record.code.toLowerCase() === normalizedSupplier || record.name.toLowerCase() === normalizedSupplier || record.legacyId?.toLowerCase() === normalizedSupplier);
    supplierId = supplierMatch?.id ?? '';
    const selectedGodown = rows.find((row) => row.godown.trim())?.godown.trim().toLowerCase() ?? '';
    const godownMatch = godownRecords.find((record) => record.code.toLowerCase() === selectedGodown || record.name.toLowerCase() === selectedGodown || record.legacyId?.toLowerCase() === selectedGodown);
    godownId = godownMatch?.id ?? '';
  }

  const packHeaders = ['No.', 'Quick Search', 'Alias Name', 'Alternate Alias Name', 'Item Name', 'Pack Units', 'Packing', 'Item Location', 'Godown', 'Batch', 'Mfg. Date', 'Expiry', 'Batch Sale Price', 'Lock Batch', 'Lock Reason', 'Description', 'Total Pieces', 'Weight/Unit', 'Total Weight', 'Pack Capacity', 'Area/Volume', ''];

  function addRow() {
    rows = [...rows, blankRow()];
  }

  function updateRow(index: number, key: keyof PurchaseRow, value: string) {
    rows = rows.map((row, rowIndex) => {
      if (rowIndex !== index) return row;
      const next = { ...row, [key]: value };
      if ((key === 'itemName' || key === 'quickSearch') && value !== row[key]) { next.itemId = undefined; next.itemLegacyId = ''; next.sourceBatchId = ''; next.sourceLineId = ''; returnAllocations = { ...returnAllocations, [index]: [] }; }
      if (key === 'quantity' || key === 'purchasePrice') next.total = ((Number(next.quantity) || 0) * (Number(next.purchasePrice) || 0)).toFixed(2);
      return next;
    });
  }

  function returnAllocationsFor(index: number, row = rows[index]): PurchaseAllocation[] {
    const explicit = returnAllocations[index];
    if (explicit?.length) return explicit;
    return [{ batchId: row?.sourceBatchId ?? '', batchNumber: row?.batch ?? '', quantity: row?.quantity || '1' }];
  }

  function setReturnAllocation(index: number, allocationIndex: number, key: keyof PurchaseAllocation, value: string) {
    const row = rows[index];
    if (!row) return;
    const allocations = [...returnAllocationsFor(index, row)];
    const selectedBatch = key === 'batchId' ? (availableBatches[index] ?? []).find((batch) => batch.batchId === value) : undefined;
    allocations[allocationIndex] = {
      ...(allocations[allocationIndex] ?? { batchId: '', batchNumber: '', quantity: '0' }),
      [key]: value,
      ...(selectedBatch ? { batchNumber: selectedBatch.batchNumber } : {})
    };
    returnAllocations = { ...returnAllocations, [index]: allocations };
    if (allocationIndex === 0 && key === 'batchId') updateRow(index, 'sourceBatchId', value);
    if (allocationIndex === 0 && key === 'batchNumber') updateBatch(index, value);
    if (key === 'batchId') message = value ? `Source batch selected for return line ${index + 1}. Enter the quantity for this allocation.` : `Automatic source batch selection restored for return line ${index + 1}.`;
  }

  function addReturnAllocation(index: number) {
    const row = rows[index];
    if (!row) return;
    returnAllocations = { ...returnAllocations, [index]: [...returnAllocationsFor(index, row), { batchId: '', batchNumber: '', quantity: '0' }] };
  }

  function removeReturnAllocation(index: number, allocationIndex: number) {
    const row = rows[index];
    if (!row) return;
    const allocations = returnAllocationsFor(index, row).filter((_, candidateIndex) => candidateIndex !== allocationIndex);
    const nextAllocations = allocations.length ? allocations : [{ batchId: '', batchNumber: '', quantity: row.quantity || '1' }];
    returnAllocations = { ...returnAllocations, [index]: nextAllocations };
  }

  async function chooseItem(index: number, value: string): Promise<boolean> {
    const records = await lookupItems(value);
    const normalized = value.trim().toLowerCase();
    const match = records.find((record) => record.code.toLowerCase() === normalized
      || record.name.toLowerCase() === normalized
      || record.legacyId.toLowerCase() === normalized
      || record.aliases.some((alias) => alias.toLowerCase() === normalized))
      ?? (records.length === 1 ? records[0] : undefined);
    updateRow(index, 'quickSearch', value);
    if (match) {
      updateRow(index, 'itemName', match.name);
      updateRow(index, 'itemLegacyId', match.legacyId || match.id);
      rows = rows.map((row, rowIndex) => rowIndex === index ? { ...row, itemId: match.id } : row);
      void refreshRowBatches(index);
    }
    return Boolean(match);
  }

  async function populatePurchaseItems() {
    activeTab = 'detail';
    error = '';
    const searchRows = rows.map((row, index) => ({
      index,
      value: row.quickSearch.trim() || (!row.itemId ? row.itemName.trim() : '')
    })).filter((candidate) => candidate.value);
    let attempted = searchRows.length;
    let populated = 0;
    let unresolved = 0;
    for (const candidate of searchRows) {
      if (await chooseItem(candidate.index, candidate.value)) populated += 1;
      else unresolved += 1;
    }
    if (!attempted) {
      const blankIndex = rows.findIndex((row) => !row.itemId && !row.itemName.trim() && !row.quickSearch.trim());
      const fallback = itemRecords[0];
      if (blankIndex >= 0 && fallback) {
        attempted = 1;
        if (await chooseItem(blankIndex, fallback.code || fallback.name)) populated = 1;
        else unresolved = 1;
      }
    }
    if (populated) {
      message = `Populate Items: ${populated} active canonical item${populated === 1 ? '' : 's'} populated${unresolved ? `; ${unresolved} search${unresolved === 1 ? '' : 'es'} still require an exact match.` : '.'}`;
    } else if (attempted) {
      message = 'Populate Items: no exact or unique active canonical item matched the entered search values.';
    } else {
      message = 'Populate Items: enter a quick-search value or run an item lookup before populating.';
    }
  }

  function templateText(payload: Record<string, unknown>, ...keys: string[]): string {
    for (const key of keys) {
      const value = payload[key];
      if (value !== undefined && value !== null && String(value).trim()) return String(value).trim();
    }
    return '';
  }

  function purchaseRowsFromSaleTemplate(template: MasterRecord): PurchaseRow[] {
    const payload = template.payload ?? {};
    const candidates = [payload.rows, payload.lines, payload.items].find((value) => Array.isArray(value));
    if (!Array.isArray(candidates)) return [];
    return candidates.flatMap((candidate) => {
      if (!candidate || typeof candidate !== 'object') return [];
      const line = candidate as Record<string, unknown>;
      const itemIdCandidate = templateText(line, 'itemId', 'ItemId');
      const itemId = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(itemIdCandidate) ? itemIdCandidate : '';
      const itemName = templateText(line, 'itemName', 'itemDescription', 'item', 'ItemName');
      const quickSearch = templateText(line, 'quickSearch', 'itemCode', 'itemLegacyId', 'alias', 'code') || itemName;
      const quantity = templateText(line, 'quantity', 'Qty') || '1';
      const purchasePrice = templateText(line, 'purchasePrice', 'unitCost', 'price', 'PurchasePrice');
      const total = templateText(line, 'total', 'lineTotal', 'amount') || ((Number(quantity) || 0) * (Number(purchasePrice) || 0)).toFixed(2);
      return [{
        ...blankRow(),
        ...(itemId ? { itemId } : {}),
        quickSearch,
        itemLegacyId: templateText(line, 'itemLegacyId', 'legacyId', 'itemCode'),
        itemName,
        packUnits: templateText(line, 'packUnits', 'PiecesInPacking'),
        packing: templateText(line, 'packing'),
        location: templateText(line, 'location', 'Location'),
        godown: templateText(line, 'godown', 'godownName'),
        batch: templateText(line, 'batch', 'batchNumber'),
        mfgDate: templateText(line, 'mfgDate', 'manufactureDate'),
        expiry: templateText(line, 'expiry', 'expiryDate'),
        batchSalePrice: templateText(line, 'batchSalePrice', 'salePrice'),
        quantity,
        purchasePrice,
        discountPercent: templateText(line, 'discountPercent', 'discPerc'),
        gstRate: templateText(line, 'gstRate', 'salesTax', 'taxRate'),
        total
      }];
    });
  }

  async function openSaleTemplatePicker() {
    saleTemplateBusy = true;
    showSaleTemplatePicker = true;
    activeTab = 'detail';
    error = '';
    try {
      saleTemplates = (await api.masterRecords('sale-template')).records.filter((record) => record.active);
      message = saleTemplates.length ? 'Populate From Sale Template: select an active template.' : 'Populate From Sale Template: no active sale templates are available.';
    } catch (cause) {
      saleTemplates = [];
      error = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Sale templates could not be loaded.';
      message = 'Populate From Sale Template could not load the template list.';
    } finally {
      saleTemplateBusy = false;
    }
  }

  async function applySaleTemplate(template: MasterRecord) {
    const templateRows = purchaseRowsFromSaleTemplate(template);
    if (!templateRows.length) {
      message = `${template.name || template.code}: no supported line payload was found; the template master remains unchanged.`;
      return;
    }
    availableBatches = {};
    returnAllocations = {};
    rows = templateRows;
    invoiceNumber = '';
    businessDocumentId = '';
    businessDocumentVersion = 0;
    creditDays = '';
    canonicalCommandSignature = '';
    canonicalCommandId = '';
    canonicalIdempotencyKey = '';
    orderCode = template.code;
    showSaleTemplatePicker = false;
    activeTab = 'detail';
    await populatePurchaseItems();
    message = `Populate From Sale Template: ${template.name || template.code} loaded ${templateRows.length} line${templateRows.length === 1 ? '' : 's'} into a new draft.`;
  }

  function chooseSupplier(value: string) {
    supplier = value;
    const normalized = value.trim().toLowerCase();
    const match = supplierRecords.find((record) => record.code.toLowerCase() === normalized || record.name.toLowerCase() === normalized || record.legacyId?.toLowerCase() === normalized);
    supplierId = match?.id ?? '';
  }

  function chooseGodown(index: number, value: string) {
    updateRow(index, 'godown', value);
    const normalized = value.trim().toLowerCase();
    const match = godownRecords.find((record) => record.code.toLowerCase() === normalized || record.name.toLowerCase() === normalized || record.legacyId?.toLowerCase() === normalized);
    godownId = match?.id ?? '';
    if (match) void refreshRowBatches(index);
  }

  async function refreshRowBatches(index: number) {
    const row = rows[index];
    if (!row?.itemLegacyId || !godownId || !isCanonicalPurchaseKind()) return;
    try {
      const result = await api.inventoryAvailability(row.itemLegacyId, godownId);
      availableBatches = { ...availableBatches, [index]: result.batches };
    } catch {
      availableBatches = { ...availableBatches, [index]: [] };
    }
  }

  async function prepareReturnSourceBatches(document: Document) {
    if (document.godownId) godownId = document.godownId;
    const godown = godownRecords.find((record) => record.id === godownId);
    rows = rows.map((row) => ({ ...row, godown: row.godown || godown?.name || godown?.code || godownId }));
    availableBatches = {};
    await Promise.all(rows.map((_, index) => refreshRowBatches(index)));
    const nextAllocations = { ...returnAllocations };
    rows = rows.map((row, index) => {
      if (nextAllocations[index]?.length || !row.batch.trim()) return row;
      const match = (availableBatches[index] ?? []).filter((batch) => batch.batchNumber.toLowerCase() === row.batch.trim().toLowerCase());
      if (match.length !== 1) return row;
      nextAllocations[index] = [{ batchId: match[0].batchId, batchNumber: match[0].batchNumber, quantity: row.quantity || '1' }];
      return { ...row, sourceBatchId: match[0].batchId };
    });
    returnAllocations = nextAllocations;
  }

  function updateBatch(index: number, value: string) {
    updateRow(index, 'batch', value);
    if (kind !== 'return') return;
    const match = (availableBatches[index] ?? []).find((batch) => batch.batchNumber.toLowerCase() === value.trim().toLowerCase());
    rows = rows.map((row, rowIndex) => rowIndex === index ? { ...row, sourceBatchId: match?.batchId ?? row.sourceBatchId } : row);
    const allocations = [...returnAllocationsFor(index, rows[index])];
    allocations[0] = { ...allocations[0], batchId: match?.batchId ?? allocations[0].batchId, batchNumber: value };
    returnAllocations = { ...returnAllocations, [index]: allocations };
  }

  function updateSourceBatchId(index: number, value: string) {
    updateRow(index, 'sourceBatchId', value);
    const row = rows[index];
    if (!row) return;
    const allocations = [...returnAllocationsFor(index, row)];
    allocations[0] = { ...allocations[0], batchId: value };
    returnAllocations = { ...returnAllocations, [index]: allocations };
  }

  function removeRow(index: number) {
    rows = rows.filter((_, rowIndex) => rowIndex !== index);
    if (!rows.length) rows = [blankRow()];
    returnAllocations = Object.fromEntries(Object.entries(returnAllocations)
      .filter(([key]) => Number(key) !== index)
      .map(([key, value]) => [Number(key) > index ? Number(key) - 1 : Number(key), value]));
    availableBatches = Object.fromEntries(Object.entries(availableBatches)
      .filter(([key]) => Number(key) !== index)
      .map(([key, value]) => [Number(key) > index ? Number(key) - 1 : Number(key), value]));
  }

  $: grandTotal = rows.reduce((sum, row) => sum + (Number(row.total) || 0), 0).toFixed(2);

  function makeEvent(): SyncEnvelope {
    if (!session?.tenantId || !session.branchId || !session.counterId || !session.operatorId) throw new Error('Select a branch and counter before saving a purchase.');
    const eventId = newEventId();
    const aggregate = kind === 'return' ? 'return' : kind === 'order' ? 'purchase_order' : 'receiving';
    return {
      eventId,
      aggregate,
      aggregateId: eventId,
      tenantId: session.tenantId,
      branchId: session.branchId,
      counterId: session.counterId,
      operatorId: session.operatorId,
      occurredAt: localDateAtNoonUtc(transactionDate),
      idempotencyKey: `${kind}:${invoiceNumber || eventId}`,
      schemaVersion: 1,
      payload: { kind, invoiceNumber, supplier, supplierInvoice, orderCode, remarks, creditDays, rows, status: 'posted' }
    };
  }

  function purchaseDocumentKind(): 'pack-purchase' | 'loose-purchase' | 'opening-purchase' | 'purchase-return' | 'purchase-order' {
    if (kind === 'return') return 'purchase-return';
    if (kind === 'order') return 'purchase-order';
    if (kind === 'loose') return 'loose-purchase';
    if (kind === 'opening') return 'opening-purchase';
    return 'pack-purchase';
  }

  function canonicalPurchaseValidation(action: 'save' | 'post' | 'save-and-post'): PurchaseRow[] {
    const requestedKind = purchaseDocumentKind();
    const lineRows = rows.filter((row) => row.itemName.trim());
    const isUuid = (value: string) => /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value.trim());
    if (!lineRows.length) throw new Error('Enter at least one item selected from the active canonical item lookup.');
    if (!supplierId || !supplierRecords.some((record) => record.id === supplierId && record.active)) throw new Error('Select an active canonical supplier before saving or posting.');
    if (requestedKind !== 'purchase-order' && (!godownId || !godownRecords.some((record) => record.id === godownId && record.active))) throw new Error('Select an active canonical godown before saving or posting the purchase.');
    if (lineRows.some((row) => !row.itemId || !isUuid(row.itemId) || !row.itemLegacyId)) throw new Error('Select every purchase item from the active canonical item list.');
    if (requestedKind === 'purchase-return') {
      if (!isUuid(sourceDocumentId)) throw new Error('Enter the canonical source purchase document UUID before saving or posting a return.');
      lineRows.forEach((row, lineIndex) => {
        const sourceIndex = rows.indexOf(row);
        if ((action === 'post' || action === 'save-and-post') && !isUuid(row.sourceLineId ?? '')) throw new Error(`Select the canonical source purchase line for return line ${lineIndex + 1}.`);
        const allocations = returnAllocationsFor(sourceIndex, row).map((allocation) => ({
          batchId: allocation.batchId.trim(),
          batchNumber: allocation.batchNumber.trim(),
          quantity: allocation.quantity.trim()
        }));
        if (allocations.some((allocation) => !allocation.batchNumber || !isUuid(allocation.batchId))) throw new Error(`Select an explicit canonical source batch and batch ID for every return allocation on line ${lineIndex + 1}.`);
        if (allocations.some((allocation) => !/^\d+(?:\.\d{1,4})?$/.test(allocation.quantity) || Number(allocation.quantity) <= 0)) throw new Error(`Enter a positive source allocation quantity with no more than four decimals for return line ${lineIndex + 1}.`);
        const batchIds = allocations.map((allocation) => allocation.batchId);
        if (new Set(batchIds).size !== batchIds.length) throw new Error(`Select each source batch only once for return line ${lineIndex + 1}.`);
        const allocated = allocations.reduce((sum, allocation) => sum + Number(allocation.quantity), 0);
        if (Math.abs(allocated - Number(row.quantity)) > 0.00000001) throw new Error(`Source batch allocations for return line ${lineIndex + 1} must total the line quantity (${row.quantity || '0'}).`);
      });
    } else if (requestedKind !== 'purchase-order') {
      if (lineRows.some((row) => !row.batch.trim() || !row.expiry.trim() || !row.purchasePrice.trim())) throw new Error('Batch, expiry, and unit cost are required for every purchase receipt.');
    }
    if (lineRows.some((row) => !row.quantity.trim() || Number(row.quantity) <= 0)) throw new Error('Every canonical purchase line requires a positive quantity.');
    if (lineRows.some((row) => Number(row.purchasePrice || '0') < 0)) throw new Error('Purchase unit cost cannot be negative.');
    if (['pack-purchase', 'loose-purchase', 'opening-purchase'].includes(requestedKind) && creditDays.trim() && (!/^-?\d{1,5}$/.test(creditDays.trim()) || Math.abs(Number(creditDays)) > 36500)) throw new Error('Credit days must be a whole number between -36500 and 36500.');
    void action;
    return lineRows;
  }

  function canonicalPurchaseCommand(action: 'save' | 'post' | 'save-and-post'): DocumentCommandForKind<PurchaseDocumentKind> {
    const documentKind = purchaseDocumentKind();
    const lineRows = canonicalPurchaseValidation(action);
    const occurredAt = localDateAtNoonUtc(transactionDate);
    const lines = lineRows.map((row, index) => ({
      lineNumber: index + 1,
      itemId: row.itemId as string,
      quantity: row.quantity || '0',
      unitPrice: row.purchasePrice || '0',
      unitCost: documentKind === 'purchase-order' ? (row.purchasePrice || '0') : row.purchasePrice || '0',
      ...(row.discountPercent.trim() ? { discountPercent: row.discountPercent.trim() } : {}),
      ...(row.gstRate.trim() ? { gstRate: row.gstRate.trim() } : {}),
      ...(documentKind === 'purchase-return' ? {
        ...(row.sourceLineId?.trim() ? { sourceLineId: row.sourceLineId.trim() } : {}),
        allocations: returnAllocationsFor(rows.indexOf(row), row).map((allocation) => ({
          batchId: allocation.batchId.trim(),
          batchNumber: allocation.batchNumber.trim(),
          quantity: allocation.quantity.trim()
        }))
      } : {}),
      ...(documentKind === 'purchase-order' ? {} : { batchNumber: row.batch, expiryDate: row.expiry })
    }));
    const signature = JSON.stringify({ documentKind, action, documentId: businessDocumentId, version: businessDocumentVersion, supplierId, godownId, sourceDocumentId, sourceDocumentNumber, creditDays, lines, transactionDate });
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
      occurredAt,
      document: {
        ...(businessDocumentId ? { id: businessDocumentId } : {}),
        kind: documentKind,
        documentNumber: invoiceNumber,
        occurredAt,
        supplierId,
        ...(documentKind !== 'purchase-order' ? { godownId } : {}),
        ...(sourceDocumentId.trim() ? { sourceDocumentId: sourceDocumentId.trim(), sourceDocumentNumber: sourceDocumentNumber.trim() } : {}),
        reference: supplierInvoice,
        remarks,
        ...(['pack-purchase', 'loose-purchase', 'opening-purchase'].includes(documentKind) && creditDays.trim() ? { creditDays: creditDays.trim() } : {}),
        lines,
        priceLevel: 1,
        flatDiscountAmount: '0',
        miscAmount: miscAmount || '0',
        documentDiscountPercent: '0',
        pricing: { priceLevel: 1, flatDiscountAmount: '0', miscAmount: miscAmount || '0', documentDiscountPercent: '0' }
      }
    };
    return businessDocumentVersion > 0 ? { ...base, expectedVersion: businessDocumentVersion } as DocumentCommandForKind<PurchaseDocumentKind> : base as DocumentCommandForKind<PurchaseDocumentKind>;
  }

  async function submitCanonicalPurchase(action: 'save' | 'post' | 'save-and-post') {
    const documentKind = purchaseDocumentKind();
    const response = await api.documentCommand(documentKind, canonicalPurchaseCommand(action));
    if (!response.accepted) throw new Error(response.errors.map((item) => item.message).join('; ') || 'The canonical purchase command was rejected.');
    if (action === 'save' && response.status !== 'draft') throw new Error(`Canonical save returned status ${response.status}; it was not saved as a draft.`);
    if ((action === 'post' || action === 'save-and-post') && response.status !== 'posted') throw new Error(`Canonical purchase was accepted with status ${response.status}; it was not posted.`);
    if (documentKind === 'purchase-order' && (response.document.stock?.direction && response.document.stock.direction !== 'none' || response.document.gl?.postings?.length)) {
      throw new Error('Purchase order response is not stock/GL-neutral.');
    }
    businessDocumentId = response.document.id;
    businessDocumentVersion = response.document.version;
    invoiceNumber = response.document.documentNumber ?? invoiceNumber;
    canonicalCommandSignature = '';
    canonicalCommandId = '';
    canonicalIdempotencyKey = '';
    const neutrality = documentKind === 'purchase-order' ? ' (stock/GL-neutral)' : '';
    message = `${title} ${action === 'save' ? 'saved as draft' : 'posted'} successfully.${neutrality}`;
  }

  async function voidPurchase() {
    if (!isCanonicalPurchaseKind() || !businessDocumentId) throw new Error('Load or save a canonical purchase before voiding it.');
    const signature = JSON.stringify({ documentKind: purchaseDocumentKind(), action: 'void', documentId: businessDocumentId, version: businessDocumentVersion });
    if (signature !== canonicalCommandSignature) {
      canonicalCommandSignature = signature;
      canonicalCommandId = newEventId();
      canonicalIdempotencyKey = `canonical:${purchaseDocumentKind()}:void:${businessDocumentId}:${businessDocumentVersion}`;
    }
    const command: DocumentCommandForKind<PurchaseDocumentKind> = {
      commandId: canonicalCommandId,
      kind: purchaseDocumentKind(),
      action: 'void',
      idempotencyKey: canonicalIdempotencyKey,
      occurredAt: localDateAtNoonUtc(transactionDate),
      expectedVersion: businessDocumentVersion,
      documentId: businessDocumentId,
      reason: 'Voided from purchase workflow'
    } as DocumentCommandForKind<PurchaseDocumentKind>;
    busy = true;
    message = '';
    error = '';
    try {
      const response = await api.documentCommand(purchaseDocumentKind(), command);
      if (!response.accepted) throw new Error(response.errors.map((item) => item.message).join('; ') || 'The canonical void command was rejected.');
      if (response.status !== 'void') throw new Error(`Canonical void returned status ${response.status}.`);
      businessDocumentVersion = response.document.version;
      canonicalCommandSignature = '';
      canonicalCommandId = '';
      canonicalIdempotencyKey = '';
      message = `${title} voided successfully.`;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'The canonical purchase could not be voided.';
    } finally {
      busy = false;
    }
  }

  async function savePurchase(action: 'save' | 'post' | 'save-and-post' = 'save-and-post') {
    busy = true;
    message = '';
    error = '';
    let event: SyncEnvelope | undefined;
    try {
      if (isCanonicalPurchaseKind()) {
        if (!online) throw new Error('Canonical purchases require an online API connection; they are not placed in the compatibility queue.');
        await submitCanonicalPurchase(action);
        return;
      }
      event = makeEvent();
      if (!online) {
        await queue.enqueue(event);
        pending = (await queue.pending()).length;
        message = 'Saved to the branch queue for synchronization.';
      } else if (kind === 'return') {
        await api.createReturn(event);
        message = 'Purchase return posted successfully.';
      } else if (kind === 'order') {
        await api.createPurchaseOrder(event);
        message = 'Purchase order saved successfully.';
      } else {
        await api.createReceiving(event);
        message = `${title} posted successfully.`;
      }
    } catch (cause) {
      if (event && (cause instanceof TypeError || !online)) {
        await queue.enqueue(event);
        pending = (await queue.pending()).length;
        message = 'Central API unavailable; saved to the branch queue.';
      } else if (cause instanceof ApiError && cause.status === 401) {
        error = 'Your session has expired. Sign in again.';
      } else {
        error = cause instanceof Error ? cause.message : 'The purchase could not be saved.';
      }
    } finally {
      busy = false;
    }
  }

  function newDocument() {
    invoiceNumber = '';
    supplier = '';
    supplierInvoice = '';
    orderCode = '';
    sourceDocumentId = '';
    sourceDocumentNumber = '';
    remarks = '';
    creditDays = '';
    rows = [blankRow()];
    availableBatches = {};
    returnAllocations = {};
    supplierId = '';
    godownId = '';
    businessDocumentId = '';
    businessDocumentVersion = 0;
    canonicalCommandSignature = '';
    canonicalCommandId = '';
    canonicalIdempotencyKey = '';
    message = 'New document ready.';
    error = '';
  }

  async function flushQueue() {
    busy = true;
    message = '';
    error = '';
    try {
      const configuredEdgeUrl = window.localStorage.getItem('abuzar.edgeUrl');
      const edgeUrl = configuredEdgeUrl || (['localhost', '127.0.0.1'].includes(window.location.hostname) ? 'http://127.0.0.1:8091' : '');
      if (!edgeUrl) throw new Error('Configure the branch-edge URL before syncing the offline queue.');
      const result = await queue.flush(edgeUrl, 500, window.localStorage.getItem('abuzar.edgeSecret') ?? '');
      pending = (await queue.pending()).length;
      message = `${result.accepted} queued event(s) accepted; ${result.duplicates} duplicate(s).`;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Offline queue sync failed.';
    } finally {
      busy = false;
    }
  }

  $: online = typeof navigator === 'undefined' ? true : navigator.onLine;
  $: void api.session().then((result) => {
    if (result.authenticated && result.context) { session = result.context; authenticatedUsername = result.context.username || 'ADMIN'; }
  }).catch(() => { /* The form remains usable offline. */ });
  onMount(() => {
    const clockTimer = window.setInterval(() => { clock = new Date(); }, 1000);
    void queue.pending().then((events) => { pending = events.length; }).catch(() => { pending = 0; });
    void loadCanonicalContext();
    return () => window.clearInterval(clockTimer);
  });
</script>

<svelte:window onkeydown={enableInteractive} />

<svelte:head><title>WASEELA · ABUZAR V3 · {title}</title></svelte:head>

<main class:legacy-pack-purchase-page={kind === 'pack'} class:legacy-pack-purchase-baseline={kind === 'pack' && activeTab === 'detail' && !interactive} class:legacy-purchase-list-page={activeTab === 'list'} class="legacy-transaction-page" onpointerdown={enableInteractive}>
  <section class="legacy-transaction-window" aria-label={title}>
    <header class="legacy-transaction-titlebar">
      <a href="/app/legacy" aria-label="Back to main window">←</a>
      <h1>{transactionWindowTitle}</h1>
    </header>
    <LegacyMenuBar context="pack-purchase" windowId={'purchase-' + kind} windowLabel={title} windowHref={'/app/purchase/' + kind} onCommand={handleMenuCommand} />
    <div class="legacy-transaction-toolbar" role="toolbar" aria-label="Transaction toolbar">
      <button type="button" aria-label="New document" onclick={newDocument} title="New document">▱</button>
      <button type="button" aria-label="Save document" onclick={() => { void savePurchase('save'); }} disabled={busy} title="Save document">▣</button>
      <button type="button" aria-label="Void document" onclick={() => { void voidPurchase(); }} disabled={busy || !businessDocumentId} title="Void document">⊘</button>
      <button type="button" aria-label="Print document" onclick={() => { message = 'Purchase Slip: print preview ready.'; window.print(); }} title="Print">▤</button>
      <span class="legacy-toolbar-separator"></span>
      <button type="button" aria-label="Previous document" onclick={() => { void navigateHistory(-1); }} title="Previous">◀</button>
      <button type="button" aria-label="Next document" onclick={() => { void navigateHistory(1); }} title="Next">▶</button>
      <span class="legacy-toolbar-caption">{online ? 'Online' : 'Offline'} · {title}</span>
    </div>
    <div class="legacy-transaction-tabs"><button aria-label="Detail" aria-pressed={activeTab === 'detail'} class:active={activeTab === 'detail'} type="button" onclick={() => { interactive = true; activeTab = 'detail'; }}>▦ Detail</button><button data-testid="purchase-list-tab" aria-label="List" aria-pressed={activeTab === 'list'} class:active={activeTab === 'list'} type="button" onclick={() => { interactive = true; activeTab = 'list'; void loadHistory(); }}>▦ List</button></div>
    <div class="legacy-transaction-detail">
      <div class="legacy-transaction-fields">
        <label>Invoice No:<input bind:value={invoiceNumber} /></label>
        <label>Alias Name:<input aria-label="Supplier" list="purchase-supplier-options" value={supplier} oninput={(event) => chooseSupplier(event.currentTarget.value)} /></label>
        <label>Supp. Inv. #:<input bind:value={supplierInvoice} /></label>
         <label>Remarks:<input bind:value={remarks} /></label>
         <label>Order Code:<input bind:value={orderCode} /></label>
         <label>Date:<input type="date" bind:value={transactionDate} /></label>
         {#if kind === 'pack' || kind === 'loose' || kind === 'opening'}<label>Credit Days:<input aria-label="Credit days" inputmode="numeric" bind:value={creditDays} /></label>{/if}
         {#if kind === 'return'}<label>Source Document ID:<input aria-label="Source document ID" bind:value={sourceDocumentId} /></label><label>Source Document #:<input aria-label="Source document number" bind:value={sourceDocumentNumber} /></label>{/if}
      </div>
      <div class="legacy-transaction-grid-wrap">
        <table class="legacy-transaction-grid" class:legacy-pack-purchase-grid={kind === 'pack'}>
          <thead>{#if kind === 'pack'}<tr>{#each packHeaders as header}<th>{header}</th>{/each}</tr>{:else}<tr><th>No.</th><th>Quick Search</th><th>Alias Name</th><th>Alternate Alias Name</th><th>Item Name</th><th>Pack Units</th><th>Packing</th><th>Item Location</th><th>Godown</th><th>Batch</th><th>Mfg. Date</th><th>Expiry</th><th>Batch Sale Price</th><th>Quantity</th><th>Purchase Price</th><th>Total</th><th></th>{#if kind === 'return'}<th>Source Batch ID</th>{/if}</tr>{/if}</thead>
          <tbody>
            {#each rows as row, index}
              <tr>
                <td>{index + 1}</td>
                <td><input aria-label={`Quick search ${index + 1}`} list="purchase-item-options" bind:value={row.quickSearch} /><button type="button" aria-label={`Lookup item ${index + 1}`} onclick={(event) => { const input = (event.currentTarget.parentElement?.querySelector('input') as HTMLInputElement | null)?.value ?? ''; void chooseItem(index, input); }}>Lookup</button></td>
                <td></td><td></td>
                <td><input aria-label={`Item name ${index + 1}`} list="purchase-item-options" bind:value={row.itemName} /></td>
                <td><input value={row.packUnits} oninput={(event) => updateRow(index, 'packUnits', event.currentTarget.value)} /></td>
                <td><input value={row.packing} oninput={(event) => updateRow(index, 'packing', event.currentTarget.value)} /></td>
                <td><input value={row.location} oninput={(event) => updateRow(index, 'location', event.currentTarget.value)} /></td>
                <td><input aria-label={`Godown ${index + 1}`} list="purchase-godown-options" value={row.godown} oninput={(event) => chooseGodown(index, event.currentTarget.value)} /></td>
                <td><input aria-label={`Batch ${index + 1}`} list={`purchase-batch-options-${index}`} value={row.batch} oninput={(event) => kind === 'return' ? updateBatch(index, event.currentTarget.value) : updateRow(index, 'batch', event.currentTarget.value)} /></td>
                <td><input aria-label={`Mfg. date ${index + 1}`} type="date" value={row.mfgDate} oninput={(event) => updateRow(index, 'mfgDate', event.currentTarget.value)} /></td>
                <td><input aria-label={`Expiry ${index + 1}`} type="date" value={row.expiry} oninput={(event) => updateRow(index, 'expiry', event.currentTarget.value)} /></td>
                <td><input value={row.batchSalePrice} oninput={(event) => updateRow(index, 'batchSalePrice', event.currentTarget.value)} /></td>
                {#if kind === 'pack'}<td></td><td></td><td></td><td></td>{/if}
                <td><input aria-label={`Quantity ${index + 1}`} value={row.quantity} oninput={(event) => updateRow(index, 'quantity', event.currentTarget.value)} /></td>
                <td><input aria-label={`Purchase price ${index + 1}`} value={row.purchasePrice} oninput={(event) => updateRow(index, 'purchasePrice', event.currentTarget.value)} /></td>
                <td>{row.total}</td>
                <td><button type="button" aria-label={`Remove row ${index + 1}`} onclick={() => removeRow(index)}>×</button></td>
                {#if kind === 'return'}<td><input aria-label={`Source batch ID ${index + 1}`} value={row.sourceBatchId} oninput={(event) => updateSourceBatchId(index, event.currentTarget.value)} /></td>{/if}
              </tr>
            {/each}
          </tbody>
        </table>
        {#if kind === 'return'}{#each rows as _, index}<datalist id={`purchase-batch-options-${index}`}>{#each availableBatches[index] ?? [] as batch}<option value={batch.batchNumber}>{batch.batchId}</option>{/each}</datalist>{/each}{/if}
      </div>
      <datalist id="purchase-item-options">{#each itemRecords as record}<option value={record.name}>{record.code}</option>{/each}</datalist>
      <datalist id="purchase-supplier-options">{#each supplierRecords as record}<option value={record.name}>{record.code}</option>{/each}</datalist>
      <datalist id="purchase-godown-options">{#each godownRecords as record}<option value={record.name}>{record.code}</option>{/each}</datalist>
      {#if kind === 'return'}
        <section class="legacy-return-batch-panel" aria-label="Purchase return source batch allocations">
          <h2>Source batch allocations</h2>
          <p>Choose one or more batches received by the source purchase. Allocation quantities must equal each return line quantity.</p>
          {#each rows as row, index}
            {#if row.itemName.trim() || row.quickSearch.trim()}
              {@const allocations = returnAllocations[index]?.length
                ? returnAllocations[index]
                : [{ batchId: row.sourceBatchId ?? '', batchNumber: row.batch ?? '', quantity: row.quantity || '1' }]}
              <fieldset class="legacy-return-batch-allocation">
                <legend>Line {index + 1}: {row.itemName || row.quickSearch || 'Item'}</legend>
                <label>Source purchase line ID:
                  <input aria-label={`Source purchase line ID ${index + 1}`} value={row.sourceLineId ?? ''} oninput={(event) => updateRow(index, 'sourceLineId', event.currentTarget.value)} />
                </label>
                {#each allocations as allocation, allocationIndex}
                  <div class="legacy-return-batch-allocation-row">
                    <label>Source batch {allocationIndex + 1}:
                      <select aria-label={`Source batch allocation ${index + 1}-${allocationIndex + 1}`} value={allocation.batchId} onchange={(event) => setReturnAllocation(index, allocationIndex, 'batchId', (event.currentTarget as HTMLSelectElement).value)}>
                        <option value="">Select source batch</option>
                        {#each availableBatches[index] ?? [] as batch}
                          <option value={batch.batchId} disabled={allocations.some((candidate, candidateIndex) => candidateIndex !== allocationIndex && candidate.batchId === batch.batchId)}>{batch.batchNumber}{batch.expiryDate ? ` · ${batch.expiryDate}` : ''} · {batch.quantity}</option>
                        {/each}
                      </select>
                    </label>
                    <label>Qty:
                      <input aria-label={`Source allocation quantity ${index + 1}-${allocationIndex + 1}`} type="number" min="0" step="any" value={allocation.quantity} oninput={(event) => setReturnAllocation(index, allocationIndex, 'quantity', (event.currentTarget as HTMLInputElement).value)} />
                    </label>
                    {#if allocations.length > 1}<button type="button" aria-label={`Remove source allocation ${index + 1}-${allocationIndex + 1}`} onclick={() => removeReturnAllocation(index, allocationIndex)}>Remove</button>{/if}
                  </div>
                {/each}
                <button type="button" aria-label={`Add source allocation ${index + 1}`} onclick={() => addReturnAllocation(index)}>Add source batch</button>
                {#if !(availableBatches[index] ?? []).length}<small>Load the source purchase batches through the active godown before selecting additional allocations.</small>{/if}
              </fieldset>
            {/if}
          {/each}
        </section>
      {/if}
      <button class="legacy-add-row" type="button" onclick={addRow}>Add item row</button>
      <div class="legacy-purchase-adjustments" aria-label="Purchase adjustments">
        <label>Item GST %<input aria-label="Item GST percent" bind:value={itemGstRate} /></label>
        <label>Item Discount %<input aria-label="Item discount percent" bind:value={itemDiscountRate} /></label>
        <label>Misc (+)<input aria-label="Purchase expenses" bind:value={miscAmount} /></label>
        <button type="button" onclick={() => { showExpenses = true; }}>Purchase Expenses</button>
      </div>
      <div class="legacy-transaction-totals"><span>{rows.length}</span><span>0</span><span>0.00</span><span>0.00</span><strong>Grand Total: {grandTotal}</strong></div>
	      {#if activeTab === 'list'}<div class="legacy-history-filter" role="search"><label>Filter:<input aria-label="Purchase history filter" bind:value={historyFilter} onkeydown={(event) => { if (event.key === 'Enter') void loadHistory(); }} /></label><button type="button" onclick={() => void loadHistory()}>Filter / Retrieve</button></div><div class="legacy-purchase-list"><table><thead><tr><th>Invoice</th><th>Date</th><th>Supplier</th><th>Item</th><th>Qty</th><th>Total</th></tr></thead><tbody>
        {#if historyBusy}<tr><td colspan="6">Loading transaction history...</td></tr>
        {:else if history.length === 0}<tr><td colspan="6">No transactions found for this date.</td></tr>
        {:else}{#each history as row}<tr><td><button type="button" onclick={() => { void applyHistoryRow(row); }}>{row.document || ''}</button></td><td>{row.occurredAt || ''}</td><td>{row.party || ''}</td><td>{row.item || ''}</td><td>{row.quantity || ''}</td><td>{row.amount || ''}</td></tr>{/each}{/if}
      </tbody></table></div>{/if}
    </div>
    <input class="legacy-hidden-file-input" type="file" multiple bind:this={attachmentInput} onchange={onAttachmentsSelected} aria-label="Attach purchase documents" />
    {#if showExpenses}<div class="legacy-dialog-backdrop" role="presentation"><div class="legacy-simple-dialog" role="dialog" aria-modal="true" aria-label="Purchase Expenses"><h2>Purchase Expenses</h2><label>Misc (+)<input aria-label="Purchase expenses dialog value" bind:value={miscAmount} /></label><p>Expenses are carried into the canonical document pricing snapshot.</p><div><button type="button" onclick={() => { showExpenses = false; }}>Ok</button><button type="button" onclick={() => { showExpenses = false; }}>Cancel</button></div></div></div>{/if}
    {#if showSaleTemplatePicker}<div class="legacy-dialog-backdrop" role="presentation"><div class="legacy-simple-dialog legacy-sale-template-dialog" role="dialog" aria-modal="true" aria-label="Sale Templates"><h2>Populate From Sale Template</h2>{#if saleTemplateBusy}<p>Loading active sale templates...</p>{:else if saleTemplates.length === 0}<p>No active sale templates are available in the current tenant scope.</p>{:else}<table><thead><tr><th>Code</th><th>Name</th><th>Updated</th><th></th></tr></thead><tbody>{#each saleTemplates as template}<tr><td>{template.code}</td><td>{template.name}</td><td>{template.updatedAt.slice(0, 10)}</td><td><button type="button" data-testid={`sale-template-${template.id}`} onclick={() => { void applySaleTemplate(template); }}>Use template</button></td></tr>{/each}</tbody></table>{/if}<div><button type="button" onclick={() => { showSaleTemplatePicker = false; }}>Cancel</button></div></div></div>{/if}
    <div class="legacy-transaction-footer">
      {#if error}<span class="error" role="alert">{error}</span>{:else if message}<span role="status">{message}</span>{:else}<span>Ready</span>{/if}
      <button type="button" class="legacy-sync-button" onclick={flushQueue} disabled={busy || pending === 0}>Sync queue ({pending})</button>
      <a href="/app/legacy">Back to main window</a>
    </div>
  </section>
</main>
