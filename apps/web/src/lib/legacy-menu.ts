import { legacyMenuCatalog, type LegacyMenuCatalogItem } from '$lib/legacy-menu-catalog';
import { contextualLegacyMenuCatalog } from '$lib/legacy-menu-contextual-catalog';

export type LegacyWindowContext = 'base' | 'pack-purchase' | 'cash-sale' | 'item-master' | 'report-sale-detail' | 'manage-groups';

export type LegacyOpenWindow = {
  id: string;
  label: string;
  href: string;
  context: LegacyWindowContext;
};

export type MenuAction = {
  label: string;
  key: string;
  shortcut?: string;
  separator?: boolean;
  href?: string;
  children?: MenuAction[];
  commandId?: number;
  legacyPath?: string;
  implementation?: 'implemented' | 'not_implemented' | 'shell';
  phase?: string;
  windowCommand?: 'cascade' | 'tile' | 'layer' | 'arrange' | 'refresh' | 'activate';
  windowId?: string;
  requiredPermission?: string;
  requiredScope?: { kind: string; key: string };
  mappingStatus?: 'unambiguous' | 'ambiguous';
  denied?: boolean;
};

export type LegacyMenu = { label: string; actions: MenuAction[] };

type ContextualCatalog = {
  schemaVersion: number;
  catalogVersion: string;
  contexts: Array<{
    windowType: Exclude<LegacyWindowContext, 'base'>;
    items: LegacyMenuCatalogItem[];
  }>;
};

const contextualMenus = contextualLegacyMenuCatalog as unknown as ContextualCatalog;

export function cleanLabel(value: string): string {
  return value.replace(/\t.*$/, '').replace(/&/g, '').trim();
}

export function normalizeCatalogPath(value: string): string {
  return value.split(/\s*>\s*/).map((segment) => {
    const [label, ...shortcut] = segment.split('\t');
    const normalizedLabel = label.trim();
    return shortcut.length ? `${normalizedLabel}\t${shortcut.join('\t').trim()}` : normalizedLabel;
  }).join(' > ');
}

function slug(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'workflow';
}

function isNavigable(path: string[]): boolean {
  const top = path[0];
  const leaf = path[path.length - 1];
  return top === 'Purchase'
    || top === 'Sales'
    || top === 'Reports'
    || top === 'Basic Data'
    || top === 'Maintenance'
    || top === 'Manage'
    || (top === 'File' && ['Exit', 'Change User'].includes(leaf))
    || (top === 'Help' && leaf === 'About');
}

function implementationFor(path: string[]): MenuAction['implementation'] {
  return isNavigable(path) ? 'implemented' : 'not_implemented';
}

function requirementFor(path: string[], context: LegacyWindowContext): Pick<MenuAction, 'requiredPermission' | 'requiredScope' | 'mappingStatus'> {
  const top = path[0];
  const leaf = path[path.length - 1];
  if (top === 'Sales') return { requiredPermission: 'sales.read', mappingStatus: 'unambiguous' };
  if (top === 'Purchase') return { requiredPermission: 'purchases.read', mappingStatus: 'unambiguous' };
  if (top === 'Reports') return {
    requiredPermission: 'reports.read',
    requiredScope: { kind: 'report', key: slug(leaf) },
    mappingStatus: 'unambiguous'
  };
  if (top === 'Basic Data') return { requiredPermission: 'master.read', mappingStatus: 'unambiguous' };
  if (top === 'Maintenance') return {
    requiredPermission: leaf === 'Preferences' ? 'preferences.read' : 'maintenance.write',
    mappingStatus: 'unambiguous'
  };
  if (top === 'Manage' && leaf === 'Users') return { requiredPermission: 'manage.users', mappingStatus: 'unambiguous' };
  if (top === 'Manage' && leaf === 'Groups') return { requiredPermission: 'manage.groups', mappingStatus: 'unambiguous' };
  if (top === 'File') {
    const writeCommands = ['Save', 'Post', 'Save And Post', 'Populate Items', 'Auto Batch Generation',
      'Apply Item GST %', 'Apply Item Discount %', 'Print Purchase Labels'];
    if (writeCommands.includes(leaf)) {
      const permission = context === 'pack-purchase' ? 'purchases.write'
        : context === 'cash-sale' ? 'sales.write'
        : context === 'item-master' ? 'master.write'
        : context === 'manage-groups' ? 'manage.groups'
        : undefined;
      return permission ? { requiredPermission: permission, mappingStatus: 'unambiguous' } : { mappingStatus: 'ambiguous' };
    }
    return { mappingStatus: 'ambiguous' };
  }
  return { mappingStatus: 'ambiguous' };
}

