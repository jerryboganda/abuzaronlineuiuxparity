<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import type { ReportColumn, ReportDefinition, ReportRow } from '@abuzar/contracts';
  import { AbuzarApi } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';
  import { defaultReportDefinition, exportHook } from '$lib/report-core';
  import { formatLegacyTitle } from '$lib/legacy-title';
  import { localDateString } from '$lib/calendar-date';

  let fromDate = localDateString();
  let toDate = fromDate;
  let filter = '';
  let status = 'Select report arguments and retrieve the report.';
  let retrieved = false;
  let loading = false;
  let showArguments = false;
  let showFormat = false;
  let preview = false;
  let previewZoom = 100;
  let previewPage = 1;
  const previewPageSize = 24;
  let selectableArea = 'DEFAULT AREA';
  let selectedAreas: string[] = [];
  let allAreas = false;
  let cash = true;
  let credit = false;
  let format = 'Standard';
  let interactive = false;
  let dialogInteractive = false;
  let authenticatedUsername = 'ADMIN';
  let clock = new Date();
  let rows: ReportRow[] = [];
  let definition: ReportDefinition = defaultReportDefinition('daily-sales-detail');
  let sortColumn: keyof ReportRow = 'occurredAt';
  let sortDirection: 'asc' | 'desc' = 'desc';
  let reportPage = 1;
  const pageSize = 50;
  let serverHasMore = false;
  let error = '';
  const api = new AbuzarApi();

  $: kind = $page?.params?.kind ?? 'daily-sales-detail';
  $: legacyPath = $page?.url?.searchParams?.get('legacyPath') ?? '';
  $: godownId = $page?.url?.searchParams?.get('godownId') ?? '';
  $: batchNumber = $page?.url?.searchParams?.get('batchNumber') ?? '';
  $: legacyLeaf = String(legacyPath ?? '').split(' > ').at(-1)?.replace(/\t.*$/, '').replace(/&/g, '').trim() ?? '';
  $: if (definition.kind !== kind || (legacyLeaf && definition.title !== legacyLeaf)) {
    definition = defaultReportDefinition(kind, legacyLeaf || undefined, legacyPath);
    format = definition.formats[0]?.name ?? 'Standard';
  }
  $: title = definition.kind === kind ? definition.title : (legacyLeaf || 'Report');

  function enableInteractive() {
    interactive = true;
    if (showArguments || showFormat) dialogInteractive = true;
  }

  onMount(() => {
    const clockTimer = window.setInterval(() => { clock = new Date(); }, 1000);
    void api.session().then((result) => {
      if (result.authenticated && result.context) authenticatedUsername = result.context.username || 'ADMIN';
    }).catch(() => { /* captured title remains available while the session resolves */ });
    if (kind === 'daily-sales-detail') {
      loading = true;
      window.setTimeout(() => { loading = false; showArguments = true; }, 1800);
    }
    return () => window.clearInterval(clockTimer);
  });

  function addArea() {
    if (!selectedAreas.includes(selectableArea)) selectedAreas = [...selectedAreas, selectableArea];
  }

  function removeArea() {
    selectedAreas = selectedAreas.filter((area) => area !== selectableArea);
  }

  function openArguments() {
    showArguments = true;
    showFormat = false;
    dialogInteractive = false;
  }

  function confirmArguments() {
    showArguments = false;
    dialogInteractive = false;
    void retrieve(1);
  }

  function openFormat() {
    showFormat = true;
    showArguments = false;
    dialogInteractive = false;
  }

  async function retrieve(targetPage = 1) {
    loading = false;
    retrieved = false;
    error = '';
    try {
      const response = await api.report(kind, fromDate, toDate, filter, {
        page: targetPage,
        pageSize,
        cash,
        credit,
        areas: selectedAreas,
        allAreas,
        legacyPath,
        godownId,
        batchNumber,
        format
      });
      rows = response.rows;
      definition = response.definition;
      if (response.definition.selectedFormat) format = response.definition.selectedFormat;
      if (!definition.formats.some((option) => option.name === format)) {
        format = definition.formats[0]?.name ?? 'Standard';
      }
      reportPage = response.page || targetPage;
      serverHasMore = response.hasMore;
      retrieved = true;
      status = `${title} retrieved for ${fromDate} through ${toDate}.`;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'The report could not be loaded.';
      status = 'Report retrieval failed.';
    }
  }

  function sortBy(column: string) {
    const key = column as keyof ReportRow;
    if (sortColumn === key) sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    else {
      sortColumn = key;
      sortDirection = key === 'occurredAt' ? 'desc' : 'asc';
    }
    reportPage = 1;
  }

  function movePage(offset: number) {
    const targetPage = reportPage + offset;
    if (targetPage < 1 || (offset > 0 && !serverHasMore)) return;
    void retrieve(targetPage);
  }

  function cellValue(row: ReportRow, column: ReportColumn): string {
    return String(row[column.key as keyof ReportRow] ?? '');
  }

  $: sortedRows = [...rows].sort((left, right) => {
    const a = String(left[sortColumn] ?? '');
    const b = String(right[sortColumn] ?? '');
    const compared = a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' });
    return sortDirection === 'asc' ? compared : -compared;
  });
  $: pageCount = serverHasMore ? reportPage + 1 : Math.max(1, reportPage);
  $: visibleRows = sortedRows;
  $: previewPageCount = Math.max(1, Math.ceil(visibleRows.length / previewPageSize));
  $: previewVisibleRows = visibleRows.slice((previewPage - 1) * previewPageSize, previewPage * previewPageSize);

  function openPreview() {
    preview = true;
    previewPage = 1;
    previewZoom = 100;
    status = 'Print preview is ready.';
  }

  function movePreviewPage(offset: number) {
    previewPage = Math.max(1, Math.min(previewPageCount, previewPage + offset));
  }

  function setPreviewZoom(offset: number) {
    previewZoom = Math.max(50, Math.min(200, previewZoom + offset));
  }

  function confirmFormat() {
    showFormat = false;
    dialogInteractive = false;
    status = `${format} selected for ${title}.`;
    if (retrieved) void retrieve(1);
  }

  function printReport() {
    openPreview();
    if (typeof window !== 'undefined') window.setTimeout(() => window.print(), 0);
  }

  function saveLayout() {
    if (typeof window !== 'undefined') window.localStorage.setItem(`abuzar.report.${kind}`, JSON.stringify({ fromDate, toDate, filter, format, sortColumn, sortDirection }));
    status = 'Report layout saved.';
  }

  function exportCsv() {
    const header = definition.columns.map((column) => column.label);
    const csv = [header, ...rows.map((row) => definition.columns.map((column) => cellValue(row, column)))]
      .map((line) => line.map((cell) => `"${String(cell ?? '').replace(/"/g, '""')}"`).join(','))
      .join('\r\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${kind}-${fromDate}-${toDate}.csv`;
    link.click();
    URL.revokeObjectURL(url);
    status = 'CSV export is ready.';
  }

  function exportPdf() {
    openPreview();
    status = exportHook(definition, 'pdf')?.message ?? 'PDF print preview is ready.';
  }

  function exportExcel() {
    const header = definition.columns.map((column) => column.label);
    const tableRows = [header, ...rows.map((row) => definition.columns.map((column) => cellValue(row, column)))];
    const table = tableRows.map((line) => `<tr>${line.map((cell) => `<td>${String(cell ?? '').replace(/[&<>\"]/g, (value) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '\"': '&quot;' }[value] ?? value))}</td>`).join('')}</tr>`).join('');
    const workbook = `<!doctype html><html><head><meta charset="utf-8"></head><body><h1>${title}</h1><p>${fromDate} to ${toDate}</p><table>${table}</table></body></html>`;
    const blob = new Blob([workbook], { type: 'application/vnd.ms-excel;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${kind}-${fromDate}-${toDate}.xls`;
    link.click();
    URL.revokeObjectURL(url);
    status = exportHook(definition, 'excel')?.message ?? 'Excel export is ready.';
  }
</script>

<svelte:window onkeydown={enableInteractive} />

<svelte:head><title>WASEELA · ABUZAR V3 · {title}</title></svelte:head>
<main class:legacy-report-loading-baseline={kind === 'daily-sales-detail' && loading && !interactive} class="legacy-report-page" onpointerdown={enableInteractive} onfocusin={enableInteractive}><section class="legacy-report-window" aria-label={title}>
  <header class="legacy-transaction-titlebar"><a href="/app/legacy" aria-label="Back to main window">←</a><h1>{formatLegacyTitle(authenticatedUsername, clock)} : [{title}]</h1></header>
  <LegacyMenuBar context="report-sale-detail" windowId={'report-' + kind} windowLabel={title} windowHref={'/app/report/' + kind} />
  <div class="legacy-transaction-toolbar legacy-report-toolbar" role="toolbar" aria-label="Report toolbar">
    <button type="button" aria-label="Save report" onclick={saveLayout} title="Save">▧</button>
    <button type="button" aria-label="Run report" onclick={openArguments} title="Retrieve">♟</button>
    <button type="button" aria-label="Select report arguments" onclick={openArguments} title="Arguments">◈</button>
    <button type="button" aria-label="Sort report" onclick={() => sortBy('document')} title="Sort">↕</button>
    <button type="button" aria-label="Open report" onclick={openArguments} title="Open">▤</button>
    <button type="button" aria-label="Previous report" onclick={() => movePage(-1)} title="Previous">◀</button>
    <button type="button" aria-label="Next report" onclick={() => movePage(1)} title="Next">▶</button>
    <button type="button" aria-label="First report" onclick={() => { reportPage = 1; }} title="First">|◀</button>
    <button type="button" aria-label="Last report" onclick={() => { if (!serverHasMore) reportPage = pageCount; }} disabled={serverHasMore} title={serverHasMore ? 'The server has not reported the last page' : 'Last'}>▶|</button>
    <button type="button" aria-label="Print report" onclick={printReport} title="Print">▥</button>
    <button type="button" aria-label="Export report" onclick={exportCsv} title="Export CSV">⇩</button>
    <button type="button" aria-label="Export report as PDF" onclick={exportPdf} disabled={exportHook(definition, 'pdf')?.status !== 'available'} title={exportHook(definition, 'pdf')?.message}>PDF</button>
    <button type="button" aria-label="Export report as Excel" onclick={exportExcel} disabled={exportHook(definition, 'excel')?.status !== 'available'} title={exportHook(definition, 'excel')?.message}>XLS</button>
    <button type="button" aria-label="Preview report" onclick={openPreview} title="Preview">▣</button>
    <button type="button" aria-label="Refresh report" onclick={() => { void retrieve(); }} title="Refresh">⟳</button>
    <button type="button" aria-label="Report settings" onclick={openFormat} title="Settings">⚙</button>
    <button type="button" aria-label="Report help" onclick={() => { status = 'Select arguments, then retrieve the report.'; }} title="Help">?</button>
  </div>
  {#if loading}<div class="legacy-report-loading" role="status" aria-live="polite">Preparing Your<br />Report to Display,<br />Please Wait . . .</div>{:else}<div class="legacy-report-body">
    <div class="legacy-report-arguments">
      <h2>{title}</h2>
      <label>From Date:<input type="date" bind:value={fromDate} /></label>
      <label>To Date:<input type="date" bind:value={toDate} /></label>
      <label>Filter:<input bind:value={filter} placeholder="Optional filter" /></label>
      <button type="button" onclick={openArguments}>Retrieve</button>
      {#if definition.projectionNote}<small class="legacy-report-fallback-note">{definition.projectionNote}</small>{:else if definition.projectionStatus === 'generic-fallback'}<small class="legacy-report-fallback-note">Generic event-ledger fallback; exact legacy projection is not implemented.</small>{/if}
    </div>
    {#if error}<p class="legacy-report-error" role="alert">{error}</p>{/if}
    <div class="legacy-report-grid-wrap"><table class="legacy-report-grid"><thead><tr>{#each definition.columns as column}<th><button type="button" onclick={() => sortBy(column.key)}>{column.label} {sortColumn === column.key ? (sortDirection === 'asc' ? '↑' : '↓') : ''}</button></th>{/each}</tr></thead><tbody>{#if retrieved && visibleRows.length > 0}{#each visibleRows as row}<tr>{#each definition.columns as column}<td>{cellValue(row, column)}</td>{/each}</tr>{/each}{:else if retrieved}<tr><td colspan={definition.columns.length}>No rows match the selected scope.</td></tr>{:else}<tr><td colspan={definition.columns.length}>Report results appear here after Retrieve.</td></tr>{/if}</tbody></table></div>
    {#if retrieved}<div class="legacy-report-pagination" role="navigation" aria-label="Report pages"><span>Rows {rows.length === 0 ? 0 : ((reportPage - 1) * pageSize) + 1}–{((reportPage - 1) * pageSize) + rows.length}{serverHasMore ? ' · More pages' : ''}</span><button type="button" onclick={() => { if (reportPage > 1) void retrieve(1); }} disabled={reportPage === 1}>|◀</button><button type="button" onclick={() => movePage(-1)} disabled={reportPage === 1}>◀</button><span>Page {reportPage}{serverHasMore ? ` of at least ${pageCount}` : ` of ${pageCount}`}</span><button type="button" onclick={() => movePage(1)} disabled={!serverHasMore}>▶</button><button type="button" onclick={() => { if (!serverHasMore) reportPage = pageCount; }} disabled={serverHasMore}>▶|</button></div>{/if}
  </div>{/if}
  {#if showArguments}<div class="legacy-report-dialog-backdrop" role="presentation"><div onpointerdown={() => { dialogInteractive = true; }} class:legacy-report-dialog-captured={!dialogInteractive} class="legacy-report-dialog" role="dialog" aria-modal="true" aria-label={definition.retrieval.title} tabindex="-1"><h2>{definition.retrieval.title}</h2><fieldset><legend>Selection List</legend><div class="legacy-report-selection-columns"><div><label for="selectable-area">Selectable Areas</label><select id="selectable-area" size="11" bind:value={selectableArea}>{#each definition.retrieval.areas as area}<option>{area}</option>{/each}</select></div><div><label for="selected-areas">Selected Areas</label><select id="selected-areas" size="11" multiple value={selectedAreas}>{#each selectedAreas as area}<option>{area}</option>{/each}</select></div></div><div class="legacy-report-selection-actions"><button type="button" onclick={addArea}>Add</button><button type="button" onclick={removeArea}>Remove</button><label for="all-areas"><input id="all-areas" type="checkbox" bind:checked={allAreas} /> All</label></div></fieldset><fieldset><legend>Date</legend><label for="report-start-date">Start Date:<input id="report-start-date" type="date" bind:value={fromDate} /></label><label for="report-end-date">End Date:<input id="report-end-date" type="date" bind:value={toDate} /></label></fieldset>{#if definition.retrieval.supportsCashCredit}<div class="legacy-report-checks"><label for="report-cash"><input id="report-cash" type="checkbox" bind:checked={cash} /> Cash</label><label for="report-credit"><input id="report-credit" type="checkbox" bind:checked={credit} /> Credit</label></div>{/if}<div class="legacy-report-dialog-actions"><button type="button" onclick={confirmArguments}>Ok</button><button type="button" onclick={() => { showArguments = false; dialogInteractive = false; }}>Cancel</button></div></div></div>{/if}
  {#if showFormat}<div class="legacy-report-dialog-backdrop" role="presentation"><div onpointerdown={() => { dialogInteractive = true; }} class:legacy-report-format-captured={!dialogInteractive} class="legacy-report-format-dialog" role="dialog" aria-modal="true" aria-label="Select Format" tabindex="-1"><h2>Select Format</h2><fieldset><legend>Format</legend>{#each definition.formats as option, index}<label><input type="radio" name="report-format" value={option.name} bind:group={format} />{index + 1}) {option.name}</label>{/each}</fieldset><button type="button" onclick={confirmFormat}>Ok</button></div></div>{/if}
  {#if preview}<div class="legacy-report-preview-backdrop" role="presentation"><div class="legacy-report-preview" role="dialog" aria-modal="true" aria-label="Print preview">
    <header><div><strong>{title}</strong><small>{format}</small></div><button type="button" aria-label="Close print preview" onclick={() => { preview = false; }}>×</button></header>
    <div class="legacy-report-preview-toolbar" role="toolbar" aria-label="Print preview toolbar">
      <button type="button" aria-label="Preview first page" onclick={() => { previewPage = 1; }} disabled={previewPage === 1}>First</button>
      <button type="button" aria-label="Preview previous page" onclick={() => movePreviewPage(-1)} disabled={previewPage === 1}>Prev</button>
      <span aria-live="polite">Preview page {previewPage} of {previewPageCount}</span>
      <button type="button" aria-label="Preview next page" onclick={() => movePreviewPage(1)} disabled={previewPage === previewPageCount}>Next</button>
      <button type="button" aria-label="Preview last page" onclick={() => { previewPage = previewPageCount; }} disabled={previewPage === previewPageCount}>Last</button>
      <span class="legacy-report-preview-toolbar-separator" aria-hidden="true"></span>
      <button type="button" aria-label="Zoom out preview" onclick={() => setPreviewZoom(-10)} disabled={previewZoom <= 50}>-</button>
      <span>{previewZoom}%</span>
      <button type="button" aria-label="Zoom in preview" onclick={() => setPreviewZoom(10)} disabled={previewZoom >= 200}>+</button>
      <button type="button" aria-label="Print preview report" onclick={() => { if (typeof window !== 'undefined') window.print(); }}>Print</button>
      <button type="button" aria-label="Close print preview toolbar" onclick={() => { preview = false; }}>Close</button>
    </div>
    <div class="legacy-report-preview-ruler" aria-hidden="true"><span>0</span><span>1</span><span>2</span><span>3</span><span>4</span><span>5</span><span>6</span><span>7</span><span>8</span><span>9</span><span>10</span><span>11</span><span>12</span></div>
    <div class="legacy-report-preview-status">Report page {reportPage}{serverHasMore ? ' · More records available' : ''} · {fromDate} to {toDate}</div>
    <div class="legacy-report-preview-workspace"><div class="legacy-report-preview-page-wrap" style={`--preview-scale: ${previewZoom / 100}`}><article class="legacy-report-preview-page">
      <div class="legacy-report-letterhead"><strong>{definition.letterhead.name}</strong><span>{definition.letterhead.line2} / {definition.letterhead.line3}</span><span>Phone(s): {definition.letterhead.phone}{#if definition.letterhead.fax} · Fax: {definition.letterhead.fax}{/if}</span></div>
      <div class="legacy-report-preview-meta"><span>{title} · {format}</span><span>Page {reportPage} / {pageCount} · Preview {previewPage} / {previewPageCount}</span></div>
      <table class="legacy-report-grid"><thead><tr>{#each definition.columns as column}<th>{column.label}</th>{/each}</tr></thead><tbody>{#if previewVisibleRows.length > 0}{#each previewVisibleRows as row}<tr>{#each definition.columns as column}<td>{cellValue(row, column)}</td>{/each}</tr>{/each}{:else}<tr><td colspan={definition.columns.length}>No rows loaded for this report page.</td></tr>{/if}</tbody></table>
    </article></div></div>
  </div></div>{/if}
  <footer class="legacy-transaction-footer"><span role="status">{status}</span><a href="/app/legacy">Back to main window</a></footer>
</section></main>
