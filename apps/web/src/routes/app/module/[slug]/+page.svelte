<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { AbuzarApi } from '$lib/api';

  const labels: Record<string, string> = {
    purchases: 'Purchases',
    'purchase-return': 'Purchase Return',
    'opening-purchase': 'Opening Purchase',
    'purchases-loose': 'Purchases (Loose)',
    'purchase-orders': 'Purchase Orders',
    'credit-sale': 'Credit Sale',
    'sale-return': 'Sale Return',
    'open-sale-return': 'Open Sale Return',
    quotation: 'Quotation',
    'refused-sales': 'Refused Sales',
    customer: 'Customer',
    supplier: 'Supplier',
    item: 'Item',
    users: 'Users',
    preferences: 'Preferences'
  };

  let message = '';
  let error = '';
  let busy = false;
  let reference = '';
  let notes = '';
  let savedReference = '';
  let savedNotes = '';
  const api = new AbuzarApi();
  // `$page` is populated after hydration; keep the SSR shell renderable when
  // a catalog leaf is opened directly without a client navigation first.
  $: slug = String($page?.params?.slug ?? 'module');
  $: legacyPath = $page?.url?.searchParams?.get('legacyPath') ?? '';
  $: safeSlug = typeof slug === 'string' && slug.length > 0 ? slug : 'module';
  $: title = labels[safeSlug] ?? safeSlug.split('-').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');

  onMount(() => { void loadState(); });

  async function loadState() {
    try {
      const result = await api.maintenanceState(`module-${safeSlug}`);
      const saved = Object.fromEntries(result.items.map((item) => [item.caption, item.value]));
      savedReference = typeof saved.reference === 'string' ? saved.reference : '';
      savedNotes = typeof saved.notes === 'string' ? saved.notes : '';
      reference = savedReference;
      notes = savedNotes;
    } catch {
      // The empty shell remains usable when the API is not authenticated.
    }
  }

  async function save() {
    busy = true;
    message = '';
    error = '';
    try {
      await api.maintenance(`module-${safeSlug}`, { reference, notes, legacyPath });
      savedReference = reference;
      savedNotes = notes;
      message = `${title} workflow saved in the current tenant scope.`;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : `${title} could not be saved.`;
    } finally {
      busy = false;
    }
  }

  function search() {
    const query = reference.trim().toLowerCase();
    if (!query) {
      message = savedReference ? `Last saved ${title} record loaded.` : 'Enter a reference to search.';
      reference = savedReference;
      notes = savedNotes;
      return;
    }
    if (savedReference.toLowerCase().includes(query)) {
      reference = savedReference;
      notes = savedNotes;
      message = `${title} record found in the current tenant scope.`;
    } else {
      message = `No ${title.toLowerCase()} record matched "${reference}".`;
    }
  }
</script>

<svelte:head><title>Abuzar Next · {title}</title></svelte:head>

<main class="module-page">
  <section class="module-window" aria-label={`${title} module`}>
    <header class="module-titlebar">
      <a class="module-back" href="/app/legacy" aria-label="Back to main window">←</a>
      <span class="module-icon" aria-hidden="true"></span>
      <h1>{title}</h1>
      <span class="module-context">WASEELA · ABUZAR</span>
    </header>
    <div class="module-toolbar" role="toolbar" aria-label="Module actions">
      <button type="button" onclick={save} disabled={busy}>Save</button>
      <button type="button" onclick={() => { reference = ''; notes = ''; message = 'Ready for a new record.'; }}>New</button>
      <button type="button" onclick={search}>Search</button>
    </div>
    <div class="module-body">
      <p class="module-eyebrow">LEGACY WORKFLOW</p>
      <h2>{title}</h2>
      <p class="module-help">This workflow is now reachable from the main menu. Enter the record details below, then save or search.</p>
      <form class="module-form" onsubmit={(event) => { event.preventDefault(); save(); }}>
        <label for="reference">Reference / document number</label>
        <input id="reference" bind:value={reference} placeholder="Enter reference" />
        <label for="notes">Notes</label>
        <textarea id="notes" bind:value={notes} rows="4" placeholder="Optional notes"></textarea>
        <div class="module-actions">
          <button class="primary" type="submit" disabled={busy}>Save</button>
          <a href="/app/legacy">Cancel</a>
        </div>
      </form>
      {#if error}<p class="module-message error" role="alert">{error}</p>{:else if message}<p class="module-message" role="status">{message}</p>{/if}
    </div>
    <footer class="module-status">{title} · Ready</footer>
  </section>
</main>
