<script lang="ts">
  import { onMount } from 'svelte';
  import type { BranchSummary, ConflictRecord, ReportRow } from '@abuzar/contracts';
  import { AbuzarApi } from '$lib/api';
  import { localDateString } from '$lib/calendar-date';

  let connectionLabel = 'Online';
  let workspaceLabel = 'Abuzar Pharmacy';
  let branchLabel = 'Head Branch';
  let counterLabel = 'Counter 01';
  let operatorLabel = 'Operator';
  let conflicts: ConflictRecord[] = [];
  let todaySales: ReportRow[] = [];
  let shiftRows: Array<{ id: string; status: string; openedAt: string; closedAt?: string; openingAmount: string; closingAmount?: string }> = [];
  let branchSummaries: BranchSummary[] = [];
  let conflictBusy = '';
  let conflictMessage = '';
  let dashboardNotice = '';
  let salesAnalysisDays = 1;
  let summaryAnalysisDays = '30';
  let purchaseAnalysisDays = '30';
  let receiptAnalysisDays = '30';
  let issueAnalysisDays = '30';
  let adjustmentAnalysisDays = '30';
  let godownTransferAnalysisDays = '30';
  let accountsAnalysisDays = '30';
  let serviceAnalysisDays = '30';
  let inventoryBreakupOn = 'Category';
  let checkManualBackupHealth = false;
  let ageOfItemChangesDays = '';
  let inSaleReturn = false;
  let inBasicData = false;
  let inAccountsModule = false;
  let inPurchaseReturn = false;
  let inPurchaseReturnServices = false;
  let inAccounts = false;
  let enableQuickSearch = false;
  let quickSearchType = '';
  let inventorySystem = '';
  let inventoryMovementMethod = '';
  let enableAliasName = false;
  let searchItemCodeIfAliasMissing = false;
  let allowLoginMultipleTimes = false;
  let autoResponsiveSearch = false;
  const api = new AbuzarApi();

  function amount(value: string): number {
    const parsed = Number(String(value ?? '').replace(/,/g, ''));
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function money(value: number): string {
    return value.toLocaleString('en-PK', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }

  $: todaySalesTotal = money(todaySales.reduce((sum, row) => sum + amount(row.amount), 0));
  $: openShiftCount = shiftRows.filter((shift) => shift.status === 'open').length;
  $: dashboardActivity = todaySales.length
    ? todaySales.slice(0, 5).map((row) => ({ time: row.occurredAt?.slice(11, 16) || '--:--', counter: counterLabel, operator: row.party || operatorLabel, action: 'Sale posted', amount: row.amount ? `Rs. ${row.amount}` : '—' }))
    : [{ time: '--:--', counter: counterLabel, operator: operatorLabel, action: 'No sales recorded today', amount: '—' }];

  onMount(() => {
    const updateConnection = () => (connectionLabel = navigator.onLine ? 'Online' : 'Offline');
    updateConnection();
    window.addEventListener('online', updateConnection);
    window.addEventListener('offline', updateConnection);
    void api.session().then(async (result) => {
      if (!result.authenticated || !result.context) {
        window.location.assign('/login');
        return;
      }
      workspaceLabel = result.context.tenantCode;
      branchLabel = result.context.branchId ? `Branch ${result.context.branchId.slice(0, 8)}` : 'Tenant scope';
      counterLabel = result.context.counterId ? `Counter ${result.context.counterId.slice(0, 8)}` : 'Counter scope';
      operatorLabel = result.context.displayName;
      const today = localDateString();
      try {
        const dashboardPrefs = await api.preferences('Dashboard');
        const yesNo = (value: string | undefined) => (value || '').trim();
        try {
          const generalPrefs = await api.preferences('General');
          for (const item of generalPrefs.registry ?? generalPrefs.items ?? []) {
            if (item.caption === 'Check Manual Backup Health At StartUp:') checkManualBackupHealth = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'Age of Item Changes For Data Carry (in Days):') ageOfItemChangesDays = item.value || ageOfItemChangesDays;
            if (item.caption === 'In Sale/Return:') inSaleReturn = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'In Basic Data:') inBasicData = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'In Accounts Module:') inAccountsModule = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'In Purchase/Purchase Return:') inPurchaseReturn = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'In Purchase/Purchase Return Services:') inPurchaseReturnServices = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'In Accounts:') inAccounts = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'Enable Quick Search:') enableQuickSearch = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'Quick Search Type:') quickSearchType = item.value || quickSearchType;
            if (item.caption === 'Inventory System:') inventorySystem = item.value || inventorySystem;
            if (item.caption === 'Inventory Movement Method:') inventoryMovementMethod = item.value || inventoryMovementMethod;
            if (item.caption === 'Enable Alias Name:') enableAliasName = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'Search Item Code if Alias Name Not Found:') searchItemCodeIfAliasMissing = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'Allow Login A User Multiple Times:') allowLoginMultipleTimes = /^(yes|true|1|y)$/i.test(item.value || 'No');
            if (item.caption === 'Auto Responsive Search With Alternate Alias Name:') autoResponsiveSearch = /^(yes|true|1|y)$/i.test(item.value || 'No');
          }
        } catch {
          /* general extras keep defaults */
        }
        for (const item of dashboardPrefs.registry ?? dashboardPrefs.items ?? []) {
          if (item.caption === 'Sales Analysis Days Lim.:') {
            const parsed = Number.parseInt(item.value || '1', 10);
            if (parsed > 0) salesAnalysisDays = parsed;
          }
          if (item.caption === 'Summary Analysis Days Lim.:') summaryAnalysisDays = item.value || summaryAnalysisDays;
          if (item.caption === 'Purchase Analysis Days Lim.:') purchaseAnalysisDays = item.value || purchaseAnalysisDays;
          if (item.caption === 'Receipt Analysis Days Lim.:') receiptAnalysisDays = item.value || receiptAnalysisDays;
          if (item.caption === 'Issue Analysis Days Lim.:') issueAnalysisDays = item.value || issueAnalysisDays;
          if (item.caption === 'Adjustment Analysis Days Lim.:') adjustmentAnalysisDays = item.value || adjustmentAnalysisDays;
          if (item.caption === 'Godown Transfer Analysis Days Lim.:') godownTransferAnalysisDays = item.value || godownTransferAnalysisDays;
          if (item.caption === 'Accounts Analysis Days Lim.:') accountsAnalysisDays = item.value || accountsAnalysisDays;
          if (item.caption === 'Service Analysis Days Lim.:') serviceAnalysisDays = item.value || serviceAnalysisDays;
          if (item.caption === 'Inventory Breakup On:') inventoryBreakupOn = yesNo(item.value) || inventoryBreakupOn;
        }
      } catch {
        /* dashboard extras keep defaults */
      }
      const start = new Date();
      start.setDate(start.getDate() - Math.max(0, salesAnalysisDays - 1));
      const from = localDateString(start);
      const [conflictResult, salesResult, shiftResult, branchResult] = await Promise.all([
        api.conflicts(),
        api.transactions('sale', from, today),
        api.shifts(),
        api.branches()
      ]);
      conflicts = conflictResult.conflicts;
      todaySales = salesResult.rows;
      shiftRows = shiftResult.shifts;
      branchSummaries = branchResult.branches;
    }).catch(() => undefined);
    return () => {
      window.removeEventListener('online', updateConnection);
      window.removeEventListener('offline', updateConnection);
    };
  });

  async function resolveConflict(conflict: ConflictRecord, status: 'resolved' | 'dismissed') {
    conflictBusy = conflict.id;
    conflictMessage = '';
    try {
      await api.resolveConflict(conflict.id, status, { reviewedBy: operatorLabel });
      conflicts = conflicts.map((item) => item.id === conflict.id ? { ...item, status } : item);
      conflictMessage = `Conflict ${status}.`;
    } catch (cause) {
      conflictMessage = cause instanceof Error ? cause.message : 'Unable to update the conflict.';
    } finally {
      conflictBusy = '';
    }
  }

  async function signOut() {
    await api.logout().catch(() => undefined);
    window.location.assign('/login');
  }

  function exportDashboard() {
    const lines = [['Document', 'Date', 'Customer', 'Item', 'Quantity', 'Amount'], ...todaySales.map((row) => [row.document, row.occurredAt, row.party, row.item, row.quantity, row.amount])];
    const csv = lines.map((line) => line.map((cell) => `"${String(cell ?? '').replace(/"/g, '""')}"`).join(',')).join('\r\n');
    const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = `abuzar-dashboard-${localDateString()}.csv`;
    link.click();
    URL.revokeObjectURL(url);
    dashboardNotice = 'Dashboard export is ready.';
  }

  const menuItems = [
    { label: 'Dashboard', icon: '▦', href: '/app', active: true },
    { label: 'Sales', icon: '▤', href: '/app/sales', active: false },
    { label: 'Purchases', icon: '⇩', href: '/app/purchase/pack', active: false },
    { label: 'Inventory', icon: '▥', href: '/app/maintenance/increase', active: false },
    { label: 'Accounts', icon: '◒', href: '/app/master/customer', active: false },
    { label: 'Reports', icon: '▧', href: '/app/report/daily-sales-detail', active: false }
  ];