export function hrefFor(path: string[], commandId: number): string | undefined {
  const top = path[0];
  const leaf = path[path.length - 1];
  const encodedPath = encodeURIComponent(path.join(' > '));
  const encodedCommand = encodeURIComponent(String(commandId));
  if (top === 'File' && leaf === 'Exit') return '/';
  if (top === 'File' && leaf === 'Change User') return '/login?changeUser=1';
  if (top === 'Purchase') {
    const purchases: Record<string, string> = {
      Purchases: 'pack',
      'Purchase Return': 'return',
      'Opening Purchase': 'opening',
      'Purchases (Loose)': 'loose',
      'Purchase Orders': 'order'
    };
    if (purchases[leaf]) return `/app/purchase/${purchases[leaf]}?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  }
  if (top === 'Sales') {
    const openReturn = path[1] === 'Open Sale Return';
    const sales: Record<string, string> = {
      // The legacy caption is "Cash&Sale": stripping the accelerator yields "CashSale" with no space.
      CashSale: 'cash',
      'Cash Sale': 'cash',
      'Credit Sale': 'credit',
      'Cash Sale Return': openReturn ? 'open-cash-return' : 'cash-return',
      'Credit Sale Return': openReturn ? 'open-credit-return' : 'credit-return',
      Quotation: 'quotation',
      'Refused Sales': 'refused'
    };
    if (sales[leaf]) return `/app/sales?kind=${encodeURIComponent(sales[leaf])}&legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  }
  if (top === 'Reports') return `/app/report/${slug(leaf)}?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  if (top === 'Basic Data') {
    const masters: Record<string, string> = { Customer: 'customer', Supplier: 'supplier', Item: 'item', Manufacturer: 'manufacturer', Users: 'user' };
    return `/app/master/${masters[leaf] ?? slug(leaf)}?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  }
  if (top === 'Maintenance' && leaf === 'Preferences') return `/app/preferences?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  if (top === 'Maintenance') return `/app/maintenance/${slug(leaf)}?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  if (top === 'Manage' && leaf === 'Users') return `/app/master/user?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  if (top === 'Manage') return `/app/manage/${slug(leaf)}?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  if (top === 'Window' && leaf === 'Arrange Icons') return '/app';
  if (top === 'Help' && leaf === 'About') return `/app/module/about?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
  // Preserve a deterministic destination for catalog coverage. The menu renderer
  // marks this as not implemented and does not navigate there until it is wired.
  return `/app/module/${slug(leaf)}?legacyPath=${encodedPath}&commandId=${encodedCommand}`;
}

function shellAction(label: string, command: NonNullable<MenuAction['windowCommand']>): MenuAction {
  return {
    label,
    key: `Window > ${label}`,
    legacyPath: `Window > ${label}`,
    implementation: 'shell',
    windowCommand: command
  };
}

function addWindowShellActions(actions: MenuAction[], windows: LegacyOpenWindow[]): void {
  const children = actions;
  const shellActions: MenuAction[] = [
    shellAction('Cascade', 'cascade'),
    shellAction('Tile', 'tile'),
    shellAction('Layer', 'layer'),
    shellAction('Arrange Icons', 'arrange'),
    shellAction('Refresh', 'refresh')
  ];
  for (const candidate of shellActions.reverse()) {
    if (!children.some((action) => action.label === candidate.label)) children.unshift(candidate);
  }

  // The captured list is an observation of the windows that happened to be open.
  // The browser shell owns the live list, so replace those entries safely.
  const staticWindows = children.filter((action) => !/^\d+\s/.test(action.label));
  const numbered = windows.map((window, index) => ({
    label: `${index + 1} ${window.label}`,
    key: `Window > ${index + 1} ${window.label}`,
    legacyPath: `Window > ${index + 1} ${window.label}`,
    implementation: 'shell' as const,
    windowCommand: 'activate' as const,
    windowId: window.id,
    href: window.href
  }));
  actions.splice(0, actions.length, ...staticWindows, ...numbered);
}

function cleanActions(actions: MenuAction[]): MenuAction[] {
  return actions.map((action) => {
    if (!action.children?.length) {
      const { children: _children, ...leaf } = action;
      return leaf;
    }
    return { ...action, children: cleanActions(action.children) };
  });
}

export function buildLegacyMenus(items: LegacyMenuCatalogItem[] = legacyMenuCatalog, windows: LegacyOpenWindow[] = [], context: LegacyWindowContext = 'base'): LegacyMenu[] {
  const menus: LegacyMenu[] = [];
  for (const item of items) {
    const rawSegments = normalizeCatalogPath(item.path).split(' > ');
    const shortcut = rawSegments.at(-1)?.match(/\t(.+)$/)?.[1]?.trim() || undefined;
    const segments = rawSegments.map(cleanLabel);
    const isSeparator = segments.length > 1 && segments.at(-1) === '' && !item.hasSubmenu;
    const visibleSegments = segments.filter(Boolean);
    if (!visibleSegments.length) continue;
    const rootLabel = visibleSegments[0];
    let root = menus.find((menu) => menu.label === rootLabel);
    if (!root) {
      root = { label: rootLabel, actions: [] };
      menus.push(root);
    }
    let actions = root.actions;
    const path: string[] = [rootLabel];
    for (let index = 1; index < visibleSegments.length; index += 1) {
      const label = visibleSegments[index];
      path.push(label);
      let action = actions.find((candidate) => candidate.label === label);
      if (!action) {
        action = { label, key: path.join(' > '), legacyPath: path.join(' > ') };
        actions.push(action);
      }
      if (index === visibleSegments.length - 1 && !item.hasSubmenu) {
        action.commandId = item.commandId;
        action.href = hrefFor(path, item.commandId);
        action.implementation = implementationFor(path);
        Object.assign(action, requirementFor(path, context));
        if (action.implementation === 'not_implemented') action.phase = path[0] === 'File' ? 'H' : 'F';
        if (shortcut) action.shortcut = shortcut;
      }
      if (!action.children) action.children = [];
      actions = action.children;
    }
    if (isSeparator) {
      const separatorPath = segments.join(' > ') + ' > ';
      actions.push({ label: '', key: separatorPath, separator: true, commandId: item.commandId, legacyPath: separatorPath });
    }
  }

  addWindowShellActions(menus.find((menu) => menu.label === 'Window')?.actions ?? [], windows);
  return menus.map((menu) => ({ ...menu, actions: cleanActions(menu.actions) }));
}

export function buildLegacyMenusForContext(context: LegacyWindowContext, windows: LegacyOpenWindow[] = []): LegacyMenu[] {
  if (context === 'base') return buildLegacyMenus(legacyMenuCatalog, windows, context);
  const captured = contextualMenus.contexts.find((entry) => entry.windowType === context);
  return buildLegacyMenus(captured?.items ?? legacyMenuCatalog, windows, context);
}

export type MenuAccess = {
  tenantAdmin: boolean;
  permissions: string[];
  scopes: Record<string, Record<string, boolean>>;
  loaded: boolean;
};

export function applyMenuAccess(menus: LegacyMenu[], access: MenuAccess): LegacyMenu[] {
  const allowed = (action: MenuAction): boolean => {
    if (!action.requiredPermission) return action.mappingStatus !== 'ambiguous' || access.loaded;
    if (access.tenantAdmin) return true;
    if (!access.permissions.some((permission) => permission.toLowerCase() === action.requiredPermission?.toLowerCase())) return false;
    if (action.requiredScope) {
      const entries = access.scopes[action.requiredScope.kind];
      if (entries && Object.keys(entries).length > 0 && !entries['*'] && !entries[action.requiredScope.key]) return false;
    }
    return true;
  };
  const visit = (actions: MenuAction[]): MenuAction[] => actions.map((action) => {
    const children = action.children ? visit(action.children) : undefined;
    const gated = !action.separator && !allowed(action);
    return {
      ...action,
      ...(children ? { children } : {}),
      ...(gated ? { denied: true } : {})
    };
  });
  return menus.map((menu) => ({ ...menu, actions: visit(menu.actions) }));
}

export function findShortcutAction(menus: LegacyMenu[], shortcut: string): MenuAction | undefined {
  const visit = (actions: MenuAction[]): MenuAction | undefined => {
    for (const action of actions) {
      if (action.shortcut?.toLowerCase() === shortcut.toLowerCase()) return action;
      if (action.children) {
        const nested = visit(action.children);
        if (nested) return nested;
      }
    }
    return undefined;
  };
  for (const menu of menus) {
    const found = visit(menu.actions);
    if (found) return found;
  }
  return undefined;
}

export function contextualCatalogCounts(): Record<string, { capturedItems: number; topLevelMenus: number }> {
  return Object.fromEntries(contextualMenus.contexts.map((context) => [
    context.windowType,
    {
      capturedItems: context.items.length,
      topLevelMenus: new Set(context.items.filter((item) => !item.path.includes(' > ')).map((item) => cleanLabel(item.path))).size
    }
  ]));
}

export const legacyMenus = buildLegacyMenus();
