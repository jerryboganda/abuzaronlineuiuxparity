<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import type { BranchSummary, CounterSummary, ItemSupplier, MasterRecord, OperatorSummary, RoleSummary } from '@abuzar/contracts';
  import { AbuzarApi, ApiError } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';
  import { formatLegacyTitle } from '$lib/legacy-title';

  type Field = { label: string; key?: string; kind?: 'text' | 'date' | 'select' | 'number' | 'textarea'; value?: string; options?: string[] };
  const definitions: Record<string, { title: string; fields: Field[] }> = {
    customer: { title: 'Customer', fields: [
      { label: 'Code' }, { label: 'Name' }, { label: 'Opening Date', kind: 'date' }, { label: 'Name (Local Language)' }, { label: 'Alias Name' }, { label: 'Schedule For Posting', kind: 'select', options: ['Yes', 'No'] }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }, { label: 'Contact Person' }, { label: 'Membership ID' }, { label: 'Contact Person 2' }, { label: 'Designation' }, { label: 'Address1' }, { label: 'Address2' }, { label: 'City' }, { label: 'City/County' }, { label: 'Post Code' }, { label: 'Country' }, { label: 'Phone' }, { label: 'Fax' }, { label: 'Area', kind: 'select', options: [''] }, { label: 'Sub Area', kind: 'select', options: [''] }, { label: 'Category', kind: 'select', options: [''] }, { label: 'Bank' }, { label: 'Bank Address' }, { label: 'Special Instructions', kind: 'textarea' }, { label: 'Email Address' }, { label: 'Message', kind: 'select', options: [''] }, { label: 'Licence No.' }, { label: 'Lic. Expiry', kind: 'date' }, { label: 'Tax Reg. No. etc' }, { label: 'SalesMan', kind: 'select', options: [''] }, { label: 'NTN#' }, { label: 'Sale < Avg. Price', kind: 'select', options: ['Yes', 'No'] }, { label: 'Collection Policy', kind: 'select', options: [''] }, { label: 'Asso. Point (Sale/Issue)', kind: 'select', options: [''] }
    ] },
    supplier: { title: 'Supplier', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Opening Date', kind: 'date' }, { label: 'Alias Name' }, { label: 'Address' }, { label: 'City' }, { label: 'Country' }, { label: 'Phone' }, { label: 'Contact Person' }, { label: 'Email Address' }, { label: 'Tax Reg. No.' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }, { label: 'Remarks', kind: 'textarea' }] },
    item: { title: 'Item', fields: [{ label: 'Code/No.', key: 'code' }, { label: 'Name', key: 'name' }, { label: 'Pieces In Packing', key: 'PiecesInPacking', kind: 'number' }, { label: 'Purchase Price', key: 'PurchasePrice', kind: 'number' }, { label: 'Sales Price', key: 'SalePrice', kind: 'number' }, { label: 'Manufacturer', key: 'Manufacturer' }, { label: 'Alias Name', key: 'AliasName' }, { label: 'Pack Sales Tax', key: 'PackSalesTax', kind: 'number' }, { label: 'Printable', key: 'Printable', kind: 'select', options: ['Yes', 'No'] }, { label: 'Category', key: 'Category', kind: 'select', options: [''] }, { label: 'Class', key: 'Class' }, { label: 'Sale Disc(%)', key: 'SaleDiscPercent', kind: 'number' }, { label: 'Location', key: 'Location' }, { label: 'Restricted', key: 'Restricted', kind: 'select', options: ['NO', 'YES'] }, { label: 'Narcotics', key: 'Narcotics', kind: 'select', options: ['No', 'Yes'] }, { label: 'Lock DiscPerc', key: 'LockDiscPercent', kind: 'select', options: ['No', 'Yes'] }, { label: 'Active', key: 'active', kind: 'select', options: ['Yes', 'No'] }, { label: 'Min. Qty', key: 'MinimumQuantity', kind: 'number' }, { label: 'Generic Name', key: 'GenericName' }, { label: 'Taxable Item', key: 'TaxableItem', kind: 'select', options: ['No', 'Yes'] }, { label: 'S/Tax Schedule', key: 'SalesTaxSchedule', kind: 'select', options: [''] }, { label: 'PCT Code', key: 'PCTCode', kind: 'select', options: [''] }, { label: 'Commission(%)', key: 'CommissionPercent', kind: 'number' }] },
    manufacturer: { title: 'Manufacturer', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Alias Name' }, { label: 'Address' }, { label: 'City' }, { label: 'Country' }, { label: 'Phone' }, { label: 'Contact Person' }, { label: 'Email Address' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }, { label: 'Remarks', kind: 'textarea' }] },
    'item-group': { title: 'Item Group', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }] },
    'item-class': { title: 'Item Group', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }] },
    category: { title: 'Category', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Category Kind', kind: 'select', options: ['category', 'item_category', 'customer_category', 'supplier_category', 'manufacturer_category'] }, { label: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }] },
    'item-category': { title: 'Item Category', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }] },
    'customer-category': { title: 'Customer Category', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }] },
    'supplier-category': { title: 'Supplier Category', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }] },
    'manufacturer-category': { title: 'Manufacturer Category', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }] },
    godown: { title: 'Godown', fields: [{ label: 'Code' }, { label: 'Name' }, { label: 'Address' }, { label: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }] },
    user: { title: 'Users', fields: [{ label: 'User Code' }, { label: 'User Name' }, { label: 'Password', kind: 'text' }, { label: 'Confirm Password', kind: 'text' }, { label: 'Active', kind: 'select', value: 'YES', options: ['YES', 'NO'] }, { label: 'Group', kind: 'select', options: ['ADMIN'] }, { label: 'Phone' }, { label: 'Remarks', kind: 'textarea' }] }
  };
  const auxiliaryMasterDefinitions: Record<string, { title: string; fields: Field[] }> = {
    'sale-promotion': { title: 'Sale Promotion', fields: [
      { label: 'Code', key: 'SalePromotionCode' }, { label: 'Name', key: 'Name' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] },
      { label: 'Customer Group', key: 'CustomerGroupCode' }, { label: 'Default Item Disc(%)', key: 'DefaultItemDiscPerc', kind: 'number' },
      { label: 'Week Day', key: 'WeekDayCode' }, { label: 'Start Date', key: 'StartDate', kind: 'date' }, { label: 'End Date', key: 'EndDate', kind: 'date' }, { label: 'Remarks', key: 'Remarks', kind: 'textarea' }
    ] },
    'customer-sector': { title: 'Customer Sector', fields: [
      { label: 'Code', key: 'CustomerSectorCode' }, { label: 'Name', key: 'Name' }, { label: 'Alias Name', key: 'AliasName' },
      { label: 'Description', key: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'generic-item': { title: 'Generic Item', fields: [
      { label: 'Code', key: 'GenericCode' }, { label: 'Name', key: 'Name' }, { label: 'Instruction', key: 'Instruction', kind: 'textarea' },
      { label: 'Description', key: 'Description', kind: 'textarea' }, { label: 'Generic Item Type', key: 'GenericItemTypeCode' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'item-basic-data': { title: 'Item Basic Data', fields: [
      { label: 'Code', key: 'ICode' }, { label: 'Name', key: 'Name' }, { label: 'Manufacturer', key: 'ManfCode' }, { label: 'Category', key: 'ICatCode' },
      { label: 'Class', key: 'ICCode' }, { label: 'Location', key: 'Location' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }, { label: 'Remarks', key: 'Remarks', kind: 'textarea' }
    ] },
    'price-policy': { title: 'Price Policy', fields: [
      { label: 'Code', key: 'PricePolicyCode' }, { label: 'Name', key: 'Name' }, { label: 'Item Code', key: 'ICode' },
      { label: 'Active', kind: 'select', options: ['YES', 'NO'] }, { label: 'Remarks', key: 'Remarks', kind: 'textarea' }
    ] },
    'item-alert': { title: 'Item Alert', fields: [
      { label: 'Code', key: 'ItemAlertCode' }, { label: 'Name', key: 'Name' }, { label: 'Alert Type', key: 'ItemAlertTypeCode' },
      { label: 'Alert Detail', key: 'AlertDetail', kind: 'textarea' }, { label: 'Background Color', key: 'BGColor' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'sales-tax-schedule': { title: 'Sales Tax Schedule', fields: [
      { label: 'Code', key: 'SalesTaxScheduleCode' }, { label: 'Name', key: 'Name' }, { label: 'Tax Percent', key: 'TaxPerc', kind: 'number' },
      { label: 'Tax Type', key: 'TaxType' }, { label: 'Applicable', key: 'Applicable' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'pct-codes': { title: 'PCT Codes', fields: [
      { label: 'Code', key: 'PCTCode' }, { label: 'Name', key: 'Description' }, { label: 'Remarks', key: 'Remarks', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'generic-item-type': { title: 'Generic Item Type', fields: [
      { label: 'Code', key: 'GenericItemTypeCode' }, { label: 'Name', key: 'Name' }, { label: 'Description', key: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'item-thickness': { title: 'Item Thickness', fields: [
      { label: 'Code', key: 'ItemThicknessCode' }, { label: 'Name', key: 'Name' }, { label: 'Description', key: 'Description', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'lock-reason': { title: 'Lock Reason', fields: [
      { label: 'Code', key: 'LockReasonCode' }, { label: 'Name', key: 'Name' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'category-segment': { title: 'Category Segment', fields: [
      { label: 'Code', key: 'CategorySegmentCode' }, { label: 'Name', key: 'Name' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'manufacturer-type': { title: 'Manufacturer Type', fields: [
      { label: 'Code', key: 'ManufacturerTypeCode' }, { label: 'Name', key: 'Name' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    'sale-template': { title: 'Sale Template', fields: [
      { label: 'Code', key: 'SaleTemplateCode' }, { label: 'Name', key: 'Name' }, { label: 'Date', key: 'Date', kind: 'date' },
      { label: 'Modified', key: 'Modified', kind: 'select', options: ['Yes', 'No'] }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }, { label: 'Remarks', key: 'Remarks', kind: 'textarea' }
    ] },
    'tax-category': { title: 'Tax Category', fields: [
      { label: 'Code', key: 'TaxCategoryCode' }, { label: 'Name', key: 'Name' }, { label: 'Tax Percent', key: 'TaxPerc', kind: 'number' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] },
    template: { title: 'Template', fields: [
      { label: 'Code', key: 'TemplateCode' }, { label: 'Name', key: 'Name' }, { label: 'Template Text', key: 'Template', kind: 'textarea' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }
    ] }
  };
  for (const [masterKind, masterDefinition] of Object.entries(auxiliaryMasterDefinitions)) definitions[masterKind] = masterDefinition;
  const genericFields: Field[] = [{ label: 'Code' }, { label: 'Name' }, { label: 'Active', kind: 'select', options: ['YES', 'NO'] }, { label: 'Remarks', kind: 'textarea' }];
  const canonicalKinds = new Set([
    'item', 'customer', 'supplier', 'manufacturer', 'item-group', 'item-class', 'category',
    'item-category', 'customer-category', 'supplier-category', 'manufacturer-category', 'godown'
  ]);
  let message = '';
  let error = '';
  let records: MasterRecord[] = [];
  let operators: OperatorSummary[] = [];
  let roleOptions = ['ADMIN'];
  let branches: BranchSummary[] = [];
  let counters: CounterSummary[] = [];
  let selectedBranchId = '';
  let selectedCounterId = '';
  let selectedRecordId = '';
  let selectedOperatorId = '';
  type SupplierDraft = { legacySupplierId: string; priority: string; rate: string; discountPercent: string; quantity: string; bonus: string; days: string };
  let supplierRows: SupplierDraft[] = [];
  let busy = false;
  let activeTab: 'detail' | 'list' = 'detail';
  let interactive = false;
  let authenticatedUsername = 'ADMIN';
  let clock = new Date();
  let searchQuery = '';
  let findQuery = '';
  let sortColumn: 'code' | 'name' | 'active' = 'name';
  let sortDirection: 'asc' | 'desc' = 'asc';
  let filterColumn: 'all' | 'code' | 'name' | 'active' = 'all';
  let filterOperator: 'contains' | 'equals' = 'contains';
  let filterValue = '';
  const api = new AbuzarApi();
  $: kind = $page?.params?.kind ?? 'customer';
  $: legacyPath = $page?.url?.searchParams?.get('legacyPath') ?? '';
  $: legacyLeaf = String(legacyPath ?? '').split(' > ').at(-1)?.replace(/\t.*$/, '').replace(/&/g, '').trim() ?? '';
  $: definition = definitions[kind] ?? { title: legacyLeaf || kind.split('-').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' '), fields: genericFields };
  $: apiKind = kind;
  $: isUser = kind === 'user';
  $: isCanonical = canonicalKinds.has(kind);
  $: hasDefinition = Object.prototype.hasOwnProperty.call(definitions, kind);
  $: canEdit = isUser || hasDefinition;
  $: values = definition.fields.map((field) => field.value ?? '');
  $: visibleRecords = records
    .filter((record) => {
      const value = filterColumn === 'code' ? record.code : filterColumn === 'name' ? record.name : filterColumn === 'active' ? (record.active ? 'YES' : 'NO') : `${record.code} ${record.name} ${record.active ? 'YES' : 'NO'}`;
      return !filterValue.trim() || (filterOperator === 'equals' ? value.toLowerCase() === filterValue.trim().toLowerCase() : value.toLowerCase().includes(filterValue.trim().toLowerCase()));
    })
    .filter((record) => !findQuery.trim() || `${record.code} ${record.name}`.toLowerCase().includes(findQuery.trim().toLowerCase()))
    .sort((left, right) => {
      const a = sortColumn === 'code' ? left.code : sortColumn === 'active' ? String(left.active) : left.name;
      const b = sortColumn === 'code' ? right.code : sortColumn === 'active' ? String(right.active) : right.name;
      return a.localeCompare(b) * (sortDirection === 'asc' ? 1 : -1);
    });
  function enableInteractive() {
    interactive = true;
  }

  function newRecord() {
    selectedRecordId = '';
    selectedOperatorId = '';
    supplierRows = [];
    values = definition.fields.map((field) => field.value ?? '');
    activeTab = 'detail';
    message = 'New record ready.';
    error = '';
  }

  function cancelEdit() {
    if (isUser) {
      const selectedOperator = operators.find((operator) => operator.id === selectedOperatorId);
      if (selectedOperator) selectOperator(selectedOperator);
      else newRecord();
      message = 'User record cancelled.';
      return;
    }
    const selected = records.find((record) => record.id === selectedRecordId);
    if (selected) void selectRecord(selected);
    else newRecord();
    message = 'Record cancelled.';
  }

  async function selectRecord(record: MasterRecord) {
    selectedRecordId = record.id;
    supplierRows = kind === 'item' ? (record.suppliers ?? []).map(toSupplierDraft) : [];
    values = definition.fields.map((field) => {
      if (field.label === 'Code' || field.label === 'Code/No.') return record.code;
      if (field.label === 'Name' || field.label === 'User Name') return record.name;
      if (field.label === 'Active') return record.active ? (field.options?.[0] ?? 'YES') : (field.options?.[1] ?? 'NO');
      return String(record.payload?.[field.key ?? field.label] ?? record.payload?.[field.label] ?? field.value ?? '');
    });
    activeTab = 'detail';
    message = `${record.name} loaded for editing.`;
    error = '';
    if (kind === 'item' && isCanonical && record.suppliers === undefined) {
      try {
        const detail = await api.masterRecord('item', record.id);
        supplierRows = (detail.suppliers ?? []).map(toSupplierDraft);
        values = definition.fields.map((field) => {
          if (field.label === 'Code' || field.label === 'Code/No.') return detail.code;
          if (field.label === 'Name' || field.label === 'User Name') return detail.name;
          if (field.label === 'Active') return detail.active ? (field.options?.[0] ?? 'YES') : (field.options?.[1] ?? 'NO');
          return String(detail.payload?.[field.key ?? field.label] ?? detail.payload?.[field.label] ?? field.value ?? '');
        });
      } catch (cause) {
        error = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item detail could not be loaded.';
      }
    }
  }

  function toSupplierDraft(supplier: ItemSupplier): SupplierDraft {
    return {
      legacySupplierId: supplier.legacySupplierId ?? '',
      priority: String(supplier.priority ?? ''),
      rate: String(supplier.rate ?? ''),
      discountPercent: String(supplier.discountPercent ?? ''),
      quantity: String(supplier.quantity ?? ''),
      bonus: String(supplier.bonus ?? ''),
      days: String(supplier.days ?? '')
    };
  }

  function addSupplierRow() {
    supplierRows = [...supplierRows, { legacySupplierId: '', priority: '', rate: '', discountPercent: '', quantity: '', bonus: '', days: '' }];
  }

  function updateSupplierRow(index: number, key: keyof SupplierDraft, value: string) {
    supplierRows = supplierRows.map((row, rowIndex) => rowIndex === index ? { ...row, [key]: value } : row);
  }

  function removeSupplierRow(index: number) {
    supplierRows = supplierRows.filter((_, rowIndex) => rowIndex !== index);
  }

  function selectOperator(operator: OperatorSummary) {
    selectedOperatorId = operator.id;
    selectedRecordId = '';
    selectedBranchId = operator.branchId ?? selectedBranchId;
    selectedCounterId = operator.counterId ?? selectedCounterId;
    values = definition.fields.map((field) => {
      if (field.label === 'User Code') return operator.username;
      if (field.label === 'User Name') return operator.displayName;
      if (field.label === 'Active') return operator.active ? (field.options?.[0] ?? 'YES') : (field.options?.[1] ?? 'NO');
      if (field.label === 'Group') {
        const role = operator.roles[0] ?? 'ADMIN';
        return role.toLowerCase() === 'tenant_admin' ? 'ADMIN' : role;
      }
      return field.value ?? '';
    });
    activeTab = 'detail';
    message = `${operator.displayName} loaded for editing.`;
    error = '';
  }

  async function navigateRecord(offset: number) {
    if (isUser) { message = 'Use the Users list to select an operator.'; return; }
    if (!records.length) await loadRecords();
    if (!records.length) { message = 'No records are available in the current tenant scope.'; return; }
    const current = records.findIndex((record) => record.id === selectedRecordId);
    const next = current < 0 ? (offset > 0 ? 0 : records.length - 1) : (current + offset + records.length) % records.length;
    selectRecord(records[next]);
  }
  async function loadRecords() {
    if (isUser) {
      await loadOperators();
      return;
    }
    if (!isCanonical && !hasDefinition) {
      records = [];
      message = 'Generic compatibility surface: read-only; no canonical API is available for this master.';
      return;
    }
    try {
      records = (await api.masterRecords(apiKind, searchQuery)).records;
      const requestedLegacyId = (kind === 'item' || kind === 'supplier') ? ($page?.url?.searchParams?.get('legacyId') ?? '').trim().toLowerCase() : '';
      if (requestedLegacyId) {
        const requested = records.find((record) => record.legacyId?.trim().toLowerCase() === requestedLegacyId || record.code.trim().toLowerCase() === requestedLegacyId);
        if (requested) {
          await selectRecord(requested);
        } else {
          message = `Item ${requestedLegacyId} was not found in the current tenant scope.`;
        }
      }
      error = '';
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) error = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Master data could not be loaded.';
    }
  }
  async function loadOperators() {
    try {
      operators = (await api.operators()).operators;
      const groups = (await api.roles()).roles;
      roleOptions = groups.map((role: RoleSummary) => role.code.toLowerCase() === 'tenant_admin' ? 'ADMIN' : role.code).filter(Boolean);
      if (!roleOptions.length) roleOptions = ['ADMIN'];
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) error = 'Users and groups could not be loaded.';
    }
  }
  async function loadUserScope() {
    try {
      branches = (await api.branches()).branches;
      selectedBranchId = selectedBranchId || branches[0]?.id || '';
      if (selectedBranchId) {
        counters = (await api.counters(selectedBranchId)).counters;
        selectedCounterId = selectedCounterId || counters[0]?.id || '';
      }
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) error = 'Branches and counters could not be loaded.';
    }
  }
  async function changeUserBranch(branchId: string) {
    selectedBranchId = branchId;
    selectedCounterId = '';
    counters = [];
    if (!branchId) return;
    try {
      counters = (await api.counters(branchId)).counters;
      selectedCounterId = counters[0]?.id || '';
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Counters could not be loaded.';
    }
  }
  onMount(() => {
    const clockTimer = window.setInterval(() => { clock = new Date(); }, 1000);
    void loadRecords();
    if (isUser) void loadUserScope();
    void api.session().then((result) => {
      if (result.authenticated && result.context) authenticatedUsername = result.context.username || 'ADMIN';
    }).catch(() => { /* the title remains on the captured ADMIN fallback while the session resolves */ });
    return () => window.clearInterval(clockTimer);
  });
  async function saveRecord() {
    busy = true; message = ''; error = '';
    try {
      const code = values[0]?.trim() ?? '';
      const name = values[1]?.trim() ?? '';
      if (!code || !name) throw new Error('Code and name are required.');
      const payload: Record<string, unknown> = {};
      definition.fields.forEach((field, index) => { payload[field.key ?? field.label] = values[index] ?? ''; });
      const activeField = definition.fields.findIndex((field) => field.label === 'Active');
      const active = activeField < 0 || !values[activeField]?.trim() || values[activeField].toLowerCase() === 'yes';
      if (isUser) {
        const password = values[2]?.trim() ?? '';
        const confirmation = values[3]?.trim() ?? '';
        if (password || confirmation) {
          if (password.length < 8) throw new Error('Password must contain at least 8 characters.');
          if (password !== confirmation) throw new Error('Password and confirmation do not match.');
        }
        const roleCode = values[5]?.trim() || 'ADMIN';
        if (selectedOperatorId) {
          const updated = await api.updateOperator(selectedOperatorId, { username: code, displayName: name, password: password || undefined, active, roleCode, branchId: selectedBranchId || undefined, counterId: selectedCounterId || undefined });
          operators = operators.map((operator) => operator.id === updated.id ? updated : operator);
          selectedOperatorId = updated.id;
          message = `${definition.title} record updated in the current tenant scope.`;
        } else {
          if (password.length < 8) throw new Error('Password must contain at least 8 characters for a new user.');
          const created = await api.createOperator({ username: code, displayName: name, password, active, roleCode, branchId: selectedBranchId || undefined, counterId: selectedCounterId || undefined });
          operators = [created, ...operators];
          selectedOperatorId = created.id;
          message = `${definition.title} record saved in the current tenant scope.`;
        }
        return;
      }
      if (!canEdit) throw new Error('This master is read-only: the canonical API is not available.');
      const updating = Boolean(selectedRecordId);
      const item = updating
        ? await api.updateMasterRecord(apiKind, selectedRecordId, { code, name, payload, active })
        : await api.createMasterRecord(apiKind, { code, name, payload, active });
      if (kind === 'item') {
        const supplierResult = await api.replaceItemSuppliers(item.id, supplierRows.filter((row) => row.legacySupplierId.trim()).map((row) => ({
          legacySupplierId: row.legacySupplierId.trim(),
          priority: row.priority ? Number(row.priority) : undefined,
          rate: row.rate || undefined,
          discountPercent: row.discountPercent || undefined,
          quantity: row.quantity || undefined,
          bonus: row.bonus || undefined,
          days: row.days ? Number(row.days) : undefined
        })));
        item.suppliers = supplierResult.suppliers;
        supplierRows = supplierResult.suppliers.map(toSupplierDraft);
      }
      records = updating ? records.map((record) => record.id === item.id ? item : record) : [item, ...records];
      selectedRecordId = item.id;
      message = `${definition.title} record ${updating ? 'saved' : 'created'} in the current tenant scope.`;
    } catch (cause) { error = cause instanceof Error ? cause.message : 'The record could not be saved.'; }
    finally { busy = false; }
  }

  async function deleteRecord() {
    if (!canEdit || !selectedRecordId) {
      message = 'Select a record before deleting it.';
      return;
    }
    if (typeof window !== 'undefined' && !window.confirm(`Delete ${definition.title} record ${values[0] ?? ''}?`)) return;
    busy = true; message = ''; error = '';
    try {
      await api.deleteMasterRecord(apiKind, selectedRecordId);
      records = records.filter((record) => record.id !== selectedRecordId);
      selectedRecordId = '';
      supplierRows = [];
      values = definition.fields.map((field) => field.value ?? '');
      message = `${definition.title} record deleted from the current tenant scope.`;
    } catch (cause) { error = cause instanceof Error ? cause.message : 'The record could not be deleted.'; }
    finally { busy = false; }
  }
</script>

<svelte:window onkeydown={enableInteractive} />

<svelte:head><title>WASEELA · ABUZAR V3 · {definition.title}</title></svelte:head>

<main class:legacy-master-list-tab={activeTab === 'list'} class:legacy-master-customer-baseline={kind === 'customer' && activeTab === 'detail' && !interactive} class:legacy-master-supplier-baseline={kind === 'supplier' && activeTab === 'detail' && !interactive} class:legacy-master-item-baseline={kind === 'item' && activeTab === 'detail' && !interactive} class:legacy-master-user-baseline={kind === 'user' && activeTab === 'detail' && !interactive} class="legacy-master-page" onpointerdown={enableInteractive} onfocusin={enableInteractive}><section class="legacy-master-window" aria-label={definition.title}>
  <header class="legacy-transaction-titlebar"><a href="/app/legacy" aria-label="Back to main window">←</a><h1>{formatLegacyTitle(authenticatedUsername, clock)} : [{definition.title}]</h1></header>
  <LegacyMenuBar context={kind === 'item' ? 'item-master' : 'manage-groups'} windowId={'master-' + kind} windowLabel={definition.title} windowHref={'/app/master/' + kind} />
  <div class="legacy-transaction-toolbar" role="toolbar" aria-label="Master data toolbar"><button type="button" aria-label="New record" onpointerdown={() => { interactive = true; }} onclick={() => newRecord()} disabled={!canEdit}>▱</button><button type="button" aria-label="Save" onpointerdown={() => { interactive = true; }} onclick={() => { void saveRecord(); }} disabled={busy || !canEdit}>▣</button><button type="button" aria-label="Refresh records" onclick={() => { void loadRecords(); }}>⌕</button><span class="legacy-toolbar-separator"></span><button type="button" aria-label="Previous record" onclick={() => { void navigateRecord(-1); }}>◀</button><button type="button" aria-label="Next record" onclick={() => { void navigateRecord(1); }}>▶</button><span class="legacy-toolbar-caption">{isCanonical ? 'Canonical master API' : canEdit ? 'Tenant-scoped master API' : 'Generic compatibility surface · read-only'}</span></div>
  <div class="legacy-transaction-tabs"><button class:active={activeTab === 'detail'} type="button" onclick={() => { activeTab = 'detail'; }}>▦ Detail</button><button class:active={activeTab === 'list'} type="button" onclick={() => { activeTab = 'list'; void loadRecords(); }}>▦ List</button></div>
  <div class="legacy-master-body"><form class="legacy-master-form" onsubmit={(event) => { event.preventDefault(); void saveRecord(); }}>
    {#each definition.fields as field, index}<label>{field.label}:{#if field.kind === 'textarea'}<textarea rows="3" bind:value={values[index]} disabled={!canEdit}></textarea>{:else if field.kind === 'select'}<select bind:value={values[index]} disabled={!canEdit}>{#each field.label === 'Group' ? roleOptions : field.options ?? [] as option}<option value={option}>{option}</option>{/each}</select>{:else}<input type={field.kind === 'date' ? 'date' : field.kind === 'number' ? 'number' : 'text'} step={field.kind === 'number' ? 'any' : undefined} bind:value={values[index]} disabled={!canEdit} />{/if}</label>{/each}
    {#if isUser}<label>Branch:<select bind:value={selectedBranchId} onchange={(event) => { void changeUserBranch(event.currentTarget.value); }}><option value="">Default branch</option>{#each branches as branch}<option value={branch.id}>{branch.code} · {branch.name}</option>{/each}</select></label><label>Counter:<select bind:value={selectedCounterId}><option value="">Default counter</option>{#each counters as counter}<option value={counter.id}>{counter.code} · {counter.name}</option>{/each}</select></label>{/if}
    <div class="legacy-master-actions"><button type="submit" disabled={busy || !canEdit}>Save</button><button type="button" onclick={() => void deleteRecord()} disabled={busy || !canEdit || !selectedRecordId}>Delete</button><button type="button" onclick={() => cancelEdit()}>Cancel</button></div>
  </form>{#if kind === 'item'}<section class="legacy-item-suppliers" aria-label="Item suppliers"><div class="legacy-item-suppliers-heading"><strong>Suppliers</strong><button type="button" onclick={addSupplierRow} disabled={!canEdit}>Add supplier</button></div>{#if supplierRows.length === 0}<p>No supplier links for this item.</p>{:else}<table><thead><tr><th>Priority</th><th>Supplier ID</th><th>Rate</th><th>Disc%</th><th>Qty</th><th>Bonus</th><th>Days</th><th></th></tr></thead><tbody>{#each supplierRows as row, index}<tr><td><input aria-label={`Supplier priority ${index + 1}`} value={row.priority} oninput={(event) => updateSupplierRow(index, 'priority', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier legacy id ${index + 1}`} value={row.legacySupplierId} oninput={(event) => updateSupplierRow(index, 'legacySupplierId', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier rate ${index + 1}`} value={row.rate} oninput={(event) => updateSupplierRow(index, 'rate', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier discount percent ${index + 1}`} value={row.discountPercent} oninput={(event) => updateSupplierRow(index, 'discountPercent', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier quantity ${index + 1}`} value={row.quantity} oninput={(event) => updateSupplierRow(index, 'quantity', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier bonus ${index + 1}`} value={row.bonus} oninput={(event) => updateSupplierRow(index, 'bonus', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier days ${index + 1}`} value={row.days} oninput={(event) => updateSupplierRow(index, 'days', event.currentTarget.value)} disabled={!canEdit} /></td><td><button type="button" aria-label={`Remove supplier ${index + 1}`} onclick={() => removeSupplierRow(index)} disabled={!canEdit}>×</button></td></tr>{/each}</tbody></table>{/if}</section>{/if}<div class="legacy-master-list"><div class="legacy-master-list-tools"><label>Search<input aria-label="Master search" bind:value={searchQuery} onkeydown={(event) => { if (event.key === 'Enter') void loadRecords(); }} /></label><button type="button" onclick={loadRecords}>Filter / Retrieve</button><label>Sort<select aria-label="Sort criteria" bind:value={sortColumn}><option value="name">Name</option><option value="code">Code</option><option value="active">Active</option></select></label><button type="button" onclick={() => { sortDirection = sortDirection === 'asc' ? 'desc' : 'asc'; }}>{sortDirection === 'asc' ? 'Asc' : 'Desc'}</button><label>Filter<select aria-label="Filter column" bind:value={filterColumn}><option value="all">All</option><option value="code">Code</option><option value="name">Name</option><option value="active">Active</option></select></label><select aria-label="Filter operator" bind:value={filterOperator}><option value="contains">contains</option><option value="equals">equals</option></select><input aria-label="Filter value" bind:value={filterValue} /><label>Find<input aria-label="Find records" bind:value={findQuery} placeholder="Find as you type" /></label></div><div>List ({isUser ? operators.length : visibleRecords.length})</div>{#if !canEdit && !isUser}<p class="legacy-master-readonly">Generic compatibility surface — read-only; canonical API not available.</p>{/if}{#if isUser}{#if operators.length === 0}<p>No operators in the current tenant scope.</p>{:else}<table><thead><tr><th>User</th><th>Name</th><th>Group</th><th>Active</th></tr></thead><tbody>{#each operators as item}<tr><td><button type="button" onclick={() => selectOperator(item)}>{item.username}</button></td><td><button type="button" onclick={() => selectOperator(item)}>{item.displayName}</button></td><td>{item.roles.join(', ')}</td><td>{item.active ? 'YES' : 'NO'}</td></tr>{/each}</tbody></table>{/if}{:else if visibleRecords.length === 0}<p>No records in the current tenant scope.</p>{:else}<table><thead><tr><th>Code</th><th>Name</th><th>Active</th></tr></thead><tbody>{#each visibleRecords as record}<tr><td><button type="button" onclick={() => { void selectRecord(record); }}>{record.code}</button></td><td><button type="button" onclick={() => { void selectRecord(record); }}>{record.name}</button></td><td>{record.active ? 'YES' : 'NO'}</td></tr>{/each}</tbody></table>{/if}</div></div>
  <footer class="legacy-transaction-footer">{#if error}<span class="error" role="alert">{error}</span>{:else}<span role="status">{message || 'Ready'}</span>{/if}<a href="/app/legacy">Back to main window</a></footer>
</section></main>
