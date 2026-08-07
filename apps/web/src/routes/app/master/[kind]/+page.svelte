<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import type { BranchSummary, CounterSummary, ItemAssociation, ItemAuthor, ItemImage, ItemLookupResult, ItemModel, ItemPricePolicyResponse, ItemPricePolicyTier, ItemRegistrationRequest, ItemSupplier, ItemUnpostedTransaction, MasterRecord, OperatorSummary, RoleSummary } from '@abuzar/contracts';
  import { AbuzarApi, ApiError } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';
  import type { MenuAction } from '$lib/legacy-menu';
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
  let alternateAliases: string[] = [];
  let alternateAliasDialogOpen = false;
  let alternateAliasBusy = false;
  let alternateAliasError = '';
  let itemImages: ItemImage[] = [];
  let itemImageDialogOpen = false;
  let itemImageBusy = false;
  let itemImageError = '';
  let itemNotesData = '';
  let itemNotesText = '';
  let itemNotesDialogOpen = false;
  let itemNotesBusy = false;
  let itemNotesError = '';
  let itemNotesTextEdited = false;
  let itemNotesRawEdited = false;
  let itemAssociations: ItemAssociation[] = [];
  let itemAssociationsDialogOpen = false;
  let itemAssociationsBusy = false;
  let itemAssociationsError = '';
  let itemAuthors: ItemAuthor[] = [];
  let itemAuthorsDialogOpen = false;
  let itemAuthorsBusy = false;
  let itemAuthorsError = '';
  let itemModels: ItemModel[] = [];
  let itemModelsDialogOpen = false;
  let itemModelsBusy = false;
  let itemModelsError = '';
  let itemPricePolicy: ItemPricePolicyResponse = { policy: null, tiers: [] };
  let itemPricePolicyDialogOpen = false;
  let itemPricePolicyBusy = false;
  let itemPricePolicyError = '';
  let itemRegistrationRequest: ItemRegistrationRequest | null = null;
  let itemRegistrationRequestDialogOpen = false;
  let itemRegistrationRequestBusy = false;
  let itemRegistrationRequestError = '';
  let populateItemDialogOpen = false;
  let populateItemBusy = false;
  let populateItemError = '';
  let populateItemQuery = '';
  let populateItemResults: ItemLookupResult[] = [];
  let itemUnpostedTransactions: ItemUnpostedTransaction[] = [];
  let itemUnpostedTransactionsDialogOpen = false;
  let itemUnpostedTransactionsBusy = false;
  let itemUnpostedTransactionsError = '';
  let itemUnpostedTransactionsTruncated = false;
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
    alternateAliases = [];
    alternateAliasDialogOpen = false;
    alternateAliasError = '';
    itemImages = [];
    itemImageDialogOpen = false;
    itemImageError = '';
    itemNotesData = '';
    itemNotesText = '';
    itemNotesDialogOpen = false;
    itemNotesError = '';
    itemNotesTextEdited = false;
    itemNotesRawEdited = false;
    itemAssociations = [];
    itemAssociationsDialogOpen = false;
    itemAssociationsError = '';
    itemAuthors = [];
    itemAuthorsDialogOpen = false;
    itemAuthorsError = '';
    itemModels = [];
    itemModelsDialogOpen = false;
    itemModelsError = '';
    itemPricePolicy = { policy: null, tiers: [] };
    itemPricePolicyDialogOpen = false;
    itemPricePolicyError = '';
    itemRegistrationRequest = null;
    itemRegistrationRequestDialogOpen = false;
    itemRegistrationRequestError = '';
    populateItemDialogOpen = false;
    populateItemError = '';
    populateItemQuery = '';
    populateItemResults = [];
    itemUnpostedTransactions = [];
    itemUnpostedTransactionsDialogOpen = false;
    itemUnpostedTransactionsError = '';
    itemUnpostedTransactionsTruncated = false;
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
    alternateAliasDialogOpen = false;
    alternateAliasError = '';
    itemImageDialogOpen = false;
    itemImageError = '';
    itemNotesDialogOpen = false;
    itemNotesError = '';
    itemAssociationsDialogOpen = false;
    itemAssociationsError = '';
    itemAuthorsDialogOpen = false;
    itemAuthorsError = '';
    itemModelsDialogOpen = false;
    itemModelsError = '';
    itemPricePolicyDialogOpen = false;
    itemPricePolicyError = '';
    itemRegistrationRequestDialogOpen = false;
    itemRegistrationRequestError = '';
    populateItemDialogOpen = false;
    populateItemError = '';
    itemUnpostedTransactionsDialogOpen = false;
    itemUnpostedTransactionsError = '';
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

  async function openAlternateAliasDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before editing alternate aliases.';
      return;
    }
    alternateAliasDialogOpen = true;
    alternateAliasBusy = true;
    alternateAliasError = '';
    try {
      const result = await api.itemAliases(selectedRecordId);
      alternateAliases = result.aliases ?? [];
    } catch (cause) {
      alternateAliasError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Alternate aliases could not be loaded.';
    } finally {
      alternateAliasBusy = false;
    }
  }

  function closeAlternateAliasDialog() {
    if (alternateAliasBusy) return;
    alternateAliasDialogOpen = false;
    alternateAliasError = '';
  }

  function addAlternateAlias() {
    if (alternateAliases.length >= 100) {
      alternateAliasError = 'An item may have at most 100 alternate aliases.';
      return;
    }
    alternateAliases = [...alternateAliases, ''];
    alternateAliasError = '';
  }

  function updateAlternateAlias(index: number, value: string) {
    alternateAliases = alternateAliases.map((alias, aliasIndex) => aliasIndex === index ? value : alias);
    alternateAliasError = '';
  }

  function removeAlternateAlias(index: number) {
    alternateAliases = alternateAliases.filter((_, aliasIndex) => aliasIndex !== index);
    alternateAliasError = '';
  }

  async function saveAlternateAliases() {
    if (!selectedRecordId) return;
    const aliases = alternateAliases.map((alias) => alias.trim());
    if (aliases.some((alias) => !alias)) {
      alternateAliasError = 'Alternate aliases cannot be blank.';
      return;
    }
    const normalized = aliases.map((alias) => alias.toLowerCase());
    if (new Set(normalized).size !== normalized.length) {
      alternateAliasError = 'Alternate aliases must be unique.';
      return;
    }
    alternateAliasBusy = true;
    alternateAliasError = '';
    try {
      const result = await api.replaceItemAliases(selectedRecordId, aliases);
      alternateAliases = result.aliases ?? aliases;
      alternateAliasDialogOpen = false;
      message = `${alternateAliases.length} alternate item alias${alternateAliases.length === 1 ? '' : 'es'} saved in the current tenant scope.`;
    } catch (cause) {
      alternateAliasError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Alternate aliases could not be saved.';
    } finally {
      alternateAliasBusy = false;
    }
  }

  function readImageFile(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        const value = String(reader.result ?? '');
        const separator = value.indexOf(',');
        resolve(separator >= 0 ? value.slice(separator + 1) : value);
      };
      reader.onerror = () => reject(reader.error ?? new Error('The image file could not be read.'));
      reader.readAsDataURL(file);
    });
  }

  async function openItemImageDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before editing item images.';
      return;
    }
    itemImageDialogOpen = true;
    itemImageBusy = true;
    itemImageError = '';
    try {
      const result = await api.itemImages(selectedRecordId);
      itemImages = result.images ?? [];
    } catch (cause) {
      itemImageError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item images could not be loaded.';
    } finally {
      itemImageBusy = false;
    }
  }

  function closeItemImageDialog() {
    if (itemImageBusy) return;
    itemImageDialogOpen = false;
    itemImageError = '';
  }

  async function addItemImageFile(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    if (itemImages.length >= 50) {
      itemImageError = 'An item may have at most 50 images.';
      return;
    }
    itemImageBusy = true;
    itemImageError = '';
    try {
      const imageData = await readImageFile(file);
      itemImages = [...itemImages, {
        rowId: itemImages.length + 1,
        imageDescription: file.name,
        imageData,
        imageType: file.type || 'application/octet-stream'
      }];
    } catch (cause) {
      itemImageError = cause instanceof Error ? cause.message : 'The image file could not be read.';
    } finally {
      itemImageBusy = false;
    }
  }

  function updateItemImage(index: number, key: 'imageDescription' | 'imageType', value: string) {
    itemImages = itemImages.map((image, imageIndex) => imageIndex === index ? { ...image, [key]: value } : image);
    itemImageError = '';
  }

  function removeItemImage(index: number) {
    itemImages = itemImages.filter((_, imageIndex) => imageIndex !== index).map((image, imageIndex) => ({ ...image, rowId: imageIndex + 1 }));
    itemImageError = '';
  }

  async function saveItemImages() {
    if (!selectedRecordId) return;
    if (itemImages.some((image) => !image.imageData)) {
      itemImageError = 'Each item image must contain image data.';
      return;
    }
    itemImageBusy = true;
    itemImageError = '';
    try {
      const result = await api.replaceItemImages(selectedRecordId, itemImages.map((image, index) => ({ ...image, rowId: index + 1 })));
      itemImages = result.images ?? itemImages;
      itemImageDialogOpen = false;
      message = `${itemImages.length} item image${itemImages.length === 1 ? '' : 's'} saved in the current tenant scope.`;
    } catch (cause) {
      itemImageError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item images could not be saved.';
    } finally {
      itemImageBusy = false;
    }
  }

  function decodeItemNotesText(value: string): string | undefined {
    if (!value) return '';
    try {
      const binary = atob(value);
      const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
      return new TextDecoder('utf-8', { fatal: true }).decode(bytes);
    } catch {
      return undefined;
    }
  }

  function encodeItemNotesText(value: string): string {
    const bytes = new TextEncoder().encode(value);
    let binary = '';
    for (const byte of bytes) binary += String.fromCharCode(byte);
    return btoa(binary);
  }

  async function openItemNotesDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before editing item notes.';
      return;
    }
    itemNotesDialogOpen = true;
    itemNotesBusy = true;
    itemNotesError = '';
    itemNotesTextEdited = false;
    itemNotesRawEdited = false;
    try {
      const result = await api.itemNotes(selectedRecordId);
      itemNotesData = result.notesData ?? '';
      itemNotesText = decodeItemNotesText(itemNotesData) ?? '';
    } catch (cause) {
      itemNotesError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item notes could not be loaded.';
    } finally {
      itemNotesBusy = false;
    }
  }

  function closeItemNotesDialog() {
    if (itemNotesBusy) return;
    itemNotesDialogOpen = false;
    itemNotesError = '';
  }

  function updateItemNotesText(value: string) {
    itemNotesText = value;
    itemNotesTextEdited = true;
    itemNotesRawEdited = false;
    itemNotesError = '';
  }

  function updateItemNotesRaw(value: string) {
    itemNotesData = value.trim();
    itemNotesRawEdited = true;
    itemNotesTextEdited = false;
    itemNotesError = '';
  }

  async function saveItemNotes() {
    if (!selectedRecordId) return;
    const notesData = itemNotesTextEdited && !itemNotesRawEdited ? encodeItemNotesText(itemNotesText) : itemNotesData.trim();
    itemNotesBusy = true;
    itemNotesError = '';
    try {
      const result = await api.replaceItemNotes(selectedRecordId, notesData);
      itemNotesData = result.notesData ?? notesData;
      itemNotesText = decodeItemNotesText(itemNotesData) ?? '';
      itemNotesTextEdited = false;
      itemNotesRawEdited = false;
      itemNotesDialogOpen = false;
      message = 'Item notes saved in the current tenant scope.';
    } catch (cause) {
      itemNotesError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item notes could not be saved.';
    } finally {
      itemNotesBusy = false;
    }
  }

  async function openItemAssociationsDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before editing item associations.';
      return;
    }
    itemAssociationsDialogOpen = true;
    itemAssociationsBusy = true;
    itemAssociationsError = '';
    try {
      const result = await api.itemAssociations(selectedRecordId);
      itemAssociations = result.associations ?? [];
    } catch (cause) {
      itemAssociationsError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item associations could not be loaded.';
    } finally {
      itemAssociationsBusy = false;
    }
  }

  function closeItemAssociationsDialog() {
    if (itemAssociationsBusy) return;
    itemAssociationsDialogOpen = false;
    itemAssociationsError = '';
  }

  function addItemAssociation() {
    if (itemAssociations.length >= 100) {
      itemAssociationsError = 'An item may have at most 100 associations.';
      return;
    }
    itemAssociations = [...itemAssociations, { legacyItemId: '' }];
    itemAssociationsError = '';
  }

  function updateItemAssociation(index: number, value: string) {
    itemAssociations = itemAssociations.map((association, associationIndex) => associationIndex === index
      ? { legacyItemId: value, code: '', name: '' }
      : association);
    itemAssociationsError = '';
  }

  function removeItemAssociation(index: number) {
    itemAssociations = itemAssociations.filter((_, associationIndex) => associationIndex !== index);
    itemAssociationsError = '';
  }

  async function saveItemAssociations() {
    if (!selectedRecordId) return;
    const legacyItemIds = itemAssociations.map((association) => association.legacyItemId.trim());
    if (legacyItemIds.some((legacyItemId) => !legacyItemId)) {
      itemAssociationsError = 'Associated item identifiers cannot be blank.';
      return;
    }
    if (new Set(legacyItemIds.map((legacyItemId) => legacyItemId.toLowerCase())).size !== legacyItemIds.length) {
      itemAssociationsError = 'Associated item identifiers must be unique.';
      return;
    }
    itemAssociationsBusy = true;
    itemAssociationsError = '';
    try {
      const result = await api.replaceItemAssociations(selectedRecordId, legacyItemIds);
      itemAssociations = result.associations ?? itemAssociations;
      itemAssociationsDialogOpen = false;
      message = `${itemAssociations.length} item association${itemAssociations.length === 1 ? '' : 's'} saved in the current tenant scope.`;
    } catch (cause) {
      itemAssociationsError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item associations could not be saved.';
    } finally {
      itemAssociationsBusy = false;
    }
  }

  async function openItemAuthorsDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before editing item authors.';
      return;
    }
    itemAuthorsDialogOpen = true;
    itemAuthorsBusy = true;
    itemAuthorsError = '';
    try {
      const result = await api.itemAuthors(selectedRecordId);
      itemAuthors = result.authors ?? [];
    } catch (cause) {
      itemAuthorsError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item authors could not be loaded.';
    } finally {
      itemAuthorsBusy = false;
    }
  }

  function closeItemAuthorsDialog() {
    if (itemAuthorsBusy) return;
    itemAuthorsDialogOpen = false;
    itemAuthorsError = '';
  }

  function addItemAuthor() {
    if (itemAuthors.length >= 50) {
      itemAuthorsError = 'An item may have at most 50 authors.';
      return;
    }
    itemAuthors = [...itemAuthors, { authorCode: 0, priority: 0, rowId: itemAuthors.length + 1 }];
    itemAuthorsError = '';
  }

  function updateItemAuthor(index: number, key: 'authorCode' | 'priority', value: string) {
    const parsed = Number(value);
    itemAuthors = itemAuthors.map((author, authorIndex) => authorIndex === index ? { ...author, [key]: Number.isFinite(parsed) ? parsed : 0 } : author);
    itemAuthorsError = '';
  }

  function removeItemAuthor(index: number) {
    itemAuthors = itemAuthors.filter((_, authorIndex) => authorIndex !== index).map((author, authorIndex) => ({ ...author, rowId: authorIndex + 1 }));
    itemAuthorsError = '';
  }

  async function saveItemAuthors() {
    if (!selectedRecordId) return;
    if (itemAuthors.some((author) => author.authorCode <= 0)) {
      itemAuthorsError = 'Author codes must be positive.';
      return;
    }
    if (itemAuthors.some((author) => author.priority < 0 || author.priority > 255)) {
      itemAuthorsError = 'Author priorities must be between 0 and 255.';
      return;
    }
    if (new Set(itemAuthors.map((author) => author.authorCode)).size !== itemAuthors.length) {
      itemAuthorsError = 'Author codes must be unique.';
      return;
    }
    itemAuthorsBusy = true;
    itemAuthorsError = '';
    try {
      const result = await api.replaceItemAuthors(selectedRecordId, itemAuthors.map((author, index) => ({ ...author, rowId: index + 1 })));
      itemAuthors = result.authors ?? itemAuthors;
      itemAuthorsDialogOpen = false;
      message = `${itemAuthors.length} item author${itemAuthors.length === 1 ? '' : 's'} saved in the current tenant scope.`;
    } catch (cause) {
      itemAuthorsError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item authors could not be saved.';
    } finally {
      itemAuthorsBusy = false;
    }
  }

  async function openItemModelsDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before editing item models.';
      return;
    }
    itemModelsDialogOpen = true;
    itemModelsBusy = true;
    itemModelsError = '';
    try {
      const result = await api.itemModels(selectedRecordId);
      itemModels = result.models ?? [];
    } catch (cause) {
      itemModelsError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item models could not be loaded.';
    } finally {
      itemModelsBusy = false;
    }
  }

  function closeItemModelsDialog() {
    if (itemModelsBusy) return;
    itemModelsDialogOpen = false;
    itemModelsError = '';
  }

  function addItemModel() {
    if (itemModels.length >= 100) {
      itemModelsError = 'An item may have at most 100 models.';
      return;
    }
    itemModels = [...itemModels, { modelCode: 0 }];
    itemModelsError = '';
  }

  function updateItemModel(index: number, value: string) {
    const parsed = Number(value);
    itemModels = itemModels.map((model, modelIndex) => modelIndex === index ? { modelCode: Number.isFinite(parsed) ? parsed : 0 } : model);
    itemModelsError = '';
  }

  function removeItemModel(index: number) {
    itemModels = itemModels.filter((_, modelIndex) => modelIndex !== index);
    itemModelsError = '';
  }

  async function saveItemModels() {
    if (!selectedRecordId) return;
    const modelCodes = itemModels.map((model) => model.modelCode);
    if (modelCodes.some((modelCode) => modelCode < -32768 || modelCode > 32767)) {
      itemModelsError = 'Model codes must fit the captured smallint range.';
      return;
    }
    if (new Set(modelCodes).size !== modelCodes.length) {
      itemModelsError = 'Model codes must be unique.';
      return;
    }
    itemModelsBusy = true;
    itemModelsError = '';
    try {
      const result = await api.replaceItemModels(selectedRecordId, modelCodes);
      itemModels = result.models ?? itemModels;
      itemModelsDialogOpen = false;
      message = `${itemModels.length} item model${itemModels.length === 1 ? '' : 's'} saved in the current tenant scope.`;
    } catch (cause) {
      itemModelsError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Item models could not be saved.';
    } finally {
      itemModelsBusy = false;
    }
  }

  async function openItemPricePolicyDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before editing its price policy.';
      return;
    }
    itemPricePolicyDialogOpen = true;
    itemPricePolicyBusy = true;
    itemPricePolicyError = '';
    try {
      itemPricePolicy = await api.itemPricePolicy(selectedRecordId);
    } catch (cause) {
      itemPricePolicyError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'The item price policy could not be loaded.';
    } finally {
      itemPricePolicyBusy = false;
    }
  }

  function closeItemPricePolicyDialog() {
    if (itemPricePolicyBusy) return;
    itemPricePolicyDialogOpen = false;
    itemPricePolicyError = '';
  }

  function addItemPricePolicyTier() {
    if (itemPricePolicy.tiers.length >= 100) {
      itemPricePolicyError = 'An item price policy may have at most 100 tiers.';
      return;
    }
    itemPricePolicy = {
      ...itemPricePolicy,
      tiers: [...itemPricePolicy.tiers, { quantityLimit: 0, price: '0', expiryDate: '', flatDiscount: '0', discountPercent: '0' }]
    };
    itemPricePolicyError = '';
  }

  function updateItemPricePolicyTier(index: number, key: keyof ItemPricePolicyTier, value: string) {
    const parsed = key === 'quantityLimit' ? Number(value) : value;
    itemPricePolicy = {
      ...itemPricePolicy,
      tiers: itemPricePolicy.tiers.map((tier, tierIndex) => tierIndex === index
        ? { ...tier, [key]: key === 'quantityLimit' ? (Number.isFinite(parsed) ? parsed : 0) : parsed }
        : tier)
    };
    itemPricePolicyError = '';
  }

  function removeItemPricePolicyTier(index: number) {
    itemPricePolicy = { ...itemPricePolicy, tiers: itemPricePolicy.tiers.filter((_, tierIndex) => tierIndex !== index) };
    itemPricePolicyError = '';
  }

  async function saveItemPricePolicy() {
    if (!selectedRecordId || !itemPricePolicy.policy) return;
    const tiers = itemPricePolicy.tiers.map((tier) => ({ ...tier, expiryDate: tier.expiryDate.trim(), price: tier.price.trim(), flatDiscount: tier.flatDiscount.trim(), discountPercent: tier.discountPercent.trim() }));
    if (tiers.some((tier) => !Number.isInteger(tier.quantityLimit))) {
      itemPricePolicyError = 'Quantity limits must be whole numbers.';
      return;
    }
    if (new Set(tiers.map((tier) => `${tier.quantityLimit}|${tier.expiryDate}`)).size !== tiers.length) {
      itemPricePolicyError = 'Quantity and expiry pairs must be unique.';
      return;
    }
    itemPricePolicyBusy = true;
    itemPricePolicyError = '';
    try {
      itemPricePolicy = await api.replaceItemPricePolicy(selectedRecordId, itemPricePolicy.policy.policyCode, tiers);
      itemPricePolicyDialogOpen = false;
      message = `${itemPricePolicy.tiers.length} price-policy tier${itemPricePolicy.tiers.length === 1 ? '' : 's'} saved in the current tenant scope.`;
    } catch (cause) {
      itemPricePolicyError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'The item price policy could not be saved.';
    } finally {
      itemPricePolicyBusy = false;
    }
  }

  async function openItemRegistrationRequestDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before populating its registration request.';
      return;
    }
    itemRegistrationRequestDialogOpen = true;
    itemRegistrationRequestBusy = true;
    itemRegistrationRequestError = '';
    try {
      itemRegistrationRequest = (await api.itemRegistrationRequest(selectedRecordId)).request;
    } catch (cause) {
      itemRegistrationRequestError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'The item registration request could not be loaded.';
    } finally {
      itemRegistrationRequestBusy = false;
    }
  }

  function closeItemRegistrationRequestDialog() {
    if (itemRegistrationRequestBusy) return;
    itemRegistrationRequestDialogOpen = false;
    itemRegistrationRequestError = '';
  }

  async function populateItemRegistrationRequest() {
    if (!selectedRecordId) return;
    itemRegistrationRequestBusy = true;
    itemRegistrationRequestError = '';
    try {
      itemRegistrationRequest = (await api.populateItemRegistrationRequest(selectedRecordId)).request;
      message = `Item registration request ${itemRegistrationRequest?.requestCode ?? ''} populated from the current item payload.`;
    } catch (cause) {
      itemRegistrationRequestError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'The item registration request could not be populated.';
    } finally {
      itemRegistrationRequestBusy = false;
    }
  }

  function openPopulateItemDialog() {
    if (kind !== 'item') {
      message = 'Populate Item is only available on the Item Form.';
      return;
    }
    populateItemDialogOpen = true;
    populateItemBusy = false;
    populateItemError = '';
    populateItemQuery = '';
    populateItemResults = [];
  }

  function closePopulateItemDialog() {
    if (populateItemBusy) return;
    populateItemDialogOpen = false;
    populateItemError = '';
  }

  async function searchPopulateItems() {
    const query = populateItemQuery.trim();
    if (!query) {
      populateItemError = 'Enter an item code, name, barcode, or legacy identifier.';
      populateItemResults = [];
      return;
    }
    populateItemBusy = true;
    populateItemError = '';
    try {
      populateItemResults = (await api.itemLookup(query)).items ?? [];
      if (populateItemResults.length === 0) populateItemError = 'No active canonical item matched the lookup value.';
    } catch (cause) {
      populateItemError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'The item lookup could not be completed.';
      populateItemResults = [];
    } finally {
      populateItemBusy = false;
    }
  }

  async function selectPopulatedItem(item: ItemLookupResult) {
    const record: MasterRecord = {
      id: item.id,
      kind: 'item',
      legacyId: item.legacyId,
      code: item.code,
      name: item.name,
      payload: item.payload,
      active: item.active,
      createdAt: '',
      updatedAt: ''
    };
    populateItemBusy = true;
    populateItemError = '';
    try {
      await selectRecord(record);
      populateItemDialogOpen = false;
      message = `${item.name} populated from the active canonical item lookup.`;
    } catch (cause) {
      populateItemError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'The selected item could not be populated.';
    } finally {
      populateItemBusy = false;
    }
  }

  async function openItemUnpostedTransactionsDialog() {
    if (kind !== 'item' || !selectedRecordId) {
      message = 'Select an item before opening its unposted transaction report.';
      return;
    }
    itemUnpostedTransactionsDialogOpen = true;
    itemUnpostedTransactionsBusy = true;
    itemUnpostedTransactionsError = '';
    itemUnpostedTransactions = [];
    itemUnpostedTransactionsTruncated = false;
    try {
      const result = await api.itemUnpostedTransactions(selectedRecordId);
      itemUnpostedTransactions = result.transactions ?? [];
      itemUnpostedTransactionsTruncated = result.truncated;
    } catch (cause) {
      itemUnpostedTransactionsError = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'The unposted transaction report could not be loaded.';
    } finally {
      itemUnpostedTransactionsBusy = false;
    }
  }

  function closeItemUnpostedTransactionsDialog() {
    if (itemUnpostedTransactionsBusy) return;
    itemUnpostedTransactionsDialogOpen = false;
    itemUnpostedTransactionsError = '';
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
  async function navigateRecordTo(index: number) {
    if (isUser) { message = 'Use the Users list to select an operator.'; return; }
    if (!records.length) await loadRecords();
    if (!records.length) { message = 'No records are available in the current tenant scope.'; return; }
    selectRecord(records[index < 0 ? records.length - 1 : Math.min(index, records.length - 1)]);
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

  function handleMenuCommand(action: MenuAction): boolean {
    switch (action.label) {
      case 'New':
      case 'New Item':
        newRecord();
        return true;
      case 'List':
        activeTab = 'list';
        void loadRecords();
        return true;
      case 'Save':
        void saveRecord();
        return true;
      case 'Delete':
      case 'Delete Item':
        void deleteRecord();
        return true;
      case 'First':
        void navigateRecordTo(0);
        return true;
      case 'Previous':
        void navigateRecord(-1);
        return true;
      case 'Next':
        void navigateRecord(1);
        return true;
      case 'Last':
        void navigateRecordTo(-1);
        return true;
      case 'Set Alternate Item Alias Names':
        void openAlternateAliasDialog();
        return true;
      case 'Set Item Image(s)':
        void openItemImageDialog();
        return true;
      case 'Set Item Notes':
        void openItemNotesDialog();
        return true;
      case 'Set Item Associations':
        void openItemAssociationsDialog();
        return true;
      case 'Set Item Author(s)':
        void openItemAuthorsDialog();
        return true;
      case 'Select Models':
        void openItemModelsDialog();
        return true;
      case 'Set Item Price Policy':
        void openItemPricePolicyDialog();
        return true;
      case 'Populate Item Registration Request':
        void openItemRegistrationRequestDialog();
        return true;
      case 'Populate Item':
        openPopulateItemDialog();
        return true;
      case 'Show Un-Posted Transaction Report':
        void openItemUnpostedTransactionsDialog();
        return true;
      case 'Exit':
        window.location.assign('/app/legacy');
        return true;
      default:
        return false;
    }
  }
</script>

<svelte:window onkeydown={enableInteractive} />

<svelte:head><title>WASEELA · ABUZAR V3 · {definition.title}</title></svelte:head>

<main class:legacy-master-list-tab={activeTab === 'list'} class:legacy-master-customer-baseline={kind === 'customer' && activeTab === 'detail' && !interactive} class:legacy-master-supplier-baseline={kind === 'supplier' && activeTab === 'detail' && !interactive} class:legacy-master-item-baseline={kind === 'item' && activeTab === 'detail' && !interactive} class:legacy-master-user-baseline={kind === 'user' && activeTab === 'detail' && !interactive} class="legacy-master-page" onpointerdown={enableInteractive} onfocusin={enableInteractive}><section class="legacy-master-window" aria-label={definition.title}>
  <header class="legacy-transaction-titlebar"><a href="/app/legacy" aria-label="Back to main window">←</a><h1>{formatLegacyTitle(authenticatedUsername, clock)} : [{definition.title}]</h1></header>
  <LegacyMenuBar context={kind === 'item' ? 'item-master' : 'manage-groups'} windowId={'master-' + kind} windowLabel={definition.title} windowHref={'/app/master/' + kind} onCommand={handleMenuCommand} />
  <div class="legacy-transaction-toolbar" role="toolbar" aria-label="Master data toolbar"><button type="button" aria-label="New record" onpointerdown={() => { interactive = true; }} onclick={() => newRecord()} disabled={!canEdit}>▱</button><button type="button" aria-label="Save" onpointerdown={() => { interactive = true; }} onclick={() => { void saveRecord(); }} disabled={busy || !canEdit}>▣</button><button type="button" aria-label="Refresh records" onclick={() => { void loadRecords(); }}>⌕</button><span class="legacy-toolbar-separator"></span><button type="button" aria-label="Previous record" onclick={() => { void navigateRecord(-1); }}>◀</button><button type="button" aria-label="Next record" onclick={() => { void navigateRecord(1); }}>▶</button><span class="legacy-toolbar-caption">{isCanonical ? 'Canonical master API' : canEdit ? 'Tenant-scoped master API' : 'Generic compatibility surface · read-only'}</span></div>
  <div class="legacy-transaction-tabs"><button class:active={activeTab === 'detail'} type="button" onclick={() => { activeTab = 'detail'; }}>▦ Detail</button><button class:active={activeTab === 'list'} type="button" onclick={() => { activeTab = 'list'; void loadRecords(); }}>▦ List</button></div>
  <div class="legacy-master-body"><form class="legacy-master-form" onsubmit={(event) => { event.preventDefault(); void saveRecord(); }}>
    {#each definition.fields as field, index}<label>{field.label}:{#if field.kind === 'textarea'}<textarea rows="3" bind:value={values[index]} disabled={!canEdit}></textarea>{:else if field.kind === 'select'}<select bind:value={values[index]} disabled={!canEdit}>{#each field.label === 'Group' ? roleOptions : field.options ?? [] as option}<option value={option}>{option}</option>{/each}</select>{:else}<input type={field.kind === 'date' ? 'date' : field.kind === 'number' ? 'number' : 'text'} step={field.kind === 'number' ? 'any' : undefined} bind:value={values[index]} disabled={!canEdit} />{/if}</label>{/each}
    {#if isUser}<label>Branch:<select bind:value={selectedBranchId} onchange={(event) => { void changeUserBranch(event.currentTarget.value); }}><option value="">Default branch</option>{#each branches as branch}<option value={branch.id}>{branch.code} · {branch.name}</option>{/each}</select></label><label>Counter:<select bind:value={selectedCounterId}><option value="">Default counter</option>{#each counters as counter}<option value={counter.id}>{counter.code} · {counter.name}</option>{/each}</select></label>{/if}
    <div class="legacy-master-actions"><button type="submit" disabled={busy || !canEdit}>Save</button><button type="button" onclick={() => void deleteRecord()} disabled={busy || !canEdit || !selectedRecordId}>Delete</button><button type="button" onclick={() => cancelEdit()}>Cancel</button></div>
  </form>{#if kind === 'item'}<section class="legacy-item-suppliers" aria-label="Item suppliers"><div class="legacy-item-suppliers-heading"><strong>Suppliers</strong><button type="button" onclick={addSupplierRow} disabled={!canEdit}>Add supplier</button></div>{#if supplierRows.length === 0}<p>No supplier links for this item.</p>{:else}<table><thead><tr><th>Priority</th><th>Supplier ID</th><th>Rate</th><th>Disc%</th><th>Qty</th><th>Bonus</th><th>Days</th><th></th></tr></thead><tbody>{#each supplierRows as row, index}<tr><td><input aria-label={`Supplier priority ${index + 1}`} value={row.priority} oninput={(event) => updateSupplierRow(index, 'priority', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier legacy id ${index + 1}`} value={row.legacySupplierId} oninput={(event) => updateSupplierRow(index, 'legacySupplierId', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier rate ${index + 1}`} value={row.rate} oninput={(event) => updateSupplierRow(index, 'rate', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier discount percent ${index + 1}`} value={row.discountPercent} oninput={(event) => updateSupplierRow(index, 'discountPercent', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier quantity ${index + 1}`} value={row.quantity} oninput={(event) => updateSupplierRow(index, 'quantity', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier bonus ${index + 1}`} value={row.bonus} oninput={(event) => updateSupplierRow(index, 'bonus', event.currentTarget.value)} disabled={!canEdit} /></td><td><input aria-label={`Supplier days ${index + 1}`} value={row.days} oninput={(event) => updateSupplierRow(index, 'days', event.currentTarget.value)} disabled={!canEdit} /></td><td><button type="button" aria-label={`Remove supplier ${index + 1}`} onclick={() => removeSupplierRow(index)} disabled={!canEdit}>×</button></td></tr>{/each}</tbody></table>{/if}</section>{/if}<div class="legacy-master-list"><div class="legacy-master-list-tools"><label>Search<input aria-label="Master search" bind:value={searchQuery} onkeydown={(event) => { if (event.key === 'Enter') void loadRecords(); }} /></label><button type="button" onclick={loadRecords}>Filter / Retrieve</button><label>Sort<select aria-label="Sort criteria" bind:value={sortColumn}><option value="name">Name</option><option value="code">Code</option><option value="active">Active</option></select></label><button type="button" onclick={() => { sortDirection = sortDirection === 'asc' ? 'desc' : 'asc'; }}>{sortDirection === 'asc' ? 'Asc' : 'Desc'}</button><label>Filter<select aria-label="Filter column" bind:value={filterColumn}><option value="all">All</option><option value="code">Code</option><option value="name">Name</option><option value="active">Active</option></select></label><select aria-label="Filter operator" bind:value={filterOperator}><option value="contains">contains</option><option value="equals">equals</option></select><input aria-label="Filter value" bind:value={filterValue} /><label>Find<input aria-label="Find records" bind:value={findQuery} placeholder="Find as you type" /></label></div><div>List ({isUser ? operators.length : visibleRecords.length})</div>{#if !canEdit && !isUser}<p class="legacy-master-readonly">Generic compatibility surface — read-only; canonical API not available.</p>{/if}{#if isUser}{#if operators.length === 0}<p>No operators in the current tenant scope.</p>{:else}<table><thead><tr><th>User</th><th>Name</th><th>Group</th><th>Active</th></tr></thead><tbody>{#each operators as item}<tr><td><button type="button" onclick={() => selectOperator(item)}>{item.username}</button></td><td><button type="button" onclick={() => selectOperator(item)}>{item.displayName}</button></td><td>{item.roles.join(', ')}</td><td>{item.active ? 'YES' : 'NO'}</td></tr>{/each}</tbody></table>{/if}{:else if visibleRecords.length === 0}<p>No records in the current tenant scope.</p>{:else}<table><thead><tr><th>Code</th><th>Name</th><th>Active</th></tr></thead><tbody>{#each visibleRecords as record}<tr><td><button type="button" onclick={() => { void selectRecord(record); }}>{record.code}</button></td><td><button type="button" onclick={() => { void selectRecord(record); }}>{record.name}</button></td><td>{record.active ? 'YES' : 'NO'}</td></tr>{/each}</tbody></table>{/if}</div></div>
  <footer class="legacy-transaction-footer">{#if error}<span class="error" role="alert">{error}</span>{:else}<span role="status">{message || 'Ready'}</span>{/if}<a href="/app/legacy">Back to main window</a></footer>
</section>
{#if alternateAliasDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeAlternateAliasDialog(); }}>
    <div class="legacy-item-alias-dialog" role="dialog" aria-modal="true" aria-label="Alternate Item Alias Names">
      <h2>Set Alternate Item Alias Names</h2>
      <p>Item: {values[1] || 'Unnamed item'}</p>
      {#if alternateAliasError}<p class="error" role="alert">{alternateAliasError}</p>{/if}
      <div class="legacy-item-alias-list">
        {#if alternateAliases.length === 0}
          <p>No alternate aliases are defined.</p>
        {:else}
          {#each alternateAliases as alias, index}
            <div class="legacy-item-alias-row">
              <input aria-label={`Alternate alias ${index + 1}`} value={alias} oninput={(event) => updateAlternateAlias(index, event.currentTarget.value)} disabled={alternateAliasBusy} />
              <button type="button" aria-label={`Remove alternate alias ${index + 1}`} onclick={() => removeAlternateAlias(index)} disabled={alternateAliasBusy}>×</button>
            </div>
          {/each}
        {/if}
      </div>
      <div class="legacy-master-actions">
        <button type="button" onclick={addAlternateAlias} disabled={alternateAliasBusy}>Add alias</button>
        <button type="button" onclick={() => void saveAlternateAliases()} disabled={alternateAliasBusy}>Save</button>
        <button type="button" onclick={closeAlternateAliasDialog} disabled={alternateAliasBusy}>Cancel</button>
      </div>
    </div>
  </div>
{/if}
{#if itemImageDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeItemImageDialog(); }}>
    <div class="legacy-item-image-dialog" role="dialog" aria-modal="true" aria-label="Set Item Images">
      <h2>Set Item Image(s)</h2>
      <p>Item: {values[1] || 'Unnamed item'}</p>
      {#if itemImageError}<p class="error" role="alert">{itemImageError}</p>{/if}
      <label class="legacy-item-image-file">Add image:<input aria-label="Add item image file" type="file" accept="image/*" onchange={(event) => { void addItemImageFile(event); }} disabled={itemImageBusy} /></label>
      <div class="legacy-item-image-list">
        {#if itemImages.length === 0}
          <p>No item images are defined.</p>
        {:else}
          {#each itemImages as image, index}
            <div class="legacy-item-image-row">
              {#if image.imageType.startsWith('image/')}<img src={`data:${image.imageType};base64,${image.imageData}`} alt={image.imageDescription || `Item image ${index + 1}`} />{/if}
              <div class="legacy-item-image-fields">
                <label>Description:<input aria-label={`Image description ${index + 1}`} value={image.imageDescription} oninput={(event) => updateItemImage(index, 'imageDescription', event.currentTarget.value)} disabled={itemImageBusy} /></label>
                <label>Type:<input aria-label={`Image type ${index + 1}`} value={image.imageType} oninput={(event) => updateItemImage(index, 'imageType', event.currentTarget.value)} disabled={itemImageBusy} /></label>
              </div>
              <button type="button" aria-label={`Remove item image ${index + 1}`} onclick={() => removeItemImage(index)} disabled={itemImageBusy}>×</button>
            </div>
          {/each}
        {/if}
      </div>
      <div class="legacy-master-actions">
        <button type="button" onclick={() => void saveItemImages()} disabled={itemImageBusy}>Save</button>
        <button type="button" onclick={closeItemImageDialog} disabled={itemImageBusy}>Cancel</button>
      </div>
    </div>
  </div>
{/if}
{#if itemNotesDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeItemNotesDialog(); }}>
    <div class="legacy-item-notes-dialog" role="dialog" aria-modal="true" aria-label="Set Item Notes">
      <h2>Set Item Notes</h2>
      <p>Item: {values[1] || 'Unnamed item'}</p>
      {#if itemNotesError}<p class="error" role="alert">{itemNotesError}</p>{/if}
      <label class="legacy-item-notes-text">Notes (UTF-8 / RTF):<textarea aria-label="Item notes text" rows="12" value={itemNotesText} oninput={(event) => updateItemNotesText(event.currentTarget.value)} disabled={itemNotesBusy}></textarea></label>
      <details class="legacy-item-notes-raw">
        <summary>Raw Notes Blob (base64)</summary>
        <textarea aria-label="Item notes base64" rows="5" value={itemNotesData} oninput={(event) => updateItemNotesRaw(event.currentTarget.value)} disabled={itemNotesBusy}></textarea>
        <small>Use the raw value to preserve or edit non-UTF-8 legacy encodings.</small>
      </details>
      <div class="legacy-master-actions">
        <button type="button" onclick={() => updateItemNotesText('')} disabled={itemNotesBusy}>Clear</button>
        <button type="button" onclick={() => void saveItemNotes()} disabled={itemNotesBusy}>Save</button>
        <button type="button" onclick={closeItemNotesDialog} disabled={itemNotesBusy}>Cancel</button>
      </div>
    </div>
  </div>
{/if}
{#if itemAssociationsDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeItemAssociationsDialog(); }}>
    <div class="legacy-item-associations-dialog" role="dialog" aria-modal="true" aria-label="Set Item Associations">
      <h2>Set Item Associations</h2>
      <p>Item: {values[1] || 'Unnamed item'}</p>
      {#if itemAssociationsError}<p class="error" role="alert">{itemAssociationsError}</p>{/if}
      <div class="legacy-item-associations-list">
        {#if itemAssociations.length === 0}
          <p>No associated items are defined.</p>
        {:else}
          {#each itemAssociations as association, index}
            <div class="legacy-item-association-row">
              <input aria-label={`Associated item legacy id ${index + 1}`} value={association.legacyItemId} oninput={(event) => updateItemAssociation(index, event.currentTarget.value)} disabled={itemAssociationsBusy} />
              <span>{association.code || 'Unresolved'}{association.name ? ` · ${association.name}` : ''}</span>
              <button type="button" aria-label={`Remove item association ${index + 1}`} onclick={() => removeItemAssociation(index)} disabled={itemAssociationsBusy}>×</button>
            </div>
          {/each}
        {/if}
      </div>
      <div class="legacy-master-actions">
        <button type="button" onclick={addItemAssociation} disabled={itemAssociationsBusy}>Add association</button>
        <button type="button" onclick={() => void saveItemAssociations()} disabled={itemAssociationsBusy}>Save</button>
        <button type="button" onclick={closeItemAssociationsDialog} disabled={itemAssociationsBusy}>Cancel</button>
      </div>
    </div>
  </div>
{/if}
{#if itemAuthorsDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeItemAuthorsDialog(); }}>
    <div class="legacy-item-authors-dialog" role="dialog" aria-modal="true" aria-label="Set Item Authors">
      <h2>Set Item Author(s)</h2>
      <p>Item: {values[1] || 'Unnamed item'}</p>
      {#if itemAuthorsError}<p class="error" role="alert">{itemAuthorsError}</p>{/if}
      <div class="legacy-item-authors-list">
        {#if itemAuthors.length === 0}
          <p>No item authors are defined.</p>
        {:else}
          {#each itemAuthors as author, index}
            <div class="legacy-item-author-row">
              <label>Author code:<input aria-label={`Item author code ${index + 1}`} type="number" min="1" step="1" value={author.authorCode} oninput={(event) => updateItemAuthor(index, 'authorCode', event.currentTarget.value)} disabled={itemAuthorsBusy} /></label>
              <label>Priority:<input aria-label={`Item author priority ${index + 1}`} type="number" min="0" max="255" step="1" value={author.priority} oninput={(event) => updateItemAuthor(index, 'priority', event.currentTarget.value)} disabled={itemAuthorsBusy} /></label>
              <button type="button" aria-label={`Remove item author ${index + 1}`} onclick={() => removeItemAuthor(index)} disabled={itemAuthorsBusy}>×</button>
            </div>
          {/each}
        {/if}
      </div>
      <div class="legacy-master-actions">
        <button type="button" onclick={addItemAuthor} disabled={itemAuthorsBusy}>Add author</button>
        <button type="button" onclick={() => void saveItemAuthors()} disabled={itemAuthorsBusy}>Save</button>
        <button type="button" onclick={closeItemAuthorsDialog} disabled={itemAuthorsBusy}>Cancel</button>
      </div>
    </div>
  </div>
{/if}
{#if itemModelsDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeItemModelsDialog(); }}>
    <div class="legacy-item-models-dialog" role="dialog" aria-modal="true" aria-label="Select Models">
      <h2>Select Models</h2>
      <p>Item: {values[1] || 'Unnamed item'}</p>
      {#if itemModelsError}<p class="error" role="alert">{itemModelsError}</p>{/if}
      <div class="legacy-item-models-list">
        {#if itemModels.length === 0}
          <p>No models are assigned.</p>
        {:else}
          {#each itemModels as model, index}
            <div class="legacy-item-model-row">
              <label>Model code:<input aria-label={`Item model code ${index + 1}`} type="number" min="-32768" max="32767" step="1" value={model.modelCode} oninput={(event) => updateItemModel(index, event.currentTarget.value)} disabled={itemModelsBusy} /></label>
              <button type="button" aria-label={`Remove item model ${index + 1}`} onclick={() => removeItemModel(index)} disabled={itemModelsBusy}>×</button>
            </div>
          {/each}
        {/if}
      </div>
      <div class="legacy-master-actions">
        <button type="button" onclick={addItemModel} disabled={itemModelsBusy}>Add model</button>
        <button type="button" onclick={() => void saveItemModels()} disabled={itemModelsBusy}>Save</button>
        <button type="button" onclick={closeItemModelsDialog} disabled={itemModelsBusy}>Cancel</button>
      </div>
    </div>
  </div>
{/if}
{#if itemPricePolicyDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeItemPricePolicyDialog(); }}>
    <div class="legacy-item-price-policy-dialog" role="dialog" aria-modal="true" aria-label="Set Item Price Policy">
      <h2>Set Item Price Policy</h2>
      <p>Item: {values[1] || 'Unnamed item'}</p>
      {#if itemPricePolicy.policy}<p>Policy: {itemPricePolicy.policy.policyCode} · {itemPricePolicy.policy.name}</p>{/if}
      {#if itemPricePolicyError}<p class="error" role="alert">{itemPricePolicyError}</p>{/if}
      {#if !itemPricePolicy.policy}
        <p>No source-backed PricePolicy is linked to this item.</p>
      {:else}
        <div class="legacy-item-price-policy-list">
          {#if itemPricePolicy.tiers.length === 0}
            <p>No price-policy tiers are defined.</p>
          {:else}
            {#each itemPricePolicy.tiers as tier, index}
              <div class="legacy-item-price-policy-row">
                <label>Qty limit:<input aria-label={`Price policy quantity limit ${index + 1}`} type="number" step="1" value={tier.quantityLimit} oninput={(event) => updateItemPricePolicyTier(index, 'quantityLimit', event.currentTarget.value)} disabled={itemPricePolicyBusy} /></label>
                <label>Price:<input aria-label={`Price policy price ${index + 1}`} value={tier.price} oninput={(event) => updateItemPricePolicyTier(index, 'price', event.currentTarget.value)} disabled={itemPricePolicyBusy} /></label>
                <label>Expiry:<input aria-label={`Price policy expiry ${index + 1}`} type="date" value={tier.expiryDate} oninput={(event) => updateItemPricePolicyTier(index, 'expiryDate', event.currentTarget.value)} disabled={itemPricePolicyBusy} /></label>
                <label>Flat disc:<input aria-label={`Price policy flat discount ${index + 1}`} value={tier.flatDiscount} oninput={(event) => updateItemPricePolicyTier(index, 'flatDiscount', event.currentTarget.value)} disabled={itemPricePolicyBusy} /></label>
                <label>Disc %:<input aria-label={`Price policy discount percent ${index + 1}`} value={tier.discountPercent} oninput={(event) => updateItemPricePolicyTier(index, 'discountPercent', event.currentTarget.value)} disabled={itemPricePolicyBusy} /></label>
                <button type="button" aria-label={`Remove price policy tier ${index + 1}`} onclick={() => removeItemPricePolicyTier(index)} disabled={itemPricePolicyBusy}>×</button>
              </div>
            {/each}
          {/if}
        </div>
        <div class="legacy-master-actions">
          <button type="button" onclick={addItemPricePolicyTier} disabled={itemPricePolicyBusy}>Add tier</button>
          <button type="button" onclick={() => void saveItemPricePolicy()} disabled={itemPricePolicyBusy}>Save</button>
          <button type="button" onclick={closeItemPricePolicyDialog} disabled={itemPricePolicyBusy}>Cancel</button>
        </div>
      {/if}
    </div>
  </div>
{/if}
{#if itemRegistrationRequestDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeItemRegistrationRequestDialog(); }}>
    <div class="legacy-item-registration-request-dialog" role="dialog" aria-modal="true" aria-label="Populate Item Registration Request">
      <h2>Populate Item Registration Request</h2>
      <p>Item: {values[1] || 'Unnamed item'}</p>
      {#if itemRegistrationRequestError}<p class="error" role="alert">{itemRegistrationRequestError}</p>{/if}
      {#if itemRegistrationRequest}
        <dl class="legacy-item-registration-request-summary">
          <dt>Request code</dt><dd>{itemRegistrationRequest.requestCode}</dd>
          <dt>Requested at</dt><dd>{itemRegistrationRequest.requestedAt}</dd>
          <dt>Legacy item</dt><dd>{itemRegistrationRequest.legacyItemId}</dd>
          <dt>Sent</dt><dd>{itemRegistrationRequest.sent === 'Y' ? 'YES' : 'NO'}</dd>
          <dt>Source fields</dt><dd>{Object.keys(itemRegistrationRequest.payload ?? {}).length}</dd>
        </dl>
        <p class="legacy-item-registration-request-boundary">The request is recorded locally from the source-shaped item payload. External registration-server delivery remains a separate acceptance boundary.</p>
      {:else}
        <p>No registration request has been populated for this item.</p>
      {/if}
      <div class="legacy-master-actions">
        <button type="button" onclick={() => void populateItemRegistrationRequest()} disabled={itemRegistrationRequestBusy}>Populate</button>
        <button type="button" onclick={closeItemRegistrationRequestDialog} disabled={itemRegistrationRequestBusy}>Close</button>
      </div>
    </div>
  </div>
{/if}
{#if itemUnpostedTransactionsDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closeItemUnpostedTransactionsDialog(); }}>
    <div class="legacy-item-unposted-dialog" role="dialog" aria-modal="true" aria-label="Show Un-Posted Transaction Report">
      <h2>Show Un-Posted Transaction Report</h2>
      <p>Item: {values[1] || 'Unnamed item'} · Code: {values[0] || 'Unselected'}</p>
      {#if itemUnpostedTransactionsError}<p class="error" role="alert">{itemUnpostedTransactionsError}</p>{/if}
      {#if itemUnpostedTransactionsBusy}
        <p class="legacy-item-unposted-empty">Loading unposted transactions...</p>
      {:else if itemUnpostedTransactions.length === 0}
        <p class="legacy-item-unposted-empty">No unposted canonical transactions contain this item.</p>
      {:else}
        <div class="legacy-item-unposted-table-wrap">
          <table class="legacy-item-unposted-table">
            <thead><tr><th>Document</th><th>Kind</th><th>Date</th><th>Qty</th><th>Unit price</th><th>Line total</th></tr></thead>
            <tbody>
              {#each itemUnpostedTransactions as transaction}
                <tr>
                  <td>{transaction.documentNumber}</td>
                  <td>{transaction.kind}</td>
                  <td>{transaction.occurredAt.replace('T', ' ').slice(0, 19)}</td>
                  <td>{transaction.quantity}</td>
                  <td>{transaction.unitPrice}</td>
                  <td>{transaction.lineTotal}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        {#if itemUnpostedTransactionsTruncated}<p class="legacy-item-unposted-boundary">Showing the first 200 rows for this item. Narrowed legacy source reconciliation remains an acceptance boundary.</p>{/if}
      {/if}
      <div class="legacy-master-actions"><button type="button" onclick={closeItemUnpostedTransactionsDialog} disabled={itemUnpostedTransactionsBusy}>Close</button></div>
    </div>
  </div>
{/if}
{#if populateItemDialogOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) closePopulateItemDialog(); }}>
    <div class="legacy-item-populate-dialog" role="dialog" aria-modal="true" aria-label="Populate Item">
      <h2>Populate Item</h2>
      <p>Lookup an active item by code, name, barcode, alias, or legacy identifier.</p>
      {#if populateItemError}<p class="error" role="alert">{populateItemError}</p>{/if}
      <div class="legacy-item-populate-search">
        <input aria-label="Populate item lookup" value={populateItemQuery} oninput={(event) => { populateItemQuery = event.currentTarget.value; populateItemError = ''; }} onkeydown={(event) => { if (event.key === 'Enter') void searchPopulateItems(); }} disabled={populateItemBusy} />
        <button type="button" onclick={() => void searchPopulateItems()} disabled={populateItemBusy}>Search</button>
      </div>
      <div class="legacy-item-populate-results">
        {#if populateItemResults.length === 0}
          <p>No lookup results.</p>
        {:else}
          {#each populateItemResults as item}
            <button type="button" class="legacy-item-populate-result" onclick={() => void selectPopulatedItem(item)} disabled={populateItemBusy}>
              <strong>{item.code}</strong><span>{item.name}</span><small>{item.legacyId}</small>
            </button>
          {/each}
        {/if}
      </div>
      <div class="legacy-master-actions"><button type="button" onclick={closePopulateItemDialog} disabled={populateItemBusy}>Cancel</button></div>
    </div>
  </div>
{/if}
</main>
