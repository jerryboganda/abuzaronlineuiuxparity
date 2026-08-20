<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import type { MasterRecord } from '@abuzar/contracts';
  import { AbuzarApi, ApiError } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';
  import LegacyWorkflowSurface from '$lib/LegacyWorkflowSurface.svelte';

  const api = new AbuzarApi();
  let kind = '';
  let godowns: MasterRecord[] = [];
  let items: MasterRecord[] = [];
  let sourceGodownId = '';
  let destinationGodownId = '';
  let itemId = '';
  let quantity = '1';
  let batchNumber = '';
  let message = '';
  let error = '';
  let busy = false;

  let mounted = false;
  let lookupsLoadedFor = '';

  $: kind = $page.params.kind ?? '';
  $: isTransfer = kind === 'godown-transfer';
  $: if (!isTransfer) lookupsLoadedFor = '';
  $: if (mounted && isTransfer && lookupsLoadedFor !== kind) {
    lookupsLoadedFor = kind;
    void loadTransferLookups();
  }

  onMount(() => {
    mounted = true;
  });

  async function loadTransferLookups() {
    error = '';
    try {
      const [godownPage, itemPage] = await Promise.all([
        api.masterRecords('godown'),
        api.masterRecords('item')
      ]);
      godowns = godownPage.records ?? [];
      items = (itemPage.records ?? []).slice(0, 200);
      if (!sourceGodownId) sourceGodownId = godowns[0]?.id ?? '';
      if (!destinationGodownId) destinationGodownId = godowns[1]?.id ?? godowns[0]?.id ?? '';
      if (!itemId) itemId = items[0]?.id ?? '';
    } catch (loadError) {
      error = loadError instanceof ApiError ? loadError.message : 'Unable to load godowns and items.';
    }
  }

  async function postTransfer() {
    error = '';
    message = '';
    busy = true;
    try {
      const result = await api.maintenance('godown-transfer', {
        sourceGodownId,
        destinationGodownId,
        occurredAt: new Date().toISOString(),
        lines: [{ itemId, quantity, batchNumber: batchNumber || undefined }]
      });
      message = result.message || `Transferred ${result.status}.`;
    } catch (postError) {
      error = postError instanceof ApiError ? postError.message : 'Godown transfer failed.';
    } finally {
      busy = false;
    }
  }
</script>

{#if !isTransfer}
  <LegacyWorkflowSurface section="maintenance" />
{:else}
  <section class="space-y-4 p-4">
    <LegacyMenuBar />
    <h1 class="text-lg font-semibold">Godown Transfer</h1>
    <p class="text-sm text-slate-600">Move on-hand stock from one godown to another. Quantity is taken FIFO unless a batch number is supplied.</p>
    {#if error}<p class="text-sm text-red-700">{error}</p>{/if}
    {#if message}<p class="text-sm text-emerald-700">{message}</p>{/if}
    <form class="grid max-w-xl gap-3" on:submit|preventDefault={postTransfer}>
      <label class="grid gap-1 text-sm">
        Source godown
        <select bind:value={sourceGodownId} required>
          {#each godowns as godown}
            <option value={godown.id}>{godown.code} — {godown.name}</option>
          {/each}
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        Destination godown
        <select bind:value={destinationGodownId} required>
          {#each godowns as godown}
            <option value={godown.id}>{godown.code} — {godown.name}</option>
          {/each}
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        Item
        <select bind:value={itemId} required>
          {#each items as item}
            <option value={item.id}>{item.code} — {item.name}</option>
          {/each}
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        Quantity
        <input type="number" min="0.0001" step="0.0001" bind:value={quantity} required />
      </label>
      <label class="grid gap-1 text-sm">
        Batch number (optional)
        <input type="text" bind:value={batchNumber} />
      </label>
      <button type="submit" disabled={busy} class="w-fit rounded bg-slate-800 px-3 py-1.5 text-sm text-white disabled:opacity-50">
        {busy ? 'Posting…' : 'Post transfer'}
      </button>
    </form>
  </section>
{/if}

