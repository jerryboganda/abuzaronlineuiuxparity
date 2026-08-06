<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import type { DocumentCommandForKind, InventoryAvailableBatch, ItemLookupResult, MasterRecord, PurchaseDocumentKind, SessionResponse, SyncEnvelope, ReportRow } from '@abuzar/contracts';
  import { AbuzarApi, ApiError, OfflineQueue, newEventId } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';
  import type { MenuAction } from '$lib/legacy-menu';
  import { formatLegacyTitle } from '$lib/legacy-title';
  import { localDateAtNoonUtc, localDateString } from '$lib/calendar-date';

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
  let clock = new Date();
  let authenticatedUsername = 'ADMIN';
  let canonicalCommandSignature = '';
  let canonicalCommandId = '';
  let canonicalIdempotencyKey = '';
  let itemGstRate = '';
  let itemDiscountRate = '';
  let miscAmount = '0';
  let showExpenses = false;
  let attachmentInput: HTMLInputElement | null = null;
  let attachments: Array<{ name: string; size: number }> = [];

  $: kind = $page?.params?.kind ?? 'pack';
  $: title = titles[kind] ?? 'Purchase';
  $: historyKind = kind === 'return' ? 'purchase-return' : kind === 'order' ? 'purchase-order' : kind === 'loose' ? 'loose-purchase' : kind === 'opening' ? 'opening-purchase' : 'pack-purchase';
  $: transactionWindowTitle = `${formatLegacyTitle(authenticatedUsername, clock)} - [${title}]`;
  function isCanonicalPurchaseKind(): boolean {
    return supportedPurchaseRouteKinds.includes(kind);
  }

  async function loadHistory() {
    historyBusy = true;
    try {
      history = (await api.transactions(historyKind, transactionDate, transactionDate)).rows;
    } catch {
      history = [];
    } finally {
      historyBusy = false;
    }
  }

  function applyHistoryRow(row: ReportRow) {
    invoiceNumber = row.document || '';
    supplier = row.party || supplier;
    rows = [{ ...blankRow(), itemName: row.item || '', quantity: row.quantity || '1', total: row.amount || '0.00' }];
    activeTab = 'detail';
    message = `${title} ${invoiceNumber || 'document'} loaded.`;
  }

  async function navigateHistory(offset: number) {
    if (!history.length) await loadHistory();
    if (!history.length) { message = 'No persisted documents found for this date.'; return; }
    const current = history.findIndex((row) => row.document === invoiceNumber);
    const next = current < 0 ? (offset > 0 ? 0 : history.length - 1) : (current + offset + history.length) % history.length;
    applyHistoryRow(history[next]);
  }

  async function navigateHistoryTo(index: number) {
    if (!history.length) await loadHistory();
    if (!history.length) { message = 'No persisted documents found for this date.'; return; }
    applyHistoryRow(history[index < 0 ? history.length - 1 : Math.min(index, history.length - 1)]);
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
    const labels = rows.filter((row) => row.itemName.trim()).map((row) => `${row.itemName} | Batch: ${row.batch || 'N/A'} | Qty: ${row.quantity || '0'} | Price: ${row.purchasePrice || '0.00'}`);
    if (!labels.length) {
      message = 'Print Purchase Labels: enter at least one item first.';
      return;
    }
    message = `Print Purchase Labels: preview ready for ${labels.length} item${labels.length === 1 ? '' : 's'}.`;
    window.print();
  }

  function onAttachmentsSelected(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    attachments = Array.from(input.files ?? []).map((file) => ({ name: file.name, size: file.size }));
    message = attachments.length ? `${attachments.length} document${attachments.length === 1 ? '' : 's'} attached to this draft.` : 'No documents selected.';
  }

  $: if (activeTab === 'list' && historyKind && transactionDate) void loadHistory();

  function enableInteractive(event?: Event) {
    const target = event?.target;
    if (target instanceof Element && target.closest('.legacy-transaction-tabs')) return;
    interactive = true;
  }

  function handleMenuCommand(action: MenuAction): boolean {
    switch (action.label) {
      case 'New':
        newDocument();
        return true;
      case 'List':
        activeTab = 'list';
        void loadHistory();
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
        activeTab = 'detail';
        message = 'Populate Items: choose synchronized item identities in the grid before posting.';
        return true;
      case 'Populate Sales Order':
      case 'Populate Pending Due Item(s)':
      case 'Populate Purchase Invoice':
      case 'Populate Purchase Return Invoice':
      case 'Populate From Sale Template':
      case 'Fetch Purchase Invoice From Other Sources':
        activeTab = 'list';
        void loadHistory();
        message = `${action.label}: persisted purchase history is ready for selection.`;
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
        activeTab = 'list';
        void loadHistory();
        message = 'Item Purchase History: filtered transaction list ready.';
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
      case 'Supplier Info.':
        window.location.assign('/app/master/supplier');
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
      if ((key === 'itemName' || key === 'quickSearch') && value !== row[key]) { next.itemId = undefined; next.itemLegacyId = ''; next.sourceBatchId = ''; }
      if (key === 'quantity' || key === 'purchasePrice') next.total = ((Number(next.quantity) || 0) * (Number(next.purchasePrice) || 0)).toFixed(2);
      return next;
    });
  }

  async function chooseItem(index: number, value: string) {
    const records = await lookupItems(value);
    const normalized = value.trim().toLowerCase();
    const match = records.find((record) => record.code.toLowerCase() === normalized
      || record.name.toLowerCase() === normalized
      || record.legacyId.toLowerCase() === normalized
      || record.aliases.some((alias) => alias.toLowerCase() === normalized));
    updateRow(index, 'quickSearch', value);
    if (match) {
      updateRow(index, 'itemName', match.name);
      updateRow(index, 'itemLegacyId', match.legacyId || match.id);
      rows = rows.map((row, rowIndex) => rowIndex === index ? { ...row, itemId: match.id } : row);
      void refreshRowBatches(index);
    }
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

  function updateBatch(index: number, value: string) {
    updateRow(index, 'batch', value);
    if (kind !== 'return') return;
    const match = (availableBatches[index] ?? []).find((batch) => batch.batchNumber.toLowerCase() === value.trim().toLowerCase());
    rows = rows.map((row, rowIndex) => rowIndex === index ? { ...row, sourceBatchId: match?.batchId ?? row.sourceBatchId } : row);
  }

  function removeRow(index: number) {
    rows = rows.filter((_, rowIndex) => rowIndex !== index);
    if (!rows.length) rows = [blankRow()];
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
      payload: { kind, invoiceNumber, supplier, supplierInvoice, orderCode, remarks, rows, status: 'posted' }
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
      if (lineRows.some((row) => !row.batch.trim() || !isUuid(row.sourceBatchId))) throw new Error('Select an explicit canonical source batch and batch ID for every return line.');
    } else if (requestedKind !== 'purchase-order') {
      if (lineRows.some((row) => !row.batch.trim() || !row.expiry.trim() || !row.purchasePrice.trim())) throw new Error('Batch, expiry, and unit cost are required for every purchase receipt.');
    }
    if (lineRows.some((row) => !row.quantity.trim() || Number(row.quantity) <= 0)) throw new Error('Every canonical purchase line requires a positive quantity.');
    if (lineRows.some((row) => Number(row.purchasePrice || '0') < 0)) throw new Error('Purchase unit cost cannot be negative.');
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
        allocations: [{ batchId: row.sourceBatchId, batchNumber: row.batch, quantity: row.quantity }]
      } : {}),
      ...(documentKind === 'purchase-order' ? {} : { batchNumber: row.batch, expiryDate: row.expiry })
    }));
    const signature = JSON.stringify({ documentKind, action, documentId: businessDocumentId, version: businessDocumentVersion, supplierId, godownId, sourceDocumentId, sourceDocumentNumber, lines, transactionDate });
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
        ...(documentKind === 'purchase-return' ? { sourceDocumentId: sourceDocumentId.trim(), sourceDocumentNumber: sourceDocumentNumber.trim() } : {}),
        reference: supplierInvoice,
        remarks,
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
    rows = [blankRow()];
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

