import type {
  BranchSummary,
  CounterSummary,
  ConflictRecord,
  ProblemDetails,
  SessionResponse,
  SyncBatchResult,
  SyncEnvelope,
  TransactionResult,
  TenantSummary,
  MasterRecord,
  ReportRow,
  ReportResponse,
  RoleSummary,
  OperatorCreateRequest,
  OperatorSummary,
  OperatorUpdateRequest,
  ItemSupplier,
  ItemLookupResult,
  InventoryAvailabilityResponse,
  DocumentCommandForKind,
  DocumentCommandResult,
  DocumentKind,
  InventoryBalanceResponse,
  PricingPreviewRequest,
  PricingPreviewResponse,
  AccessResponse,
  RoleRightsResponse,
  Document,
  ApplyItemGSTRequest,
  ItemAlternateAliasesResponse,
  ItemImagesResponse,
  ItemNotesResponse,
  ItemAssociationsResponse,
  ItemAuthorsResponse,
  ItemModelsResponse,
  ItemPricePolicyResponse,
  ItemPricePolicyTier,
  ItemRegistrationRequestResponse,
  ItemUnpostedTransactionsResponse
} from '@abuzar/contracts';

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly problem?: ProblemDetails
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

function configuredBaseUrl(): string {
  if (typeof window === 'undefined') return '';
  return window.localStorage.getItem('abuzar.apiBaseUrl')?.replace(/\/$/, '') ?? '';
}

export function edgeBaseUrl(): string {
  if (typeof window === 'undefined') return '';
  const configured = window.localStorage.getItem('abuzar.edgeUrl')?.replace(/\/$/, '');
  if (configured) return configured;
  return ['localhost', '127.0.0.1'].includes(window.location.hostname) ? 'http://127.0.0.1:8091' : '';
}

export async function edgeRequest<T>(path: string, body: unknown): Promise<T> {
  const baseUrl = edgeBaseUrl();
  if (!baseUrl) throw new Error('Configure the branch-edge URL before using a hardware adapter.');
  const response = await fetch(`${baseUrl}${path}`, {
    method: 'POST',
    credentials: 'omit',
    headers: {
      accept: 'application/json',
      'content-type': 'application/json',
      ...(typeof window !== 'undefined' && window.localStorage.getItem('abuzar.edgeSecret') ? {
        authorization: `Bearer ${window.localStorage.getItem('abuzar.edgeSecret')}`
      } : {})
    },
    body: JSON.stringify(body)
  });
  const result = await response.json().catch(() => undefined);
  if (!response.ok) {
    const problem = result as ProblemDetails | undefined;
    throw new ApiError(problem?.detail ?? `Edge request failed with status ${response.status}`, response.status, problem);
  }
  return result as T;
}

export class AbuzarApi {
  constructor(private readonly baseUrl?: string) {}

  private get base(): string {
    return this.baseUrl ?? configuredBaseUrl();
  }

