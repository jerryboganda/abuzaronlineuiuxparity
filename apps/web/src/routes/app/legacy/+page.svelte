<script lang="ts">
  import { onMount } from 'svelte';
  import { AbuzarApi } from '$lib/api';
  import LegacyMenuBar from '$lib/LegacyMenuBar.svelte';
  import { formatLegacyTitle } from '$lib/legacy-title';

  const api = new AbuzarApi();
  let status = 'Ready';
  let authenticatedUsername = 'ADMIN';
  let clock = new Date();
  let changeUserOpen = false;
  let changeUserInteractive = false;
  let minimized = false;

  onMount(() => {
    const clockTimer = window.setInterval(() => {
      clock = new Date();
    }, 1000);
    void api.session().then(async (result) => {
      if (!result.authenticated || !result.context) {
        window.location.assign('/login');
        return;
      }
      const context = result.context;
      authenticatedUsername = context.username || 'ADMIN';
    }).catch(() => {
      // The shell remains visible while the session boundary resolves; the API
      // will redirect unauthenticated users on the next navigation.
    });
    return () => window.clearInterval(clockTimer);
  });

  $: title = formatLegacyTitle(authenticatedUsername, clock);

  function enableChangeUserInteractive() {
    changeUserInteractive = true;
  }

  function confirmChangeUser() {
    enableChangeUserInteractive();
    window.location.assign('/login?changeUser=1');
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.ctrlKey && event.altKey && event.key.toLowerCase() === 'm') {
      event.preventDefault();
      window.location.assign('/app/manage/session-monitor');
    }
  }

</script>

<svelte:window onkeydown={handleKeydown} />

<svelte:head>
  <title>WASEELA · Abuzar</title>
</svelte:head>

<main class="legacy-shell-page" data-testid="legacy-shell-page">
  <section class:legacy-shell-minimized={minimized} class="legacy-shell" aria-label="Abuzar legacy main window">
    <header class="legacy-window-titlebar">
      <span class="legacy-window-icon" aria-hidden="true"><i></i><b></b><em></em></span>
      <h1>{title}</h1>
      <div class="legacy-window-controls" aria-label="Window controls">
        <button type="button" aria-label="Minimize" onclick={() => { minimized = true; status = 'Minimize'; }}>−</button>
        <button type="button" aria-label="Restore" onclick={() => { minimized = false; status = 'Restore'; }}>□</button>
        <button type="button" aria-label="Close" onclick={() => { window.location.assign('/'); }}>×</button>
      </div>
    </header>

    {#if !minimized}<LegacyMenuBar bind:status context="base" windowId="main" windowLabel="Main Window" windowHref="/app/legacy" />

    <div class="legacy-workspace-frame">
      <div class="legacy-client-top-chrome" aria-hidden="true"><span></span><span></span><span></span><span></span></div>
      <button class="legacy-client-area" type="button" aria-label="Application workspace"></button>
      <div class="legacy-client-bottom-line" aria-hidden="true"></div>
      <footer class="legacy-statusbar" class:legacy-statusbar-live={status !== 'Ready'} role="status">{status}</footer>
    </div>{:else}<button class="legacy-minimized-strip" type="button" onclick={() => { minimized = false; status = 'Restore'; }}>Restore WASEELA ABUZAR</button>{/if}
  </section>
  {#if changeUserOpen}
    <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) changeUserOpen = false; }}>
      <div class:legacy-change-user-captured={!changeUserInteractive} class="legacy-change-user-dialog" role="alertdialog" aria-modal="true" aria-label="Change User">
        <h2>Change User</h2><p>Are you sure you want to change current user with another?</p>
        <button type="button" onclick={confirmChangeUser} onpointerdown={enableChangeUserInteractive}>Yes</button>
        <button type="button" onclick={() => { enableChangeUserInteractive(); changeUserOpen = false; }} onpointerdown={enableChangeUserInteractive}>No</button>
      </div>
    </div>
  {/if}
</main>
