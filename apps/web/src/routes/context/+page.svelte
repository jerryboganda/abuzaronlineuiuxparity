<script lang="ts">
  import { onMount } from 'svelte';
  import type { BranchSummary, CounterSummary, SessionResponse } from '@abuzar/contracts';
  import { AbuzarApi, ApiError } from '$lib/api';

  const api = new AbuzarApi();
  let session: SessionResponse['context'] = null;
  let branches: BranchSummary[] = [];
  let counters: CounterSummary[] = [];
  let branchId = '';
  let counterId = '';
  let busy = false;
  let error = '';

  onMount(() => {
    void (async () => {
      try {
        const result = await api.session();
        if (!result.authenticated || !result.context) { window.location.assign('/login'); return; }
        session = result.context;
        branchId = result.context.branchId ?? '';
        counterId = result.context.counterId ?? '';
        branches = (await api.branches()).branches;
        if (branchId) counters = (await api.counters(branchId)).counters;
      } catch (cause) {
        error = cause instanceof Error ? cause.message : 'The operational context could not be loaded.';
      }
    })();
  });

  async function selectBranch(value: string) {
    branchId = value;
    counterId = '';
    counters = [];
    if (!value) return;
    try { counters = (await api.counters(value)).counters; }
    catch (cause) { error = cause instanceof Error ? cause.message : 'Counters could not be loaded.'; }
  }

  async function continueToApplication() {
    busy = true;
    error = '';
    try {
      if (!branchId || !counterId) throw new Error('Select a branch and counter.');
      await api.setContext(branchId, counterId);
      window.location.assign('/app/legacy');
    } catch (cause) {
      error = cause instanceof ApiError ? cause.message : cause instanceof Error ? cause.message : 'The context could not be saved.';
    } finally { busy = false; }
  }
</script>

<svelte:head><title>Abuzar · Operational context</title></svelte:head>
<main class="context-page">
  <section class="context-window" aria-labelledby="context-title">
    <header class="context-titlebar"><span class="legacy-app-icon" aria-hidden="true"><i></i><b></b><em></em></span><h1 id="context-title">Select Branch and Counter</h1></header>
    <div class="context-body">
      <p class="context-eyebrow">{session?.tenantCode ?? 'TENANT'} · OPERATIONS</p>
      <h2>Choose the operational context</h2>
      <p>Every transaction is restricted to the selected branch and counter for this operator.</p>
      <label>Branch<select value={branchId} onchange={(event) => selectBranch(event.currentTarget.value)}><option value="">Select branch</option>{#each branches as branch}<option value={branch.id}>{branch.code} · {branch.name}</option>{/each}</select></label>
      <label>Counter<select bind:value={counterId} disabled={!branchId}><option value="">Select counter</option>{#each counters as counter}<option value={counter.id}>{counter.code} · {counter.name}</option>{/each}</select></label>
      {#if error}<p class="context-error" role="alert">{error}</p>{/if}
      <button class="context-primary" type="button" onclick={continueToApplication} disabled={busy}>Continue</button>
      <a class="context-cancel" href="/login">Cancel</a>
    </div>
  </section>
</main>