  async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await fetch(`${this.base}${path}`, {
      ...init,
      credentials: 'include',
      headers: { accept: 'application/json', ...(init.body ? { 'content-type': 'application/json' } : {}), ...init.headers }
    });
    const body = await response.json().catch(() => undefined);
    if (!response.ok) {
      const problem = body as ProblemDetails | undefined;
      throw new ApiError(problem?.detail ?? `Request failed with status ${response.status}`, response.status, problem);
    }
    return body as T;
  }

  session(): Promise<SessionResponse> {
    return this.request('/v1/session');
  }

  setContext(branchId: string, counterId: string): Promise<SessionResponse> {
    return this.request('/v1/session/context', { method: 'POST', body: JSON.stringify({ branchId, counterId }) });
  }

  access(): Promise<AccessResponse> {
    return this.request('/v1/access');
  }

  tenants(): Promise<{ tenants: TenantSummary[] }> {
    return this.request('/v1/tenants');
  }

  branches(): Promise<{ branches: BranchSummary[] }> {
    return this.request('/v1/branches');
  }

  counters(branchId: string): Promise<{ counters: CounterSummary[] }> {
    return this.request(`/v1/counters?branchId=${encodeURIComponent(branchId)}`);
  }

  masterRecords(kind: string, search = ''): Promise<{ records: MasterRecord[] }> {
    const query = search ? `?search=${encodeURIComponent(search)}` : '';
    return this.request(`/v1/master/${encodeURIComponent(kind)}${query}`);
  }

  masterRecord(kind: string, id: string): Promise<MasterRecord> {
    return this.request(`/v1/master/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`);
  }

  deleteMasterRecord(kind: string, id: string): Promise<void> {
    return this.request(`/v1/master/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, { method: 'DELETE' });
  }

  itemLookup(query: string): Promise<{ items: ItemLookupResult[] }> {
    const trimmed = query.trim();
    if (!trimmed) return Promise.resolve({ items: [] });
    return this.request(`/v1/items/lookup?q=${encodeURIComponent(trimmed)}`);
  }

  createMasterRecord(kind: string, record: { code: string; name: string; payload?: Record<string, unknown>; active?: boolean }): Promise<MasterRecord> {
    return this.request(`/v1/master/${encodeURIComponent(kind)}`, { method: 'POST', body: JSON.stringify(record) });
  }

  updateMasterRecord(kind: string, id: string, record: Partial<{ code: string; name: string; payload: Record<string, unknown>; active: boolean }>): Promise<MasterRecord> {
    return this.request(`/v1/master/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(record) });
  }

  replaceItemSuppliers(itemId: string, suppliers: Array<Partial<ItemSupplier> & { legacySupplierId: string }>): Promise<{ suppliers: ItemSupplier[] }> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/suppliers`, {
      method: 'PUT',
      body: JSON.stringify({ suppliers })
    });
  }

  itemSuppliers(itemId: string): Promise<{ suppliers: ItemSupplier[] }> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/suppliers`);
  }

  itemAliases(itemId: string): Promise<ItemAlternateAliasesResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/aliases`);
  }

  replaceItemAliases(itemId: string, aliases: string[]): Promise<ItemAlternateAliasesResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/aliases`, {
      method: 'PUT',
      body: JSON.stringify({ aliases })
    });
  }

  itemImages(itemId: string): Promise<ItemImagesResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/images`);
  }

  replaceItemImages(itemId: string, images: ItemImagesResponse['images']): Promise<ItemImagesResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/images`, {
      method: 'PUT',
      body: JSON.stringify({ images })
    });
  }

  itemNotes(itemId: string): Promise<ItemNotesResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/notes`);
  }

  replaceItemNotes(itemId: string, notesData: string): Promise<ItemNotesResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/notes`, {
      method: 'PUT',
      body: JSON.stringify({ notesData })
    });
  }

  itemAssociations(itemId: string): Promise<ItemAssociationsResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/associations`);
  }

  replaceItemAssociations(itemId: string, legacyItemIds: string[]): Promise<ItemAssociationsResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/associations`, {
      method: 'PUT',
      body: JSON.stringify({ legacyItemIds })
    });
  }

  itemAuthors(itemId: string): Promise<ItemAuthorsResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/authors`);
  }

  replaceItemAuthors(itemId: string, authors: ItemAuthorsResponse['authors']): Promise<ItemAuthorsResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/authors`, {
      method: 'PUT',
      body: JSON.stringify({ authors })
    });
  }

  itemModels(itemId: string): Promise<ItemModelsResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/models`);
  }

  replaceItemModels(itemId: string, modelCodes: number[]): Promise<ItemModelsResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/models`, {
      method: 'PUT',
      body: JSON.stringify({ modelCodes })
    });
  }

  itemPricePolicy(itemId: string): Promise<ItemPricePolicyResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/price-policy`);
  }

  replaceItemPricePolicy(itemId: string, policyCode: string, tiers: ItemPricePolicyTier[]): Promise<ItemPricePolicyResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/price-policy`, {
      method: 'PUT',
      body: JSON.stringify({ policyCode, tiers })
    });
  }

  itemRegistrationRequest(itemId: string): Promise<ItemRegistrationRequestResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/registration-request`);
  }

  populateItemRegistrationRequest(itemId: string): Promise<ItemRegistrationRequestResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/registration-request`, { method: 'POST', body: JSON.stringify({}) });
  }

  itemUnpostedTransactions(itemId: string): Promise<ItemUnpostedTransactionsResponse> {
    return this.request(`/v1/master/item/${encodeURIComponent(itemId)}/unposted-transactions`);
  }

  report(kind: string, from = '', to = '', filter = '', options: { page?: number; pageSize?: number; cash?: boolean; credit?: boolean; areas?: string[]; allAreas?: boolean; legacyPath?: string; godownId?: string; batchNumber?: string; format?: string } = {}): Promise<ReportResponse> {
    const params = new URLSearchParams({ from, to, filter });
    if (options.page) params.set('page', String(options.page));
    if (options.pageSize) params.set('pageSize', String(options.pageSize));
    if (options.cash !== undefined) params.set('cash', String(options.cash));
    if (options.credit !== undefined) params.set('credit', String(options.credit));
    if (options.areas?.length) params.set('areas', options.areas.join(','));
    if (options.allAreas !== undefined) params.set('allAreas', String(options.allAreas));
    if (options.legacyPath) params.set('legacyPath', options.legacyPath);
    if (options.godownId) params.set('godownId', options.godownId);
    if (options.batchNumber) params.set('batchNumber', options.batchNumber);
    if (options.format) params.set('format', options.format);
    return this.request(`/v1/reports/${encodeURIComponent(kind)}?${params.toString()}`);
  }

  transactions(kind: string, from = '', to = '', filter = ''): Promise<{ kind: string; rows: ReportRow[] }> {
    const params = new URLSearchParams({ from, to, filter });
    return this.request(`/v1/transactions/${encodeURIComponent(kind)}?${params.toString()}`);
  }

  document(id: string): Promise<Document> {
    return this.request(`/v1/documents/${encodeURIComponent(id)}`);
  }

  previewPricing(request: PricingPreviewRequest): Promise<PricingPreviewResponse> {
    return this.request('/v1/transactions/preview', { method: 'POST', body: JSON.stringify(request) });
  }

  applyItemGST(request: ApplyItemGSTRequest): Promise<{ rateId: string; itemsApplied: number; effectiveFrom: string; effectiveTo?: string }> {
    return this.request('/v1/tax-assignments/apply-item-gst', { method: 'POST', body: JSON.stringify(request) });
  }

  documentCommand<K extends DocumentKind>(kind: K, command: DocumentCommandForKind<K>): Promise<DocumentCommandResult<K>> {
    return this.request(`/v1/documents/${encodeURIComponent(kind)}`, { method: 'POST', body: JSON.stringify(command) });
  }

  inventoryBalance(itemLegacyId: string): Promise<InventoryBalanceResponse> {
    return this.request(`/v1/inventory/balance?itemLegacyId=${encodeURIComponent(itemLegacyId)}`);
  }

  inventoryAvailability(itemLegacyId: string, godownId: string): Promise<InventoryAvailabilityResponse> {
    const params = new URLSearchParams({ itemLegacyId, godownId });
    return this.request(`/v1/inventory/availability?${params.toString()}`);
  }

  maintenance(kind: string, payload: Record<string, unknown> = {}): Promise<{ kind: string; status: string; outcome?: string; operationId?: string; message?: string; saved?: number; checks?: Array<{ table: string; rows: number; status: string }> }> {
    return this.request(`/v1/maintenance/${encodeURIComponent(kind)}`, { method: 'POST', body: JSON.stringify(payload) });
  }

  maintenanceState(kind: string): Promise<{ kind: string; items: Array<{ caption: string; value: string; position: number }>; operations?: Array<{ id: string; kind: string; status: string; message: string; occurredAt: string }>; lastOperation?: { id: string; kind: string; status: string; message: string; occurredAt: string } | null }> {
    return this.request(`/v1/maintenance/${encodeURIComponent(kind)}`);
  }

  preferences(category: string): Promise<{
    category: string;
    scope?: { tenantId: string; branchId: string };
    items: Array<{ caption: string; fieldKey?: string; value: string; position: number }>;
    registry?: Array<{
      caption: string;
      type: string;
      default: string;
      value: string;
      allowed?: string[];
      minimum?: number;
      maximum?: number;
      behavior: string;
      runtimeStatus: string;
      position: number;
    }>;
    divergences?: Array<{ category: string; status: string; detail: string }>;
  }> {
    return this.request(`/v1/preferences?category=${encodeURIComponent(category)}`);
  }

  savePreferences(category: string, items: Array<{ caption: string; fieldKey?: string; value: string; position?: number }>): Promise<{ category: string; saved: number }> {
    return this.request('/v1/preferences', { method: 'PUT', body: JSON.stringify({ category, items }) });
  }

  openShift(amount = '0'): Promise<Record<string, unknown>> {
    return this.request('/v1/shifts/open', { method: 'POST', body: JSON.stringify({ amount }) });
  }

  closeShift(id: string, amount = '0'): Promise<Record<string, unknown>> {
    return this.request(`/v1/shifts/${encodeURIComponent(id)}/close`, { method: 'POST', body: JSON.stringify({ amount }) });
  }

  createSale(event: SyncEnvelope): Promise<TransactionResult> {
    return this.request('/v1/transactions/sales', { method: 'POST', body: JSON.stringify(event) });
  }

  createSaleReturn(event: SyncEnvelope): Promise<TransactionResult> {
    return this.request('/v1/transactions/sale-returns', { method: 'POST', body: JSON.stringify(event) });
  }

  createQuotation(event: SyncEnvelope): Promise<TransactionResult> {
    return this.request('/v1/transactions/quotations', { method: 'POST', body: JSON.stringify(event) });
  }

  createRefusedSale(event: SyncEnvelope): Promise<TransactionResult> {
    return this.request('/v1/transactions/refused-sales', { method: 'POST', body: JSON.stringify(event) });
  }

  createReturn(event: SyncEnvelope): Promise<TransactionResult> {
    return this.request('/v1/transactions/returns', { method: 'POST', body: JSON.stringify(event) });
  }

  createReceiving(event: SyncEnvelope): Promise<TransactionResult> {
    return this.request('/v1/transactions/receiving', { method: 'POST', body: JSON.stringify(event) });
  }

  createPurchaseOrder(event: SyncEnvelope): Promise<TransactionResult> {
    return this.request('/v1/transactions/purchase-orders', { method: 'POST', body: JSON.stringify(event) });
  }

  createInventoryTransaction(event: SyncEnvelope): Promise<TransactionResult> {
    return this.request('/v1/transactions/inventory', { method: 'POST', body: JSON.stringify(event) });
  }

  push(events: SyncEnvelope[]): Promise<SyncBatchResult> {
    return this.request('/v1/sync/push', { method: 'POST', body: JSON.stringify({ events }) });
  }

  conflicts(): Promise<{ conflicts: ConflictRecord[] }> {
    return this.request('/v1/conflicts');
  }

  roles(): Promise<{ roles: RoleSummary[] }> {
    return this.request('/v1/roles');
  }

  operators(): Promise<{ operators: OperatorSummary[] }> {
    return this.request('/v1/operators');
  }

  createOperator(request: OperatorCreateRequest): Promise<OperatorSummary> {
    return this.request('/v1/operators', { method: 'POST', body: JSON.stringify(request) });
  }

  updateOperator(id: string, request: OperatorUpdateRequest): Promise<OperatorSummary> {
    return this.request(`/v1/operators/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(request) });
  }

  createRole(code: string, name: string, permissions: string[] = []): Promise<RoleSummary> {
    return this.request('/v1/roles', { method: 'POST', body: JSON.stringify({ code, name, permissions }) });
  }

  updateRole(id: string, code: string, name: string, permissions: string[] = []): Promise<RoleSummary> {
    return this.request(`/v1/roles/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify({ code, name, permissions }) });
  }

  roleRights(id: string): Promise<RoleRightsResponse> {
    return this.request(`/v1/roles/${encodeURIComponent(id)}/rights`);
  }

  updateRoleRights(id: string, request: {
    permissions?: string[];
    legacyRights?: Array<{ rightCode: string; allowed: boolean }>;
    scopes?: Array<{ scopeKind: string; scopeKey: string; allowed: boolean }>;
  }): Promise<RoleRightsResponse> {
    return this.request(`/v1/roles/${encodeURIComponent(id)}/rights`, {
      method: 'PATCH',
      body: JSON.stringify(request)
    });
  }

  resolveConflict(id: string, status: 'resolved' | 'dismissed', resolution: unknown = {}): Promise<{ id: string; status: string }> {
    return this.request(`/v1/conflicts/${encodeURIComponent(id)}/resolve`, {
      method: 'POST',
      body: JSON.stringify({ status, resolution })
    });
  }

  logout(): Promise<void> {
    return this.request('/v1/auth/logout', { method: 'POST' }).then(() => undefined);
  }

  changePassword(currentPassword: string, newPassword: string, confirmPassword: string): Promise<{ changed: boolean }> {
    return this.request('/v1/auth/change-password', { method: 'POST', body: JSON.stringify({ currentPassword, newPassword, confirmPassword }) });
  }

  shifts(): Promise<{ shifts: Array<{ id: string; branchId: string; counterId: string; operatorId: string; openedAt: string; closedAt?: string; status: string; openingAmount: string; closingAmount?: string }> }> {
    return this.request('/v1/shifts');
  }

  sessionMonitor(): Promise<{ tenantId: string; branchId: string; sessions: Array<{ userId: string; username: string; displayName: string; branchId: string; counterId: string; createdAt: string; lastSeenAt: string; expiresAt: string; current: boolean }> }> {
    return this.request('/v1/session-monitor');
  }
}