</script>

<svelte:head>
  <title>Abuzar Next · Workspace</title>
</svelte:head>

<div class="app-shell" data-testid="workspace-page">
  <header class="topbar">
    <div class="topbar-left">
      <div class="brand-mark tiny">A</div>
      <span class="product-name">Abuzar Software</span>
      <span class="separator">/</span>
      <span class="workspace-name">Operations</span>
    </div>
    <div class="topbar-right">
      <span class="connection-pill"><span class:offline={connectionLabel === 'Offline'} class="status-dot online"></span> {connectionLabel}</span>
      <button class="operator-chip" type="button" aria-label="Current operator" onclick={() => window.location.assign('/context')}>{operatorLabel} · {counterLabel} <span>⌄</span></button>
      <button class="icon-button" type="button" aria-label="Notifications" onclick={() => { dashboardNotice = conflicts.length ? `${conflicts.length} synchronization review item(s).` : 'No new notifications.'; }}>♢</button>
      <button class="avatar" type="button" aria-label="Operator menu" onclick={() => window.location.assign('/context')}>OP</button>
    </div>
  </header>

  <div class="workspace">
    <aside class="sidebar" aria-label="Primary navigation">
      <div class="scope-card">
        <p class="scope-label">CURRENT WORKSPACE</p>
        <strong>{workspaceLabel}</strong>
        <span>{branchLabel} · {counterLabel}</span>
      </div>
      <nav>
        {#each menuItems as item}
          <a class:active={item.active} class="nav-item" href={item.href}>
            <span class="nav-icon">{item.icon}</span>
            <span>{item.label}</span>
          </a>
        {/each}
      </nav>
      <div class="sidebar-bottom">
        <a class="nav-item" href="/app/preferences"><span class="nav-icon">⚙</span><span>Preferences</span></a>
        <button class="nav-item" type="button" onclick={signOut}><span class="nav-icon">↪</span><span>Sign out</span></button>
      </div>
    </aside>

    <main class="content">
      <div class="page-heading">
        <div>
          <p class="eyebrow">{branchLabel.toUpperCase()} · {counterLabel.toUpperCase()}</p>
          <h1>Dashboard</h1>
          <p class="muted">Tuesday, 05 August 2026 · Local activity is synchronized.</p>
        </div>
        <div class="heading-actions">
          <button class="button secondary" type="button" onclick={exportDashboard}>Export</button>
          <a class="button primary" href="/app/sales">New sale</a>
        </div>
      </div>

      <section class="metric-grid" aria-label="Operational summary">
        <article class="metric-card">
          <div class="metric-top"><span>Today’s sales</span><span class="metric-icon blue">↗</span></div>
          <strong>Rs. {todaySalesTotal}</strong>
          <small class="muted">{todaySales.length} persisted document(s)</small>
        </article>
        <article class="metric-card">
          <div class="metric-top"><span>Open shifts</span><span class="metric-icon green">◷</span></div>
          <strong>{String(openShiftCount).padStart(2, '0')}</strong>
          <small class="muted">{shiftRows.length} shift record(s)</small>
        </article>
        <article class="metric-card">
          <div class="metric-top"><span>Low stock items</span><span class="metric-icon amber">!</span></div>
          <strong>—</strong>
          <small class="muted">Stock snapshot pending</small>
        </article>
        <article class="metric-card">
          <div class="metric-top"><span>Receivables</span><span class="metric-icon purple">◌</span></div>
          <strong>—</strong>
          <small class="muted">Receivables ledger pending</small>
        </article>
      </section>
      <section class="legacy-sale-optional-field" aria-label="Dashboard analysis limits">
        <p class="muted">Summary {summaryAnalysisDays}d · Purchase {purchaseAnalysisDays}d · Receipt {receiptAnalysisDays}d · Issue {issueAnalysisDays}d · Adjustment {adjustmentAnalysisDays}d · Godown transfer {godownTransferAnalysisDays}d · Accounts {accountsAnalysisDays}d · Service {serviceAnalysisDays}d · Breakup {inventoryBreakupOn} · Age of item changes {ageOfItemChangesDays || '—'}d{#if checkManualBackupHealth} · Backup health extra{/if}{#if inSaleReturn} · In Sale/Return{/if}{#if inBasicData} · In Basic Data{/if}{#if inAccountsModule} · In Accounts Module{/if}{#if inPurchaseReturn} · In Purchase/Return{/if}{#if inPurchaseReturnServices} · In Purchase/Return Services{/if}{#if inAccounts} · In Accounts{/if}{#if enableQuickSearch} · Quick search extra{/if}{quickSearchType ? ` · Quick search type ${quickSearchType}` : ''}{inventorySystem ? ` · ${inventorySystem}` : ''}{inventoryMovementMethod ? ` · ${inventoryMovementMethod}` : ''}{#if enableAliasName} · Alias extra{/if}{#if searchItemCodeIfAliasMissing} · Alias code extra{/if}{#if allowLoginMultipleTimes} · Multi-login extra{/if}{#if autoResponsiveSearch} · Responsive search extra{/if}</p>
      </section>

      <section class="content-grid">
        <article class="panel activity-panel">
          <div class="panel-heading"><div><h2>Recent activity</h2><p class="muted">Live counter events</p></div><a href="/app/report/daily-sales-detail">View all</a></div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>Time</th><th>Counter</th><th>Operator</th><th>Activity</th><th class="align-right">Amount</th></tr></thead>
              <tbody>
                {#each dashboardActivity as row}
                  <tr><td>{row.time}</td><td>{row.counter}</td><td>{row.operator}</td><td><span class="activity-dot"></span>{row.action}</td><td class="align-right amount">{row.amount}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        </article>
        <article class="panel branch-panel">
          <div class="panel-heading"><div><h2>Branch status</h2><p class="muted">Tenant-wide overview</p></div><button class="more-button" type="button" aria-label="More branch actions" onclick={() => window.location.assign('/context')}>•••</button></div>
          {#if branchSummaries.length === 0}<div class="branch-row"><span class="branch-avatar">—</span><div><strong>No branches in scope</strong><small>Choose a tenant/branch context to continue.</small></div><span class="health queued">Pending</span></div>{:else}{#each branchSummaries as branch}<div class="branch-row"><span class="branch-avatar">{branch.code.slice(0, 2).toUpperCase()}</span><div><strong>{branch.name}</strong><small>{branch.code} · {branch.timezone}</small></div><span class="health good">{branch.active ? 'Healthy' : 'Inactive'}</span></div>{/each}{/if}
          <button class="text-button" type="button" onclick={() => window.location.assign('/context')}>Manage branches <span>→</span></button>
        </article>
      </section>

      {#if conflicts.length > 0}
        <section class="panel conflict-panel" aria-labelledby="conflicts-heading">
          <div class="panel-heading">
            <div><h2 id="conflicts-heading">Synchronization review</h2><p class="muted">Server-authoritative conflicts requiring an operator decision</p></div>
            <span class="health queued">{conflicts.filter((item) => item.status === 'open').length} open</span>
          </div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>Type</th><th>Entity</th><th>Created</th><th>Status</th><th class="align-right">Action</th></tr></thead>
              <tbody>
                {#each conflicts as conflict}
                  <tr>
                    <td>{conflict.entityType}</td>
                    <td>{conflict.entityId.slice(0, 8)}…</td>
                    <td>{new Date(conflict.createdAt).toLocaleString()}</td>
                    <td><span class:health={conflict.status !== 'open'} class:queued={conflict.status === 'open'}>{conflict.status}</span></td>
                    <td class="align-right">
                      {#if conflict.status === 'open'}
                        <button class="table-action" type="button" disabled={conflictBusy === conflict.id} onclick={() => resolveConflict(conflict, 'resolved')}>Resolve</button>
                        <button class="table-action muted-action" type="button" disabled={conflictBusy === conflict.id} onclick={() => resolveConflict(conflict, 'dismissed')}>Dismiss</button>
                      {:else}
                        <span class="muted">Reviewed</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
          {#if conflictMessage}<p class="notice" role="status">{conflictMessage}</p>{/if}
        </section>
      {/if}

      {#if dashboardNotice}<p class="notice" role="status">{dashboardNotice}</p>{/if}

      <div class="parity-banner"><span class="banner-icon">✓</span><div><strong>Parity workspace foundation</strong><span>This shell is the first shared Chrome/Tauri surface. Each existing screen will be captured, implemented, and screenshot-tested before it is marked complete.</span></div><span class="banner-status">Baseline pending</span></div>
    </main>
  </div>
</div>
