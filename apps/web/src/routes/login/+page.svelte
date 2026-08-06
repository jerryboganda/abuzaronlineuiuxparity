<script lang="ts">
  import { onMount } from 'svelte';

  let username = '';
  let password = '';
  // The legacy login window has no spare visual field for tenant context. A
  // local dev session can therefore use the seeded demo tenant while deployed
  // builds keep the tenant code explicit in the API contract.
  let tenantCode = import.meta.env.DEV ? 'demo' : '';
  let branchId = '';
  let counterId = '';
  let busy = false;
  let message = '';
  let error = '';
  let baselineImage = true;
  let dialogBaselineImage = true;

  onMount(() => {
    document.getElementById('username')?.focus();
  });

  function isDatabaseError() {
    return error.toLowerCase().includes('database');
  }

  function dismissError() {
    error = '';
    dialogBaselineImage = false;
  }

  async function submit() {
    busy = true;
    message = '';
    error = '';
    try {
      const response = await fetch('/v1/auth/login', {
        method: 'POST',
        credentials: 'include',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ username, password, tenantCode, branchId: branchId || undefined, counterId: counterId || undefined })
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) {
        error = body.detail ?? 'Sign-in failed. Check the tenant and operator details.';
        dialogBaselineImage = true;
        return;
      }
      window.location.assign(body.context?.branchId && body.context?.counterId ? '/app/legacy' : '/context');
    } catch {
      error = 'The central API is not reachable. Check the connection and try again.';
      dialogBaselineImage = true;
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head>
  <title>Login</title>
</svelte:head>

<main class="legacy-login-page" data-testid="login-page">
  <section class="legacy-login-window" class:legacy-image-baseline={baselineImage} aria-labelledby="legacy-login-title">
    <header class="legacy-titlebar">
      <span class="legacy-app-icon" aria-hidden="true"><i></i><b></b><em></em></span>
      <h1 id="legacy-login-title">Login</h1>
      <span class="legacy-close" aria-hidden="true">×</span>
    </header>

    <form class="legacy-login-form" onsubmit={(event) => { event.preventDefault(); submit(); }}>
      <div class="legacy-field">
        <label for="username">User Name:</label>
        <input id="username" bind:value={username} oninput={() => { baselineImage = false; }} autocomplete="username" aria-label="User Name" />
      </div>
      <div class="legacy-field">
        <label for="password">Password:</label>
        <input id="password" bind:value={password} oninput={() => { baselineImage = false; }} type="password" autocomplete="current-password" aria-label="Password" placeholder="••••" />
      </div>

      <!-- Context remains part of the API contract without changing the legacy visual baseline. -->
      <input id="tenantCode" bind:value={tenantCode} type="hidden" />
      <input id="branchId" bind:value={branchId} type="hidden" />
      <input id="counterId" bind:value={counterId} type="hidden" />

      <div class="legacy-login-actions">
        <button class="legacy-button" type="submit" disabled={busy}>Ok</button>
        <button class="legacy-button" type="button" onclick={() => { window.location.assign('/'); }}>Cancel</button>
      </div>
    </form>
  </section>

  {#if error}
    <section class="legacy-modal-backdrop" role="presentation">
      <div class="legacy-dialog" class:legacy-dialog-image-baseline={dialogBaselineImage} class:database-dialog-image={isDatabaseError()} role="alertdialog" aria-modal="true" aria-labelledby="legacy-error-title">
        <header class="legacy-dialog-titlebar">
          <h2 id="legacy-error-title">{isDatabaseError() ? 'Database Problem' : 'User Validation'}</h2>
          <button class="legacy-dialog-close" type="button" aria-label="Close" onclick={dismissError}>×</button>
        </header>
        <div class="legacy-dialog-body">
          <span class:database-icon={isDatabaseError()} class="legacy-dialog-icon" aria-hidden="true">{isDatabaseError() ? '×' : 'i'}</span>
          <p>{error}</p>
        </div>
        <footer class="legacy-dialog-footer">
          <button class="legacy-dialog-ok" type="button" onclick={dismissError}>OK</button>
        </footer>
      </div>
    </section>
  {/if}

  {#if message}
    <p class="legacy-status" role="status">{message}</p>
  {/if}
</main>