type QueueRecord = SyncEnvelope & { queuedAt: string };

export class OfflineQueue {
  private readonly databaseName = 'abuzar-next-offline';
  private readonly storeName = 'events';

  async enqueue(event: SyncEnvelope): Promise<void> {
    const database = await this.open();
    await this.write(database, 'readwrite', (store) => store.put({ ...event, queuedAt: new Date().toISOString() }));
    database.close();
  }

  async pending(limit = 500): Promise<QueueRecord[]> {
    const database = await this.open();
    const records = await this.read(database, limit);
    database.close();
    return records;
  }

  async flush(edgeBaseUrl: string, limit = 500, sharedSecret = ''): Promise<SyncBatchResult> {
    const records = await this.pending(limit);
    if (records.length === 0) return { accepted: 0, duplicates: 0, conflicts: 0 };
    const response = await fetch(`${edgeBaseUrl.replace(/\/$/, '')}/v1/sync/push`, {
      method: 'POST',
      headers: {
        accept: 'application/json',
        'content-type': 'application/json',
        ...(sharedSecret ? { authorization: `Bearer ${sharedSecret}` } : {})
      },
      body: JSON.stringify({ events: records })
    });
    const result = (await response.json().catch(() => undefined)) as SyncBatchResult | ProblemDetails | undefined;
    if (!response.ok) {
      throw new ApiError((result as ProblemDetails | undefined)?.detail ?? 'Offline queue flush failed', response.status, result as ProblemDetails | undefined);
    }
    const database = await this.open();
    await this.write(database, 'readwrite', (store) => {
      for (const record of records) store.delete(record.eventId);
    });
    database.close();
    return result as SyncBatchResult;
  }

