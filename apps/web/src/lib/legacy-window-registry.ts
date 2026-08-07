import { writable, type Readable } from 'svelte/store';
import type { LegacyOpenWindow } from '$lib/legacy-menu';

export type WindowLayout = 'cascade' | 'tile' | 'layer' | 'arrange';
export type LegacyWindowRegistry = {
  windows: LegacyOpenWindow[];
  activeId: string;
  layout: WindowLayout;
  refreshToken: number;
};

const legacyWindowRegistryStorageKey = 'abuzar.legacy-window-registry.v1';
const legacyWindowContexts = new Set<string>([
  'base',
  'pack-purchase',
  'purchase-pack',
  'purchase-return',
  'purchase-opening',
  'purchase-loose',
  'purchase-order',
  'cash-sale',
  'credit-sale',
  'cash-return',
  'credit-return',
  'open-cash-return',
  'open-credit-return',
  'quotation',
  'refused',
  'refused-sales',
  'item-master',
  'customer-master',
  'supplier-master',
  'manufacturer-master',
  'user-master',
  'master-customer',
  'master-supplier',
  'master-item',
  'master-manufacturer',
  'master-user',
  'report-sale-detail',
  'manage-groups',
  'manage-users',
  'preferences'
]);
const legacyWindowLayouts = new Set<WindowLayout>(['cascade', 'tile', 'layer', 'arrange']);

function emptyRegistry(): LegacyWindowRegistry {
  return { windows: [], activeId: '', layout: 'arrange', refreshToken: 0 };
}

function isStoredWindow(value: unknown): value is LegacyOpenWindow {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<LegacyOpenWindow>;
  return typeof candidate.id === 'string'
    && candidate.id.length > 0
    && candidate.id.length <= 160
    && typeof candidate.label === 'string'
    && candidate.label.length <= 240
    && typeof candidate.href === 'string'
    && candidate.href.startsWith('/')
    && typeof candidate.context === 'string'
    && candidate.context.trim().length > 0
    && (legacyWindowContexts.has(candidate.context) || /^[a-z0-9-]+$/i.test(candidate.context));
}

function restoreRegistry(): LegacyWindowRegistry {
  if (typeof window === 'undefined') return emptyRegistry();
  try {
    const raw = window.sessionStorage.getItem(legacyWindowRegistryStorageKey);
    if (!raw) return emptyRegistry();
    const parsed = JSON.parse(raw) as Partial<LegacyWindowRegistry>;
    const windows = Array.isArray(parsed.windows)
      ? parsed.windows.filter(isStoredWindow).slice(0, 32)
      : [];
    const activeId = typeof parsed.activeId === 'string' && windows.some((item) => item.id === parsed.activeId)
      ? parsed.activeId
      : windows.at(-1)?.id ?? '';
    const layout = typeof parsed.layout === 'string' && legacyWindowLayouts.has(parsed.layout as WindowLayout)
      ? parsed.layout as WindowLayout
      : 'arrange';
    return { windows, activeId, layout, refreshToken: 0 };
  } catch {
    return emptyRegistry();
  }
}

function persistRegistry(value: LegacyWindowRegistry): void {
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.setItem(legacyWindowRegistryStorageKey, JSON.stringify({
      windows: value.windows,
      activeId: value.activeId,
      layout: value.layout
    }));
  } catch {
    // Private browsing and storage quotas must not interrupt the legacy shell.
  }
}

export function createLegacyWindowRegistry(options: { persist?: boolean } = {}): {
  subscribe: Readable<LegacyWindowRegistry>['subscribe'];
  open: (window: LegacyOpenWindow) => void;
  activate: (id: string) => void;
  close: (id: string) => void;
  command: (command: 'cascade' | 'tile' | 'layer' | 'arrange' | 'refresh') => void;
  clear: () => void;
  snapshot: () => LegacyWindowRegistry;
} {
  const persist = options.persist === true;
  const initial = persist ? restoreRegistry() : emptyRegistry();
  const store = writable<LegacyWindowRegistry>(initial);
  let current: LegacyWindowRegistry = initial;
  store.subscribe((value) => {
    current = value;
    if (persist) persistRegistry(value);
  });
  return {
    subscribe: store.subscribe,
    open: (window) => store.update((state) => ({
      ...state,
      windows: state.windows.some((item) => item.id === window.id) ? state.windows : [...state.windows, window],
      activeId: window.id
    })),
    activate: (id) => store.update((state) => state.windows.some((window) => window.id === id) ? { ...state, activeId: id } : state),
    close: (id) => store.update((state) => {
      const windows = state.windows.filter((window) => window.id !== id);
      const activeId = state.activeId === id
        ? (windows.at(-1)?.id ?? '')
        : state.activeId;
      return { ...state, windows, activeId };
    }),
    command: (command) => store.update((state) => command === 'refresh'
      ? { ...state, refreshToken: state.refreshToken + 1 }
      : { ...state, layout: command }),
    clear: () => store.set(emptyRegistry()),
    snapshot: () => current
  };
}

export const legacyWindowRegistry = createLegacyWindowRegistry({ persist: true });
