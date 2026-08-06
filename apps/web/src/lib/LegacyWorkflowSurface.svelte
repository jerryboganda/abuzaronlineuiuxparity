<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import type { LegacyRight, LegacyScope, RoleSummary, SessionResponse, SyncEnvelope } from '@abuzar/contracts';
  import { AbuzarApi, ApiError, OfflineQueue, newEventId } from '$lib/api';
  import { formatLegacyTitle } from '$lib/legacy-title';

  export let section: 'maintenance' | 'manage';

  type WorkflowField = { key: string; label: string; kind?: 'text' | 'number' | 'date' | 'select'; value?: string; options?: string[] };

  const api = new AbuzarApi();
  const queue = new OfflineQueue();
  const permissionOptions = [
    { code: 'sales.read', label: 'Sales - view' },
    { code: 'sales.write', label: 'Sales - post/edit' },
    { code: 'purchases.read', label: 'Purchases - view' },
    { code: 'purchases.write', label: 'Purchases - post/edit' },
    { code: 'reports.read', label: 'Reports - view' },
    { code: 'master.read', label: 'Basic data - view' },
    { code: 'master.write', label: 'Basic data - edit' },
    { code: 'maintenance.write', label: 'Maintenance - run' },
    { code: 'manage.users', label: 'Manage - users' },
    { code: 'manage.groups', label: 'Manage - groups' },
    { code: 'sync.push', label: 'Sync - upload branch events' },
    { code: 'sync.pull', label: 'Sync - download central events' },
    { code: 'sync.review', label: 'Sync - review conflicts' },
    { code: 'preferences.read', label: 'Preferences - view' },
    { code: 'preferences.write', label: 'Preferences - edit' }
  ];
  const workflowFieldDefinitions: Record<string, WorkflowField[]> = {
    'change-items-price': [
      { key: 'itemCode', label: 'Item Code' }, { key: 'priceType', label: 'Price Type', kind: 'select', value: 'Sale Price', options: ['Sale Price', 'Purchase Price'] }, { key: 'price', label: 'New Price', kind: 'number' }, { key: 'effectiveDate', label: 'Effective Date', kind: 'date' }
    ],
    'priority-setting': [
      { key: 'itemCode', label: 'Item Code' }, { key: 'priority', label: 'Priority', kind: 'number', value: '1' }, { key: 'effectiveDate', label: 'Effective Date', kind: 'date' }
    ],
    'change-item-discount': [
      { key: 'itemCode', label: 'Item Code' }, { key: 'discountType', label: 'Discount Type', kind: 'select', value: 'Percent', options: ['Percent', 'Amount'] }, { key: 'discount', label: 'Discount', kind: 'number' }, { key: 'effectiveDate', label: 'Effective Date', kind: 'date' }
    ],
    'modify-sale-invoices': [
      { key: 'invoiceNumber', label: 'Invoice Number' }, { key: 'customer', label: 'Customer' }, { key: 'invoiceDate', label: 'Invoice Date', kind: 'date' }, { key: 'reason', label: 'Reason' }
    ],
    'modify-last-transaction-date': [
      { key: 'transactionDate', label: 'Last Transaction Date', kind: 'date' }, { key: 'reason', label: 'Reason' }
    ],
    'group-wise-godown-setting': [
      { key: 'group', label: 'Group' }, { key: 'godown', label: 'Godown' }, { key: 'active', label: 'Active', kind: 'select', value: 'Yes', options: ['Yes', 'No'] }
    ],
    'interface-setting': [
      { key: 'interfaceType', label: 'Interface', kind: 'select', value: 'Printer', options: ['Printer', 'Barcode', 'Cash Drawer', 'SMS', 'Email'] }, { key: 'enabled', label: 'Enabled', kind: 'select', value: 'No', options: ['Yes', 'No'] }, { key: 'endpoint', label: 'Endpoint / Port' }
    ],
    'update-item-basic-data': [
      { key: 'itemCode', label: 'Item Code' }, { key: 'field', label: 'Field', kind: 'select', value: 'Name', options: ['Name', 'Manufacturer', 'Category', 'Class', 'Location'] }, { key: 'value', label: 'New Value' }
    ],
    'update-item-suppliers': [
      { key: 'itemCode', label: 'Item Code' }, { key: 'supplier', label: 'Supplier' }, { key: 'purchasePrice', label: 'Purchase Price', kind: 'number' }, { key: 'priority', label: 'Priority', kind: 'number', value: '1' }
    ],
    'change-item-reorder-qty': [
      { key: 'itemCode', label: 'Item Code' }, { key: 'reorderQty', label: 'Reorder Quantity', kind: 'number' }, { key: 'minimumQty', label: 'Minimum Quantity', kind: 'number' }
    ],
    'import-previous-sales': [
      { key: 'sourceFile', label: 'Source File' }, { key: 'fromDate', label: 'From Date', kind: 'date' }, { key: 'toDate', label: 'To Date', kind: 'date' }, { key: 'dryRun', label: 'Validate Only', kind: 'select', value: 'Yes', options: ['Yes', 'No'] }
    ],
    'inplace-initialization': [
      { key: 'scope', label: 'Initialization Scope', kind: 'select', value: 'Current Branch', options: ['Current Branch', 'Current Tenant'] }, { key: 'confirm', label: 'Confirm', kind: 'select', value: 'No', options: ['Yes', 'No'] }
    ],
    'lock-item-batches': [
      { key: 'itemCode', label: 'Item Code' }, { key: 'batch', label: 'Batch' }, { key: 'locked', label: 'Locked', kind: 'select', value: 'Yes', options: ['Yes', 'No'] }
    ],
    'group-wise-header-setting': [
      { key: 'group', label: 'Group' }, { key: 'saleHeader', label: 'Sale Header' }, { key: 'purchaseHeader', label: 'Purchase Header' }, { key: 'enabled', label: 'Enabled', kind: 'select', value: 'Yes', options: ['Yes', 'No'] }
    ],
    'group-allowed-price-setting': [
      { key: 'group', label: 'Group' }, { key: 'priceLevel', label: 'Price Level', kind: 'select', value: 'Retail', options: ['Retail', 'Wholesale', 'Special'] }, { key: 'allowed', label: 'Allowed', kind: 'select', value: 'Yes', options: ['Yes', 'No'] }
    ],
    'group-wise-cash-account-setting': [
      { key: 'group', label: 'Group' }, { key: 'cashAccount', label: 'Cash Account' }, { key: 'active', label: 'Active', kind: 'select', value: 'Yes', options: ['Yes', 'No'] }
    ],
    'job-schedule': [
      { key: 'job', label: 'Job' }, { key: 'schedule', label: 'Schedule' }, { key: 'enabled', label: 'Enabled', kind: 'select', value: 'No', options: ['Yes', 'No'] }
    ],
    'group-wise-supplier-category': [
      { key: 'group', label: 'Group' }, { key: 'category', label: 'Supplier Category' }, { key: 'enabled', label: 'Enabled', kind: 'select', value: 'Yes', options: ['Yes', 'No'] }
    ],
    increase: [
      { key: 'itemLegacyId', label: 'Item legacy ID' }, { key: 'godownId', label: 'Godown ID' }, { key: 'batchNumber', label: 'Batch' }, { key: 'expiryDate', label: 'Expiry', kind: 'date' }, { key: 'unitCost', label: 'Unit cost', kind: 'number' }
    ],
    decrease: [
      { key: 'itemLegacyId', label: 'Item legacy ID' }, { key: 'godownId', label: 'Godown ID' }, { key: 'batchNumber', label: 'Batch' }, { key: 'expiryDate', label: 'Expiry', kind: 'date' }
    ],
    'stock-adjustment': [
      { key: 'itemLegacyId', label: 'Item legacy ID' }, { key: 'godownId', label: 'Godown ID' }, { key: 'batchNumber', label: 'Batch' }, { key: 'expiryDate', label: 'Expiry', kind: 'date' }, { key: 'adjustmentSign', label: 'Adjustment sign', kind: 'select', value: '1', options: ['1', '-1'] }
    ],
    'opening-stock': [
      { key: 'itemLegacyId', label: 'Item legacy ID' }, { key: 'godownId', label: 'Godown ID' }, { key: 'batchNumber', label: 'Batch' }, { key: 'expiryDate', label: 'Expiry', kind: 'date' }, { key: 'unitCost', label: 'Unit cost', kind: 'number' }
    ]
  };
  let session: SessionResponse['context'] = null;
  let clock = new Date();
  let reference = '';
  let notes = '';
  let itemName = '';
  let quantity = '1';
  let amount = '0';
  let shiftAction: 'open' | 'close' = 'open';
  let currentPassword = '';
  let newPassword = '';
  let confirmPassword = '';
  let busy = false;
  let online = true;
  let message = '';
  let error = '';
  let checks: Array<{ table: string; rows: number; status: string }> = [];
  let interactive = false;
  let backupDialog: 'database' | 'device' | 'information' = 'database';
  let backupDialogInteractive = false;
  let roles: RoleSummary[] = [];
  let roleCode = '';
  let roleName = '';
  let rolePermissions: string[] = [];
  let legacyRights: LegacyRight[] = [];
  let roleScopes: LegacyScope[] = [];
  let rightsLoaded = false;
  let selectedRoleId = '';
  let shiftRows: Array<{ id: string; branchId: string; counterId: string; operatorId: string; openedAt: string; closedAt?: string; status: string; openingAmount: string; closingAmount?: string }> = [];
  let sessionRows: Array<{ userId: string; username: string; displayName: string; branchId: string; counterId: string; createdAt: string; lastSeenAt: string; expiresAt: string; current: boolean }> = [];
  let extraValues: Record<string, string> = {};
  let configuredKind = '';
  let savedWorkflow: { reference: string; notes: string; itemName: string; quantity: string; amount: string; shiftAction: 'open' | 'close'; extraValues: Record<string, string> } | null = null;
  let operationId = '';
  let operationStatus = '';

  $: kind = $page?.params?.kind ?? 'workflow';
  $: legacyPath = $page?.url?.searchParams?.get('legacyPath') ?? '';
  $: legacyLeaf = String(legacyPath ?? '').split(' > ').at(-1)?.replace(/\t.*$/, '').replace(/&/g, '').trim() ?? '';
  $: title = legacyLeaf || kind.split('-').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');
  $: adjustment = section === 'maintenance' && ['increase', 'decrease', 'stock-adjustment'].includes(kind);
  $: isBackup = section === 'maintenance' && kind === 'backup-database';
  $: isIntegrity = section === 'maintenance' && kind === 'check-database-integrity';
  $: isGroups = section === 'manage' && kind === 'groups';
  $: isCashierActivity = section === 'manage' && kind === 'cashier-activity-window';
  $: isSessionMonitor = section === 'manage' && kind === 'session-monitor';
  $: persistedKind = section === 'maintenance' ? kind : `manage-${kind}`;
  $: workflowFields = workflowFieldDefinitions[kind] ?? [];
  $: if (kind && kind !== configuredKind) {
    configuredKind = kind;
    extraValues = Object.fromEntries(workflowFields.map((field) => [field.key, field.value ?? '']));
  }

  function enableInteractive() {
    if (isBackup && !backupDialogInteractive) return;
    interactive = true;
  }

  function enableBackupDialogInteractive() {
    interactive = true;
    backupDialogInteractive = true;
  }

  function cancelWorkflow() {
    reference = savedWorkflow?.reference ?? '';
    notes = savedWorkflow?.notes ?? '';
    itemName = savedWorkflow?.itemName ?? '';
    quantity = savedWorkflow?.quantity ?? '1';
    amount = savedWorkflow?.amount ?? '0';
    currentPassword = '';
    newPassword = '';
    confirmPassword = '';
    shiftAction = savedWorkflow?.shiftAction ?? 'open';
    extraValues = savedWorkflow ? { ...savedWorkflow.extraValues } : Object.fromEntries(workflowFields.map((field) => [field.key, field.value ?? '']));
    error = '';
    message = 'Changes cancelled; the last saved state was restored.';
  }

  function openBackupDialog(dialog: 'database' | 'device' | 'information') {
    backupDialog = dialog;
    backupDialogInteractive = false;
  }

  onMount(() => {
    const clockTimer = window.setInterval(() => { clock = new Date(); }, 1000);
    online = navigator.onLine;
    const update = () => (online = navigator.onLine);
    window.addEventListener('online', update);
    window.addEventListener('offline', update);
    if (isGroups) void loadRoles();
    if (isCashierActivity) void loadShiftActivity();
    if (isSessionMonitor) void loadSessionMonitor();
    if (!isGroups && !isSessionMonitor && kind !== 'change-password') void loadWorkflowState();
    void api.session().then((result) => { if (result.authenticated) session = result.context; }).catch(() => { /* offline workflow remains visible */ });
    return () => { window.clearInterval(clockTimer); window.removeEventListener('online', update); window.removeEventListener('offline', update); };
  });

  async function loadWorkflowState() {
    try {
      const result = await api.maintenanceState(persistedKind);
      const saved = Object.fromEntries(result.items.map((item) => [item.caption, item.value]));
      if (typeof saved.reference === 'string') reference = saved.reference;
      if (typeof saved.notes === 'string') notes = saved.notes;
      if (typeof saved.itemName === 'string') itemName = saved.itemName;
      if (typeof saved.quantity === 'string') quantity = saved.quantity;
      if (typeof saved.amount === 'string') amount = saved.amount;
      if (saved.shiftAction === 'open' || saved.shiftAction === 'close') shiftAction = saved.shiftAction;
      for (const field of workflowFields) if (typeof saved[field.key] === 'string') extraValues = { ...extraValues, [field.key]: saved[field.key] as string };
      savedWorkflow = { reference, notes, itemName, quantity, amount, shiftAction, extraValues: { ...extraValues } };
      const last = result.lastOperation;
      if (last) {
        operationId = last.id;
        operationStatus = last.status;
      }
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) error = 'Saved workflow values could not be loaded.';
    }
  }

  async function loadRoles() {
    try {
      roles = (await api.roles()).roles;
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 403) error = 'The current operator does not have group-management permission.';
      else if (!(cause instanceof ApiError && cause.status === 401)) error = 'Groups could not be loaded.';
    }
  }

  async function loadShiftActivity() {
    try {
      shiftRows = (await api.shifts()).shifts;
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) error = 'Cashier activity could not be loaded.';
    }
  }

  async function loadSessionMonitor() {
    try {
      sessionRows = (await api.sessionMonitor()).sessions;
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 403) error = 'The current operator does not have session-monitor permission.';
      else if (!(cause instanceof ApiError && cause.status === 401)) error = 'Session monitor could not be loaded.';
    }
  }

  function selectRole(role: RoleSummary) {
    selectedRoleId = role.id;
    roleCode = role.code;
    roleName = role.name;
    rolePermissions = [...(role.permissions ?? [])];
    legacyRights = [];
    roleScopes = [];
    rightsLoaded = false;
    message = `${role.name} selected.`;
    error = '';
    void loadRoleRights(role.id);
  }

  async function loadRoleRights(roleId: string) {
    try {
      const rights = await api.roleRights(roleId);
      legacyRights = rights.legacyRights ?? [];
      roleScopes = rights.scopes ?? [];
      rolePermissions = rights.permissions ?? rolePermissions;
      rightsLoaded = true;
    } catch (cause) {
      rightsLoaded = false;
      if (!(cause instanceof ApiError && cause.status === 401)) {
        error = cause instanceof ApiError ? cause.problem?.detail ?? cause.message : 'Imported group rights could not be loaded.';
      }
    }
  }

  function togglePermission(code: string, checked: boolean) {
    rolePermissions = checked
      ? [...new Set([...rolePermissions, code])]
      : rolePermissions.filter((permission) => permission !== code);
  }

  function toggleLegacyRight(rightCode: string, checked: boolean) {
    legacyRights = legacyRights.map((right) => right.rightCode === rightCode ? { ...right, allowed: checked } : right);
  }

  function toggleRoleScope(scopeKind: string, scopeKey: string, checked: boolean) {
    roleScopes = roleScopes.map((scope) => scope.scopeKind === scopeKind && scope.scopeKey === scopeKey ? { ...scope, allowed: checked } : scope);
  }

  async function saveRole() {
    busy = true;
    message = '';
    error = '';
    try {
      if (!roleCode.trim() || !roleName.trim()) throw new Error('Group code and name are required.');
      const updating = Boolean(selectedRoleId);
      const saved = updating
        ? await api.updateRole(selectedRoleId, roleCode.trim(), roleName.trim(), rolePermissions)
        : await api.createRole(roleCode.trim(), roleName.trim(), rolePermissions);
      if (updating && rightsLoaded) {
        await api.updateRoleRights(saved.id, {
          permissions: rolePermissions,
          legacyRights: legacyRights.map((right) => ({ rightCode: right.rightCode, allowed: right.allowed })),
          scopes: roleScopes.map((scope) => ({ scopeKind: scope.scopeKind, scopeKey: scope.scopeKey, allowed: scope.allowed }))
        });
      }
      roles = updating ? roles.map((role) => role.id === saved.id ? { ...role, ...saved } : role) : [...roles, saved];
      selectedRoleId = saved.id;
      message = `${saved.name} ${updating ? 'saved' : 'created'} successfully.`;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'The group could not be saved.';
    } finally {
      busy = false;
    }
  }

  function newRole() {
    selectedRoleId = '';
    roleCode = '';
    roleName = '';
    rolePermissions = [];
    legacyRights = [];
    roleScopes = [];
    rightsLoaded = false;
    message = 'New group ready.';
    error = '';
  }

  function inventoryEvent(): SyncEnvelope {
    if (!session?.tenantId || !session.branchId || !session.counterId || !session.operatorId) throw new Error('Select a branch and counter before posting an adjustment.');
    const eventId = newEventId();
    const direction = kind === 'increase' || kind === 'opening-stock' ? 'in' : kind === 'decrease' ? 'out' : 'adjustment';
    const adjustmentSign = kind === 'stock-adjustment' ? Number(extraValues.adjustmentSign || '1') : 1;
    return {
      eventId,
      aggregate: 'inventory',
      aggregateId: eventId,
      tenantId: session.tenantId,
      branchId: session.branchId,
      counterId: session.counterId,
      operatorId: session.operatorId,
      occurredAt: new Date().toISOString(),
      idempotencyKey: `maintenance:${kind}:${reference || eventId}`,
      schemaVersion: 1,
      payload: {
        itemName,
        itemLegacyId: extraValues.itemLegacyId || itemName,
        quantity,
        direction,
        adjustmentSign,
        godownId: extraValues.godownId || '',
        batchNumber: extraValues.batchNumber || '',
        expiryDate: extraValues.expiryDate || '',
        unitCost: extraValues.unitCost || '0',
        reference,
        notes
      }
    };
  }

  function workflowPayload(): Record<string, unknown> {
    const payload: Record<string, unknown> = { reference, notes };
    if (adjustment) Object.assign(payload, { itemName, quantity });
    if (kind === 'cashier-job') Object.assign(payload, { shiftAction, amount });
    for (const field of workflowFields) payload[field.key] = extraValues[field.key] ?? field.value ?? '';
    return payload;
  }

  async function run() {
    busy = true;
    message = '';
    error = '';
    checks = [];
    let event: SyncEnvelope | undefined;
    try {
      if (adjustment) {
        event = inventoryEvent();
        if (!online) {
          await queue.enqueue(event);
          message = 'Adjustment saved to the branch queue for synchronization.';
        } else {
          await api.createInventoryTransaction(event);
          message = `${title} posted successfully.`;
        }
      } else if (kind === 'cashier-job') {
        if (shiftAction === 'open') {
          await api.openShift(amount);
          message = 'Cashier shift opened successfully.';
        } else {
          if (!reference.trim()) throw new Error('Enter the open shift identifier before closing it.');
          await api.closeShift(reference.trim(), amount);
          message = 'Cashier shift closed successfully.';
        }
      } else if (kind === 'change-password') {
        await api.changePassword(currentPassword, newPassword, confirmPassword);
        currentPassword = '';
        newPassword = '';
        confirmPassword = '';
        message = 'Password changed successfully.';
      } else if (kind === 'session-monitor') {
        await loadSessionMonitor();
        message = `${sessionRows.length} active session${sessionRows.length === 1 ? '' : 's'} in the current branch.`;
      } else if (section === 'maintenance') {
        const result = await api.maintenance(kind, workflowPayload());
        checks = result.checks ?? [];
        operationId = result.operationId ?? '';
        operationStatus = result.status ?? '';
        message = result.message ?? `${title} saved.`;
      } else {
        const result = await api.maintenance(persistedKind, workflowPayload());
        operationId = result.operationId ?? '';
        operationStatus = result.status ?? '';
        message = result.message ?? `${title} saved.`;
      }
      if (!event) savedWorkflow = { reference, notes, itemName, quantity, amount, shiftAction, extraValues: { ...extraValues } };
    } catch (cause) {
      if (event && (cause instanceof TypeError || !online)) {
        await queue.enqueue(event);
        message = 'Central API unavailable; saved to the branch queue.';
      } else if (cause instanceof ApiError && cause.status === 401) {
        error = 'Your session has expired. Sign in again.';
      } else {
        error = cause instanceof Error ? cause.message : `${title} failed.`;
      }
    } finally {
      busy = false;
    }
  }
