import { writable, type Readable } from 'svelte/store';
import type { LegacyOpenWindow } from '$lib/legacy-menu';

export type WindowLayout = 'cascade' | 'tile' | 'layer' | 'arrange';
export type LegacyWindowRegistry = {
  windows: LegacyOpenWindow[];
  activeId: string;
  layout: WindowLayout;
  refreshToken: number;
};

export function createLegacyWindowRegistry(): {
  subscribe: Readable<LegacyWindowRegistry>['subscribe'];
  open: (window: LegacyOpenWindow) => void;
  activate: (id: string) => void;
  command: (command: 'cascade' | 'tile' | 'layer' | 'arrange' | 'refresh') => void;
  snapshot: () => LegacyWindowRegistry;
} {
  const store = writable<LegacyWindowRegistry>({ windows: [], activeId: '', layout: 'arrange', refreshToken: 0 });
  let current: LegacyWindowRegistry = { windows: [], activeId: '', layout: 'arrange', refreshToken: 0 };
  store.subscribe((value) => { current = value; });
  return {
    subscribe: store.subscribe,
    open: (window) => store.update((state) => ({
      ...state,
      windows: state.windows.some((item) => item.id === window.id) ? state.windows : [...state.windows, window],
      activeId: window.id
    })),
    activate: (id) => store.update((state) => state.windows.some((window) => window.id === id) ? { ...state, activeId: id } : state),
    command: (command) => store.update((state) => command === 'refresh'
      ? { ...state, refreshToken: state.refreshToken + 1 }
      : { ...state, layout: command }),
    snapshot: () => current
  };
}

export const legacyWindowRegistry = createLegacyWindowRegistry();