<main class:legacy-pack-purchase-page={kind === 'pack'} class:legacy-pack-purchase-baseline={kind === 'pack' && activeTab === 'detail' && !interactive} class:legacy-purchase-list-page={activeTab === 'list'} class="legacy-transaction-page" onpointerdown={enableInteractive} onfocusin={enableInteractive}>
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
                {#if kind === 'return'}<td><input aria-label={`Source batch ID ${index + 1}`} value={row.sourceBatchId} oninput={(event) => updateRow(index, 'sourceBatchId', event.currentTarget.value)} /></td>{/if}
              </tr>
            {/each}
          </tbody>
        </table>
        {#if kind === 'return'}{#each rows as _, index}<datalist id={`purchase-batch-options-${index}`}>{#each availableBatches[index] ?? [] as batch}<option value={batch.batchNumber}>{batch.batchId}</option>{/each}</datalist>{/each}{/if}
      </div>
      <datalist id="purchase-item-options">{#each itemRecords as record}<option value={record.name}>{record.code}</option>{/each}</datalist>
      <datalist id="purchase-supplier-options">{#each supplierRecords as record}<option value={record.name}>{record.code}</option>{/each}</datalist>
      <datalist id="purchase-godown-options">{#each godownRecords as record}<option value={record.name}>{record.code}</option>{/each}</datalist>
      <button class="legacy-add-row" type="button" onclick={addRow}>Add item row</button>
      <div class="legacy-purchase-adjustments" aria-label="Purchase adjustments">
        <label>Item GST %<input aria-label="Item GST percent" bind:value={itemGstRate} /></label>
        <label>Item Discount %<input aria-label="Item discount percent" bind:value={itemDiscountRate} /></label>
        <label>Misc (+)<input aria-label="Purchase expenses" bind:value={miscAmount} /></label>
        <button type="button" onclick={() => { showExpenses = true; }}>Purchase Expenses</button>
      </div>
      <div class="legacy-transaction-totals"><span>{rows.length}</span><span>0</span><span>0.00</span><span>0.00</span><strong>Grand Total: {grandTotal}</strong></div>
      {#if activeTab === 'list'}<div class="legacy-purchase-list"><table><thead><tr><th>Invoice</th><th>Date</th><th>Supplier</th><th>Item</th><th>Qty</th><th>Total</th></tr></thead><tbody>
        {#if historyBusy}<tr><td colspan="6">Loading transaction history...</td></tr>
        {:else if history.length === 0}<tr><td colspan="6">No transactions found for this date.</td></tr>
        {:else}{#each history as row}<tr><td><button type="button" onclick={() => applyHistoryRow(row)}>{row.document || ''}</button></td><td>{row.occurredAt || ''}</td><td>{row.party || ''}</td><td>{row.item || ''}</td><td>{row.quantity || ''}</td><td>{row.amount || ''}</td></tr>{/each}{/if}
      </tbody></table></div>{/if}
    </div>
    <input class="legacy-hidden-file-input" type="file" multiple bind:this={attachmentInput} onchange={onAttachmentsSelected} aria-label="Attach purchase documents" />
    {#if showExpenses}<div class="legacy-dialog-backdrop" role="presentation"><div class="legacy-simple-dialog" role="dialog" aria-modal="true" aria-label="Purchase Expenses"><h2>Purchase Expenses</h2><label>Misc (+)<input aria-label="Purchase expenses dialog value" bind:value={miscAmount} /></label><p>Expenses are carried into the canonical document pricing snapshot.</p><div><button type="button" onclick={() => { showExpenses = false; }}>Ok</button><button type="button" onclick={() => { showExpenses = false; }}>Cancel</button></div></div></div>{/if}
    <div class="legacy-transaction-footer">
      {#if error}<span class="error" role="alert">{error}</span>{:else if message}<span role="status">{message}</span>{:else}<span>Ready</span>{/if}
      <button type="button" class="legacy-sync-button" onclick={flushQueue} disabled={busy || pending === 0}>Sync queue ({pending})</button>
      <a href="/app/legacy">Back to main window</a>
    </div>
  </section>
</main>