</script>

<svelte:window onkeydown={enableInteractive} />

<svelte:head><title>WASEELA · ABUZAR V3 · {title}</title></svelte:head>

<main class:legacy-integrity-baseline={isIntegrity && !interactive && checks.length === 0} class="legacy-workflow-page" onpointerdown={enableInteractive} onfocusin={enableInteractive}>
  {#if isBackup}
    <section class="legacy-captured-dialog-canvas" aria-label="Backup Database">
      {#if backupDialog === 'database'}
        <div class:legacy-backup-database-captured={!backupDialogInteractive} class="legacy-captured-dialog legacy-backup-database-dialog" role="dialog" aria-modal="true" aria-label="Backup Database">
          <h2>Backup Database</h2><input aria-label="Backup destination" />
          <button type="button" onclick={() => { message = 'Submitting backup request…'; enableBackupDialogInteractive(); void run(); }}>BackUp Now</button>
          <button type="button" onclick={() => { enableBackupDialogInteractive(); window.location.assign('/app/legacy'); }}>Cancel</button>
          <button type="button" onclick={() => { enableBackupDialogInteractive(); openBackupDialog('device'); }}>Verify Backup</button>
          <button type="button" onclick={() => { enableBackupDialogInteractive(); openBackupDialog('information'); }}>Last Backup Info.</button>
          {#if message}<p role="status">{message}</p>{/if}{#if operationId}<p role="status">Operation {operationId}: {operationStatus}</p>{/if}
        </div>
      {:else if backupDialog === 'device'}
        <div class:legacy-backup-device-captured={!backupDialogInteractive} class="legacy-captured-dialog legacy-backup-device-dialog" role="alertdialog" aria-modal="true" aria-label="Backup Device">
          <h2>BackUp Device</h2><p>Unable to locate Database backup device.</p>
          <button type="button" onclick={() => { enableBackupDialogInteractive(); openBackupDialog('database'); }}>OK</button>
        </div>
      {:else}
        <div class:legacy-backup-information-captured={!backupDialogInteractive} class="legacy-captured-dialog legacy-backup-information-dialog" role="alertdialog" aria-modal="true" aria-label="Backup Information">
          <h2>Backup Information</h2><p>Backup Age = N/A<br />Backup Size = N/A</p><p>Any Accidental Event may Result in Loss of 46237 days previous data.</p><p>You Must take Backup Now to avoid any potential data loss....</p>
          <button type="button" onclick={() => { enableBackupDialogInteractive(); openBackupDialog('database'); }}>OK</button>
        </div>
      {/if}
    </section>
  {:else if isGroups}
    <section class="legacy-workflow-window" aria-label="Groups">
      <header class="legacy-transaction-titlebar"><a href="/app/legacy" aria-label="Back to main window">←</a><h1>{formatLegacyTitle(session?.username, clock)} : [Groups]</h1></header>
      <div class="legacy-transaction-toolbar" role="toolbar" aria-label="Groups toolbar">
        <button type="button" aria-label="New group" onclick={newRole} title="New">△</button>
        <button type="button" aria-label="Save group" onclick={saveRole} disabled={busy} title="Save">▣</button>
        <button type="button" aria-label="Refresh groups" onclick={loadRoles} title="Refresh">⌕</button>
        <span class="legacy-toolbar-separator"></span><span class="legacy-toolbar-caption">{online ? 'Online' : 'Offline'} · Manage</span>
      </div>
      <div class="legacy-workflow-body">
        <form class="legacy-workflow-form" onsubmit={(event) => { event.preventDefault(); void saveRole(); }}>
          <label>Group Code:<input bind:value={roleCode} maxlength="80" required /></label>
          <label>Group Name:<input bind:value={roleName} maxlength="160" required /></label>
          <fieldset class="legacy-permission-fieldset">
            <legend>Permissions</legend>
            <div class="legacy-permission-grid">
              {#each permissionOptions as permission}
                <label class="legacy-permission-option"><input type="checkbox" checked={rolePermissions.includes(permission.code)} onchange={(event) => togglePermission(permission.code, (event.currentTarget as HTMLInputElement).checked)} /><span>{permission.label}</span></label>
              {/each}
            </div>
          </fieldset>
          {#if selectedRoleId}
            <fieldset class="legacy-permission-fieldset" aria-label="Imported legacy rights">
              <legend>Imported legacy rights {rightsLoaded ? '' : '(loading)'}</legend>
              {#if rightsLoaded && legacyRights.length}
                <div class="legacy-rights-matrix">
                  {#each legacyRights as right}
                    <label class="legacy-permission-option">
                      <input type="checkbox" checked={right.allowed} onchange={(event) => toggleLegacyRight(right.rightCode, (event.currentTarget as HTMLInputElement).checked)} />
                      <span>{right.rightCode} {right.permission ? `(${right.permission})` : '(ambiguous mapping)'}</span>
                    </label>
                  {/each}
                </div>
                {#if legacyRights.some((right) => right.mapping === 'ambiguous')}
                  <p class="legacy-access-exception">Ambiguous legacy right codes are shown for audit but are not claimed as exact command mappings.</p>
                {/if}
              {:else if rightsLoaded}
                <p>No imported legacy right rows are available for this group.</p>
              {:else}
                <p>Imported rights are unavailable; no legacy rights will be changed.</p>
              {/if}
            </fieldset>
            {#if rightsLoaded && roleScopes.length}
              <fieldset class="legacy-permission-fieldset" aria-label="Imported access scopes">
                <legend>Branch / godown / price / report scopes</legend>
                <div class="legacy-rights-matrix">
                  {#each roleScopes as scope}
                    <label class="legacy-permission-option">
                      <input type="checkbox" checked={scope.allowed} onchange={(event) => toggleRoleScope(scope.scopeKind, scope.scopeKey, (event.currentTarget as HTMLInputElement).checked)} />
                      <span>{scope.scopeKind}: {scope.scopeLabel || scope.scopeKey}</span>
                    </label>
                  {/each}
                </div>
              </fieldset>
            {/if}
          {/if}
          <div class="legacy-master-actions"><button type="button" onclick={saveRole} disabled={busy}>Save</button><button type="button" onclick={newRole}>Cancel</button></div>
        </form>
        <div class="legacy-workflow-help legacy-role-list"><h2>Groups</h2>
          {#if roles.length === 0}<p>No groups are defined in the current tenant.</p>{:else}<table><thead><tr><th>Code</th><th>Name</th><th>Members</th><th>Permissions</th></tr></thead><tbody>{#each roles as role}<tr><td><button type="button" onclick={() => selectRole(role)}>{role.code}</button></td><td>{role.name}</td><td>{role.memberCount}</td><td>{role.permissions?.length ?? 0}</td></tr>{/each}</tbody></table>{/if}
        </div>
      </div>
      <footer class="legacy-transaction-footer">{#if error}<span class="error" role="alert">{error}</span>{:else}<span role="status">{message || 'Ready'}</span>{/if}<a href="/app/legacy">Back to main window</a></footer>
    </section>
  {:else if isCashierActivity}
    <section class="legacy-workflow-window" aria-label="Cashier Activity Window">
      <header class="legacy-transaction-titlebar"><a href="/app/legacy" aria-label="Back to main window">←</a><h1>{formatLegacyTitle(session?.username, clock)} : [Cashier Activity Window]</h1></header>
      <div class="legacy-transaction-toolbar" role="toolbar" aria-label="Cashier activity toolbar"><button type="button" aria-label="Refresh cashier activity" onclick={loadShiftActivity}>⟳</button><span class="legacy-toolbar-separator"></span><span class="legacy-toolbar-caption">{online ? 'Online' : 'Offline'} · Cashier Activity</span></div>
      <div class="legacy-workflow-body"><div class="legacy-workflow-help"><h2>Cashier Activity Window</h2>{#if shiftRows.length === 0}<p>No cashier shifts are recorded for the current branch and counter.</p>{:else}<table><thead><tr><th>Shift</th><th>Operator</th><th>Opened</th><th>Closed</th><th>Status</th><th>Opening</th><th>Closing</th></tr></thead><tbody>{#each shiftRows as shift}<tr><td>{shift.id}</td><td>{shift.operatorId}</td><td>{shift.openedAt}</td><td>{shift.closedAt || '—'}</td><td>{shift.status}</td><td>{shift.openingAmount}</td><td>{shift.closingAmount || '—'}</td></tr>{/each}</tbody></table>{/if}</div></div>
      <footer class="legacy-transaction-footer">{#if error}<span class="error" role="alert">{error}</span>{:else}<span role="status">{message || 'Ready'}</span>{/if}<a href="/app/legacy">Back to main window</a></footer>
    </section>
  {:else if isSessionMonitor}
    <section class="legacy-workflow-window" aria-label="Session Monitor">
      <header class="legacy-transaction-titlebar"><a href="/app/legacy" aria-label="Back to main window">←</a><h1>{formatLegacyTitle(session?.username, clock)} : [Session Monitor]</h1></header>
      <div class="legacy-transaction-toolbar" role="toolbar" aria-label="Session monitor toolbar"><button type="button" aria-label="Refresh sessions" onclick={loadSessionMonitor}>⟳</button><span class="legacy-toolbar-separator"></span><span class="legacy-toolbar-caption">{online ? 'Online' : 'Offline'} · Branch scoped</span></div>
      <div class="legacy-workflow-body"><div class="legacy-workflow-help"><h2>Session Monitor</h2>{#if sessionRows.length === 0}<p>No active sessions are recorded for the current branch.</p>{:else}<table><thead><tr><th>User</th><th>Name</th><th>Branch</th><th>Counter</th><th>Last seen</th><th>Current</th></tr></thead><tbody>{#each sessionRows as activeSession}<tr><td>{activeSession.username}</td><td>{activeSession.displayName}</td><td>{activeSession.branchId}</td><td>{activeSession.counterId || '—'}</td><td>{activeSession.lastSeenAt}</td><td>{activeSession.current ? 'Yes' : 'No'}</td></tr>{/each}</tbody></table>{/if}</div></div>
      <footer class="legacy-transaction-footer">{#if error}<span class="error" role="alert">{error}</span>{:else}<span role="status">{message || 'Ready'}</span>{/if}<a href="/app/legacy">Back to main window</a></footer>
    </section>
  {:else}
  <section class="legacy-workflow-window" aria-label={title}>
    <header class="legacy-transaction-titlebar"><a href="/app/legacy" aria-label="Back to main window">←</a><h1>{formatLegacyTitle(session?.username, clock)} : [{title}]</h1></header>
    <div class="legacy-transaction-toolbar" role="toolbar" aria-label="Workflow toolbar">
      <button type="button" aria-label="New workflow" onclick={() => { reference = ''; notes = ''; itemName = ''; quantity = '1'; extraValues = Object.fromEntries(workflowFields.map((field) => [field.key, field.value ?? ''])); message = 'New workflow ready.'; error = ''; }} title="New">△</button>
      <button type="button" aria-label="Save workflow" onclick={run} disabled={busy} title="Save">▣</button>
      <button type="button" aria-label="Print workflow" onclick={() => { message = 'Print preview is ready.'; window.print(); }} title="Print">▤</button>
      <span class="legacy-toolbar-separator"></span><span class="legacy-toolbar-caption">{online ? 'Online' : 'Offline'} · {section === 'maintenance' ? 'Maintenance' : 'Manage'}</span>
    </div>
    <div class="legacy-workflow-body">
      <form class="legacy-workflow-form" onsubmit={(event) => { event.preventDefault(); run(); }}>
        <label>Reference / code:<input bind:value={reference} /></label>
        {#if adjustment}<label>Item:<input bind:value={itemName} required /></label><label>Quantity:<input type="number" min="0.0001" step="0.0001" bind:value={quantity} required /></label>{/if}
        {#if kind === 'cashier-job'}<label>Shift action:<select bind:value={shiftAction}><option value="open">Open shift</option><option value="close">Close shift</option></select></label><label>Amount:<input type="number" step="0.01" bind:value={amount} /></label>{/if}
        {#if kind === 'change-password'}<label>Current password:<input type="password" bind:value={currentPassword} required /></label><label>New password:<input type="password" bind:value={newPassword} required /></label><label>Confirm password:<input type="password" bind:value={confirmPassword} required /></label>{/if}
        {#each workflowFields as field}
          <label>{field.label}:{#if field.kind === 'select'}<select bind:value={extraValues[field.key]}>{#each field.options ?? [] as option}<option value={option}>{option}</option>{/each}</select>{:else}<input type={field.kind === 'date' ? 'date' : field.kind === 'number' ? 'number' : 'text'} bind:value={extraValues[field.key]} />{/if}</label>
        {/each}
        <label>Notes:<textarea rows="5" bind:value={notes}></textarea></label>
        <div class="legacy-master-actions"><button type="submit" disabled={busy}>Save</button><button type="button" onclick={cancelWorkflow}>Cancel</button></div>
      </form>
      <div class="legacy-workflow-help"><h2>{title}</h2><p>This captured legacy workflow is available in the shared Svelte shell. Its tenant and branch scope comes from the authenticated session.</p>{#if kind === 'backup-database'}<p>Backups are recorded centrally and produced only by a configured deployment backup policy; this screen never claims a physical backup succeeded.</p>{/if}{#if operationId}<p role="status">Operation {operationId}: {operationStatus || 'recorded'}</p>{/if}{#if checks.length}<table><thead><tr><th>Table</th><th>Rows</th><th>Status</th></tr></thead><tbody>{#each checks as check}<tr><td>{check.table}</td><td>{check.rows}</td><td>{check.status}</td></tr>{/each}</tbody></table>{/if}</div>
    </div>
    <footer class="legacy-transaction-footer">{#if error}<span class="error" role="alert">{error}</span>{:else}<span role="status">{message || 'Ready'}</span>{/if}<a href="/app/legacy">Back to main window</a></footer>
  </section>
  {/if}
</main>