  private open(): Promise<IDBDatabase> {
    return new Promise((resolve, reject) => {
      const request = indexedDB.open(this.databaseName, 1);
      request.onupgradeneeded = () => {
        request.result.createObjectStore(this.storeName, { keyPath: 'eventId' });
      };
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error ?? new Error('Unable to open offline queue'));
    });
  }

  private write(database: IDBDatabase, mode: IDBTransactionMode, action: (store: IDBObjectStore) => void): Promise<void> {
    return new Promise((resolve, reject) => {
      const transaction = database.transaction(this.storeName, mode);
      action(transaction.objectStore(this.storeName));
      transaction.oncomplete = () => resolve();
      transaction.onerror = () => reject(transaction.error ?? new Error('Offline queue write failed'));
      transaction.onabort = () => reject(transaction.error ?? new Error('Offline queue write aborted'));
    });
  }

  private read(database: IDBDatabase, limit: number): Promise<QueueRecord[]> {
    return new Promise((resolve, reject) => {
      const records: QueueRecord[] = [];
      const request = database.transaction(this.storeName, 'readonly').objectStore(this.storeName).openCursor();
      request.onsuccess = () => {
        const cursor = request.result;
        if (!cursor || records.length >= limit) {
          resolve(records);
          return;
        }
        records.push(cursor.value as QueueRecord);
        cursor.continue();
      };
      request.onerror = () => reject(request.error ?? new Error('Offline queue read failed'));
    });
  }
}

export function newEventId(): string {
  return typeof crypto.randomUUID === 'function' ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
