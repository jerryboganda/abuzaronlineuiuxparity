<script lang="ts">
  import { onMount } from 'svelte';
  import {
    buildLegacyMenusForContext,
    findShortcutAction,
    type LegacyMenu,
    type LegacyOpenWindow,
    type LegacyWindowContext,
    type MenuAction
  } from '$lib/legacy-menu';
  import { legacyWindowRegistry } from '$lib/legacy-window-registry';

  export let context: LegacyWindowContext = 'base';
  export let windowId = 'main';
  export let windowLabel = 'Main Window';
  export let windowHref = '/app/legacy';
  export let status = 'Ready';
  export let onCommand: ((action: MenuAction) => boolean) | undefined = undefined;

  let openMenu = '';
  let openSubmenu = '';
  let notice = '';

  $: registryWindows = $legacyWindowRegistry.windows;
  $: menus = buildLegacyMenusForContext(context, registryWindows);

  onMount(() => {
    const entry: LegacyOpenWindow = { id: windowId, label: windowLabel, href: windowHref, context };
    legacyWindowRegistry.open(entry);
  });

  function setStatus(value: string) {
    status = value;
    notice = '';
  }

  function toggleMenu(label: string) {
    openMenu = openMenu === label ? '' : label;
    openSubmenu = '';
    const menu = menus.find((entry) => entry.label === label);
    if (openMenu && menu?.actions[0]) setStatus(menu.actions[0].label || 'Ready');
    if (!openMenu) setStatus('Ready');
  }

  function choose(action: MenuAction) {
    setStatus(action.label);
    openMenu = '';
    openSubmenu = '';
    if (onCommand?.(action)) return;
    if (action.windowCommand) {
      if (action.windowCommand === 'activate' && action.windowId) {
        const target = registryWindows.find((window) => window.id === action.windowId);
        if (target) {
          legacyWindowRegistry.activate(target.id);
          window.location.assign(target.href);
        }
        return;
      }
      if (action.windowCommand !== 'activate') legacyWindowRegistry.command(action.windowCommand);
      status = action.windowCommand === 'refresh' ? 'Refresh' : `${action.label} windows`;
      return;
    }
    if (action.implementation === 'not_implemented') {
      notice = `${action.label} is not implemented in this shell slice.`;
      status = notice;
      return;
    }
    if (action.href) window.location.assign(action.href);
  }

  function shortcutAction(shortcut: string): MenuAction | undefined {
    return findShortcutAction(menus, shortcut);
  }

  function phaseFor(windowContext: LegacyWindowContext): string {
    return windowContext === 'base' ? 'C' : 'H';
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
          aria-haspopup={action.children ? 'menu' : undefined}
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
    <button
      type="button"
      role="tab"
      aria-selected={$legacyWindowRegistry.activeId === item.id}
      class:active={$legacyWindowRegistry.activeId === item.id}
      onclick={() => { legacyWindowRegistry.activate(item.id); window.location.assign(item.href); }}
    >{index + 1}. {item.label}</button>
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
