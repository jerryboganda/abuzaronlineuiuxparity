<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import {
    buildLegacyMenusForContext,
    findShortcutAction,
    type LegacyMenu,
    type LegacyOpenWindow,
    type LegacyWindowContext,
    type MenuAction,
    type MenuAccess,
    applyMenuAccess
  } from '$lib/legacy-menu';
  import { legacyWindowRegistry } from '$lib/legacy-window-registry';
  import { AbuzarApi } from '$lib/api';

  export let context: LegacyWindowContext = 'base';
  export let windowId = 'main';
  export let windowLabel = 'Main Window';
  export let windowHref = '/app/legacy';
  export let status = 'Ready';
  export let onCommand: ((action: MenuAction) => boolean) | undefined = undefined;

  let openMenu = '';
  let openSubmenu = '';
  let notice = '';
  let changeUserOpen = false;
  let changeUserInteractive = false;
  let menuAccess: MenuAccess = { tenantAdmin: false, permissions: [], scopes: {}, loaded: false };
  const api = new AbuzarApi();

  $: registryWindows = $legacyWindowRegistry.windows;
  $: menus = applyMenuAccess(buildLegacyMenusForContext(context, registryWindows), menuAccess);

  onMount(() => {
    const entry: LegacyOpenWindow = { id: windowId, label: windowLabel, href: windowHref, context };
    legacyWindowRegistry.open(entry);
    void api.access().then((access) => {
      menuAccess = {
        tenantAdmin: access.tenantAdmin,
        permissions: access.permissions ?? [],
        scopes: access.scopes ?? {},
        loaded: true
      };
    }).catch(() => {
      // Keep the menu disabled until the guarded access response is available.
      menuAccess = { tenantAdmin: false, permissions: [], scopes: {}, loaded: false };
      notice = 'Access rights are unavailable; commands remain disabled.';
    });
  });

  function setStatus(value: string) {
    status = value;
    notice = '';
  }

  function enableChangeUserInteractive() {
    changeUserInteractive = true;
  }

  async function confirmChangeUser() {
    enableChangeUserInteractive();
    legacyWindowRegistry.clear();
    await api.logout().catch(() => undefined);
    window.location.assign('/login?changeUser=1');
  }

  function toggleMenu(label: string) {
    openMenu = openMenu === label ? '' : label;
    openSubmenu = '';
    const menu = menus.find((entry) => entry.label === label);
    if (openMenu && menu?.actions[0]) setStatus(menu.actions[0].label || 'Ready');
    if (!openMenu) setStatus('Ready');
  }

  function choose(action: MenuAction) {
    if (action.denied) {
      notice = action.mappingStatus === 'ambiguous'
        ? `${action.label} has no unambiguous legacy-right mapping and remains disabled.`
        : `${action.label} is not allowed for the current operator.`;
      status = 'Access denied';
      return;
    }
    setStatus(action.label);
    openMenu = '';
    openSubmenu = '';
    if (action.label === 'Change User') {
      changeUserOpen = true;
      changeUserInteractive = false;
      return;
    }
    if (onCommand?.(action)) return;
    if (action.windowCommand) {
      if (action.windowCommand === 'activate' && action.windowId) {
        const target = registryWindows.find((window) => window.id === action.windowId);
        if (target) {
          legacyWindowRegistry.activate(target.id);
          navigate(target.href);
        }
        return;
      }
      if (action.windowCommand !== 'activate') legacyWindowRegistry.command(action.windowCommand);
      status = action.windowCommand === 'refresh' ? 'Refresh' : `${action.label} windows`;
      return;
    }
    if (action.implementation === 'not_implemented') {
      // Every captured leaf still has a deterministic destination. Navigate to
      // the contextual workbench instead of leaving the menu click inert; the
      // workbench preserves the legacy path/command id so the workflow can be
      // implemented and audited without losing the user's selected command.
      if (action.href) {
        navigate(action.href);
        return;
      }
      notice = `${action.label} is not wired to a destination yet.`;
      status = notice;
      return;
    }
    if (action.href) navigate(action.href);
  }

  function shortcutAction(shortcut: string): MenuAction | undefined {
    return findShortcutAction(menus, shortcut);
  }

  function phaseFor(windowContext: LegacyWindowContext): string {
    return windowContext === 'base' ? 'C' : 'H';
  }

  function navigate(href: string) {
    void goto(href).catch(() => {
      // Keep the command usable if a client-side route cannot be loaded.
      window.location.assign(href);
    });
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      openMenu = '';
      openSubmenu = '';
      notice = '';
      setStatus('Ready');
      return;
    }
    const modifiers = `${event.ctrlKey ? 'Ctrl+' : ''}${event.altKey ? 'Alt+' : ''}${event.shiftKey ? 'Shift+' : ''}${event.key.toUpperCase()}`;
    const action = shortcutAction(modifiers);
    if (action && !action.children) {
      event.preventDefault();
      choose(action);
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#snippet renderActions(actions: MenuAction[])}
  {#each actions as action}
    <div class="legacy-menu-item-wrap">
      {#if action.separator}
        <div class="legacy-menu-separator" role="separator"></div>
      {:else}
        <button
          type="button"
          role="menuitem"
          data-command-id={action.commandId ?? undefined}
          data-legacy-path={action.legacyPath ?? action.label}
          data-mapping-status={action.mappingStatus ?? undefined}
          aria-haspopup={action.children ? 'menu' : undefined}
          aria-disabled={action.denied ? 'true' : undefined}
          disabled={action.denied}
          title={action.denied ? (action.mappingStatus === 'ambiguous' ? 'No unambiguous legacy-right mapping' : 'Permission denied') : action.mappingStatus === 'ambiguous' ? 'Legacy command mapping is an exception; exact right code not claimed' : undefined}
          aria-expanded={action.children ? (openSubmenu === action.key || openSubmenu.startsWith(`${action.key} > `)) : undefined}
          onmouseenter={() => { setStatus(action.label); if (action.children) openSubmenu = action.key; }}
          onfocus={() => { setStatus(action.label); if (action.children) openSubmenu = action.key; }}
          onclick={() => action.children ? (openSubmenu = action.key) : choose(action)}
        >{action.label}{#if action.shortcut}<span class="legacy-menu-shortcut" aria-hidden="true">{action.shortcut}</span>{/if}{#if action.children}<span class="legacy-menu-arrow" aria-hidden="true">›</span>{/if}</button>
        {#if action.children && (openSubmenu === action.key || openSubmenu.startsWith(`${action.key} > `))}
          <div class="legacy-menu-subdropdown" role="menu" aria-label={action.label}>
            {@render renderActions(action.children)}
          </div>
        {/if}
      {/if}
    </div>
  {/each}
{/snippet}

<nav class="legacy-menu-bar legacy-contextual-menu-bar" aria-label="Application menu">
  {#each menus as menu}
    <div class="legacy-menu-group">
      <button
        class="legacy-menu-button"
        type="button"
        aria-haspopup="menu"
        aria-expanded={openMenu === menu.label}
        onclick={() => toggleMenu(menu.label)}
      >{menu.label}</button>
      {#if openMenu === menu.label}
        <div class="legacy-menu-dropdown" role="menu" aria-label={menu.label}>
          {@render renderActions(menu.actions)}
        </div>
      {/if}
    </div>
  {/each}
</nav>

<div class="legacy-mdi-tabs" data-layout={$legacyWindowRegistry.layout} role="tablist" aria-label="Open document windows">
  {#each registryWindows as item, index}
    <div class="legacy-mdi-tab-item" class:active={$legacyWindowRegistry.activeId === item.id}>
      <button
        type="button"
        role="tab"
        aria-selected={$legacyWindowRegistry.activeId === item.id}
        class:active={$legacyWindowRegistry.activeId === item.id}
        onclick={() => { legacyWindowRegistry.activate(item.id); navigate(item.href); }}
      >{index + 1}. {item.label}</button>
      <button
        type="button"
        class="legacy-mdi-tab-close"
        aria-label={`Close ${item.label}`}
        title={`Close ${item.label}`}
        onclick={(event) => { event.stopPropagation(); legacyWindowRegistry.close(item.id); }}
      >×</button>
    </div>
  {/each}
</div>

<div class="legacy-contextual-status" role={context === 'base' ? undefined : 'status'} aria-live="polite">
  <span>{status}</span>
  {#if notice}
  <span class="legacy-unimplemented-note" data-testid="legacy-unimplemented-note">
    {notice} <a href="/app?phase={phaseFor(context)}">See Phase {phaseFor(context)}</a>
  </span>
  {/if}
</div>

{#if changeUserOpen}
  <div class="legacy-shell-modal-backdrop" role="presentation" onclick={(event) => { if (event.currentTarget === event.target) changeUserOpen = false; }}>
    <div class:legacy-change-user-captured={!changeUserInteractive} class="legacy-change-user-dialog" role="alertdialog" aria-modal="true" aria-label="Change User">
      <h2>Change User</h2><p>Are you sure you want to change current user with another?</p>
      <button type="button" onclick={confirmChangeUser} onpointerdown={enableChangeUserInteractive}>Yes</button>
      <button type="button" onclick={() => { enableChangeUserInteractive(); changeUserOpen = false; }} onpointerdown={enableChangeUserInteractive}>No</button>
    </div>
  </div>
{/if}
