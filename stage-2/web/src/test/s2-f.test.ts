import { ApiError } from '@nrynss/chaaya/keel';
import { tick } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import FloorPlan from '../lib/components/FloorPlan.svelte';
import LiveSearch from '../lib/components/LiveSearch.svelte';
import Shell from '../lib/components/Shell.svelte';
import { rememberHold, rememberSearch } from '../lib/hold';
import { revealOutcome } from '../lib/motion';
import type { Transport } from '../lib/transport';
import { render } from './render';

interface Box {
  path: string;
  resolve: (value: unknown) => void;
  reject: (error: unknown) => void;
}

function queueTransport() {
  const boxes: Box[] = [];
  const transport: Transport = (path: string) =>
    new Promise((resolve, reject) => {
      boxes.push({ path, resolve, reject });
    });
  return { transport, boxes };
}

function installMedia(matches: (query: string) => boolean): void {
  window.matchMedia = (query: string) =>
    ({
      matches: matches(query),
      media: query,
      onchange: null,
      addListener() {},
      removeListener() {},
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() {
        return false;
      },
    }) as MediaQueryList;
}

function take(boxes: Box[], predicate: (path: string) => boolean): Box {
  const index = boxes.findIndex((box) => predicate(box.path));
  if (index < 0) throw new Error(`missing request; have ${boxes.map((box) => box.path).join(' | ')}`);
  return boxes.splice(index, 1)[0];
}

async function until(check: () => boolean, label: string): Promise<void> {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    if (check()) return;
    await Promise.resolve();
    await tick();
  }
  throw new Error(label);
}

async function settle(): Promise<void> {
  for (let attempt = 0; attempt < 8; attempt += 1) {
    await Promise.resolve();
    await tick();
  }
}

function setControl(element: HTMLInputElement | HTMLSelectElement, value: string): void {
  const prototype = element instanceof HTMLSelectElement ? HTMLSelectElement.prototype : HTMLInputElement.prototype;
  const descriptor = Object.getOwnPropertyDescriptor(prototype, 'value');
  descriptor?.set?.call(element, value);
  element.dispatchEvent(new Event('input', { bubbles: true }));
  element.dispatchEvent(new Event('change', { bubbles: true }));
}

function navLabels(target: ParentNode): string[] {
  return [...target.querySelectorAll('nav[aria-label="Primary"] a')].map((link) => link.textContent?.trim() ?? '');
}

const anker = {
  id: 'r_anker',
  name: 'Zum Anker',
  timezone: 'Europe/Berlin',
  tables: [
    { id: 't_1', label: '1', capacity: 2 },
    { id: 't_2', label: '2', capacity: 4 },
  ],
};

const nord = {
  id: 'r_nord',
  name: 'Nordlicht',
  timezone: 'America/New_York',
  tables: [
    { id: 't_window', label: 'Window', capacity: 2 },
    { id: 't_corner', label: 'Corner', capacity: 4 },
  ],
};

function availability(restaurantId: string, ids: string[], starts = '2027-06-17T18:00') {
  return {
    restaurant_id: restaurantId,
    date: '2027-06-17',
    timezone: restaurantId === 'r_nord' ? 'America/New_York' : 'Europe/Berlin',
    slots: [
      {
        starts_at_local: starts,
        starts_at: `${starts}:00+02:00`,
        available_table_ids: ids,
      },
    ],
  };
}

function reservation(reference: string) {
  return {
    reservation_id: `res_${reference}`,
    reference,
    restaurant_id: 'r_nord',
    table_id: 't_window',
    party_size: 2,
    status: 'confirmed',
    starts_at_local: '2027-06-17T17:00',
    starts_at: '2027-06-17T17:00:00-04:00',
    ends_at: '2027-06-17T18:30:00-04:00',
    created_at: '2026-09-21T11:04:03+00:00',
  };
}

const originalRect = Element.prototype.getBoundingClientRect;
const originalScrollBy = window.scrollBy;
let scrolls: { top: number; behavior: string; confirmation: string | null; form: boolean; caption: boolean }[] = [];

function rect(top: number, height: number): DOMRect {
  return {
    x: 0,
    y: top,
    left: 0,
    right: 320,
    top,
    bottom: top + height,
    width: 320,
    height,
    toJSON() {
      return {};
    },
  } as DOMRect;
}

function installLayout(): void {
  scrolls = [];
  window.innerHeight = 900;
  Element.prototype.getBoundingClientRect = function rectFor() {
    if (this.classList?.contains('grid-caption')) return rect(1400, 36);
    const id = this.getAttribute?.('data-testid');
    if (id === 'booking-form') return rect(1700, 280);
    if (id === 'confirmation') return rect(2100, 180);
    if (id === 'no-slots') return rect(1200, 160);
    if (id === 'auth-error') return rect(1000, 48);
    if (id === 'booking-error') return rect(2000, 48);
    if (id === 'booking-uncertain') return rect(2000, 72);
    return rect(20, 24);
  };
  window.scrollBy = ((options?: ScrollToOptions) => {
    const top = typeof options === 'object' && options ? Number(options.top ?? 0) : 0;
    const behavior = typeof options === 'object' && options?.behavior ? options.behavior : 'auto';
    scrolls.push({
      top,
      behavior,
      confirmation: document.querySelector('[data-testid="confirmation-reference"]')?.textContent ?? null,
      form: Boolean(document.querySelector('[data-testid="booking-form"]')),
      caption: Boolean(document.querySelector('.grid-caption')),
    });
  }) as typeof window.scrollBy;
}

async function openCatalog(target: ParentNode, boxes: Box[]) {
  await until(() => boxes.some((box) => box.path === '/restaurants'), 'catalog');
  take(boxes, (path) => path === '/restaurants').resolve({
    restaurants: [
      { id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' },
      { id: 'r_nord', name: 'Nordlicht', timezone: 'America/New_York' },
    ],
  });
  await until(() => target.querySelectorAll('[data-testid="restaurant-select"] option').length === 2, 'options');
}

afterEach(() => {
  installMedia(() => false);
  Element.prototype.getBoundingClientRect = originalRect;
  window.scrollBy = originalScrollBy;
  rememberSearch(null);
  rememberHold(null);
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
});

describe('signed-in header', () => {
  it('hides only the auth links for a session, including a blank display name', async () => {
    installMedia(() => false);
    const signedOut = render(Shell, { path: '/', user: null });
    expect(navLabels(signedOut.target)).toEqual(['Search', 'Look up', 'Sign in', 'Create account']);
    expect(signedOut.target.querySelector('[data-testid="current-user"]')).toBeNull();
    expect(signedOut.target.querySelector('[data-testid="logout-button"]')).toBeNull();
    expect(signedOut.target.querySelector('[data-testid="theme-light"]')).toBeTruthy();
    await signedOut.cleanup();

    let loggedOut = false;
    const named = render(Shell, { path: '/lookup', user: 'Ada', onLogout: () => { loggedOut = true; } });
    expect(navLabels(named.target)).toEqual(['Search', 'Look up']);
    expect(named.target.querySelector('[data-testid="current-user"]')?.textContent).toBe('Ada');
    expect(named.target.querySelector('a[href="/login"]')).toBeNull();
    expect(named.target.querySelector('a[href="/signup"]')).toBeNull();
    (named.target.querySelector('[data-testid="logout-button"]') as HTMLButtonElement).click();
    expect(loggedOut).toBe(true);
    await named.cleanup();

    const blank = render(Shell, { path: '/', user: '' });
    expect(navLabels(blank.target)).toEqual(['Search', 'Look up']);
    expect(blank.target.querySelector('[data-testid="current-user"]')).toBeTruthy();
    expect(blank.target.querySelector('[data-testid="current-user"]')?.textContent).toBe('');
    expect(blank.target.querySelector('[data-testid="logout-button"]')).toBeTruthy();
    expect(blank.target.querySelector('a[href="/login"]')).toBeNull();
    await blank.cleanup();
  });
});

describe('floor paint order', () => {
  it('paints pair connectors before tabletops and badges after the plate text', async () => {
    const view = render(FloorPlan, {
      tables: [
        { id: 't_1', label: 'Window alcove', capacity: 2 },
        { id: 't_2', label: 'Garden corner', capacity: 4 },
        { id: 't_3', label: 'Hearth booth', capacity: 4 },
      ],
      availableIds: ['t_3'],
      selectedIds: ['t_1', 't_2'],
      time: '18:00',
      pairs: [
        { ids: ['t_1', 't_2'], labels: ['Window alcove', 'Garden corner'], capacity: 6, available: true },
        { ids: ['t_2', 't_3'], labels: ['Garden corner', 'Hearth booth'], capacity: 8, available: false },
      ],
      onSelect: () => {},
    });
    const scene = view.target.querySelector('svg.room-scene');
    const nodes = [...(scene?.querySelectorAll('*') ?? [])];
    const indexOf = (selector: string) => nodes.findIndex((node) => node.matches(selector));
    const link = indexOf('.pair-link');
    const top = indexOf('.table-top');
    const plate = indexOf('.plate-label');
    const seats = indexOf('.plan-seats');
    const badge = indexOf('.pair-badge');
    expect(link).toBeGreaterThanOrEqual(0);
    expect(top).toBeGreaterThan(link);
    expect(plate).toBeGreaterThan(top);
    expect(seats).toBeGreaterThan(plate);
    expect(badge).toBeGreaterThan(seats);
    const garden = view.target.querySelector('[data-testid="plan-t_2"]');
    expect(garden?.querySelector('.plan-name')).toBeNull();
    expect(garden?.querySelector('.plate-label')?.textContent).toContain('Garden');
    expect(garden?.getAttribute('aria-label')).toContain('Table Garden corner');
    const chip = view.target.querySelector('[data-testid="plan-t_1+t_2"]');
    expect(chip?.textContent).toContain('Window alcove');
    expect(chip?.textContent).toContain('Garden corner');
    expect(chip?.getAttribute('aria-label')).toContain('Table Window alcove');
    expect(chip?.getAttribute('aria-label')).toContain('Table Garden corner');
    expect(chip?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('.place-cards')?.textContent).toContain('Table Garden corner');
    await view.cleanup();
  });
});

describe('outcome reveal', () => {
  it('fits a short target and reveals only the start of a tall grid', () => {
    installMedia(() => false);
    installLayout();
    const caption = document.createElement('p');
    caption.className = 'grid-caption';
    document.body.appendChild(caption);
    revealOutcome(caption, 220);
    expect(scrolls).toHaveLength(1);
    expect(scrolls[0].behavior).toBe('smooth');
    expect(scrolls[0].top).toBe(1400 + 220 - (900 - 16));
    scrolls.length = 0;
    const visible = document.createElement('p');
    visible.getBoundingClientRect = () => rect(40, 80);
    document.body.appendChild(visible);
    revealOutcome(visible, 0);
    expect(scrolls).toHaveLength(0);
    installMedia((query) => query.includes('prefers-reduced-motion'));
    const below = document.createElement('div');
    below.getBoundingClientRect = () => rect(1200, 160);
    document.body.appendChild(below);
    revealOutcome(below, 0);
    expect(scrolls.at(-1)?.behavior).toBe('auto');
    expect(scrolls.at(-1)?.top).toBe(1200 + 160 - (900 - 16));
  });

  it('reveals the latest search, the form and the confirmation without scrolling edits or stale results', async () => {
    installMedia(() => false);
    installLayout();
    const queued = queueTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(view.target, queued.boxes);

    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.some((box) => box.path === '/restaurants/r_anker'), 'first detail');
    const firstDetail = take(queued.boxes, (path) => path === '/restaurants/r_anker');
    const firstAvailability = take(queued.boxes, (path) => path.includes('restaurant_id=r_anker'));
    setControl(view.target.querySelector('[data-testid="restaurant-select"]') as HTMLSelectElement, 'r_nord');
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.some((box) => box.path === '/restaurants/r_nord'), 'second detail');
    take(queued.boxes, (path) => path === '/restaurants/r_nord').resolve(nord);
    take(queued.boxes, (path) => path.includes('restaurant_id=r_nord')).resolve(
      availability('r_nord', ['t_window'], '2027-06-17T17:00'),
    );
    await settle();
    expect(view.target.querySelector('.grid-caption')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="slot-t_window-17:00"]')?.getAttribute('data-available')).toBe('true');
    expect(scrolls.some((scroll) => scroll.caption && scroll.top === 1400 + 220 - (900 - 16))).toBe(true);
    const afterLatest = scrolls.length;
    firstDetail.resolve(anker);
    firstAvailability.resolve(availability('r_anker', ['t_1']));
    await settle();
    expect(scrolls).toHaveLength(afterLatest);
    expect(view.target.querySelector('[data-testid="slot-t_1-18:00"]')).toBeNull();
    expect(view.target.querySelector('h1')?.textContent).toContain('Nordlicht');

    (view.target.querySelector('[data-testid="slot-t_window-17:00"]') as HTMLButtonElement).click();
    await settle();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(scrolls.at(-1)?.form).toBe(true);
    expect(scrolls.at(-1)?.top).toBe(1700 + 280 - (900 - 16));
    const afterForm = scrolls.length;
    const party = view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement;
    setControl(party, '3');
    await settle();
    expect(scrolls).toHaveLength(afterForm);
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('3');
    setControl(party, '2');
    await tick();

    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.some((box) => box.path === '/reservations'), 'booking');
    const posted = take(queued.boxes, (boxPath) => boxPath === '/reservations');
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    posted.resolve(reservation('RIVER7'));
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('RIVER7');
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(scrolls.at(-1)?.confirmation).toBe('RIVER7');
    expect(scrolls.at(-1)?.top).toBe(2100 + 180 - (900 - 16));
    await view.cleanup();
  });

  it('reveals an empty day, a signed-out hold and a lost response without a confirmation', async () => {
    installMedia((query) => query.includes('prefers-reduced-motion'));
    installLayout();
    const queued = queueTransport();
    const guest = render(LiveSearch, { transport: queued.transport, signedIn: false });
    await openCatalog(guest.target, queued.boxes);
    (guest.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.length >= 2, 'closed');
    take(queued.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(queued.boxes, (path) => path.startsWith('/availability?')).resolve({
      restaurant_id: 'r_anker',
      date: '2027-06-13',
      timezone: 'Europe/Berlin',
      slots: [],
    });
    await settle();
    expect(guest.target.querySelector('[data-testid="no-slots"]')).toBeTruthy();
    expect(guest.target.querySelector('[data-testid="availability-grid"]')).toBeNull();
    expect(scrolls.at(-1)?.behavior).toBe('auto');
    expect(scrolls.at(-1)?.top).toBe(1200 + 160 - (900 - 16));
    await guest.cleanup();

    const next = queueTransport();
    const member = render(LiveSearch, { transport: next.transport, signedIn: false });
    await openCatalog(member.target, next.boxes);
    (member.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => next.boxes.length >= 2, 'open');
    take(next.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(next.boxes, (path) => path.startsWith('/availability?')).resolve(availability('r_anker', ['t_1']));
    await settle();
    scrolls.length = 0;
    (member.target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement).click();
    await settle();
    expect(member.target.querySelector('[data-testid="auth-error"]')?.textContent).toContain('Sign in');
    expect(member.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(scrolls.at(-1)?.top).toBe(1000 + 48 - (900 - 16));
    expect(scrolls.at(-1)?.behavior).toBe('auto');
    await member.cleanup();

    const lostQueue = queueTransport();
    const lost = render(LiveSearch, { transport: lostQueue.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(lost.target, lostQueue.boxes);
    (lost.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => lostQueue.boxes.length >= 2, 'member search');
    take(lostQueue.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(lostQueue.boxes, (path) => path.startsWith('/availability?')).resolve(availability('r_anker', ['t_1']));
    await settle();
    (lost.target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement).click();
    await settle();
    (lost.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => lostQueue.boxes.some((box) => box.path === '/reservations'), 'lost post');
    take(lostQueue.boxes, (path) => path === '/reservations').reject(new Error('network down'));
    await settle();
    expect(lost.target.querySelector('[data-testid="booking-uncertain"]')?.textContent?.trim().length).toBeGreaterThan(0);
    expect(lost.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(lost.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(lost.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(scrolls.at(-1)?.top).toBe(2000 + 72 - (900 - 16));
    await lost.cleanup();

    const refusedQueue = queueTransport();
    const refused = render(LiveSearch, {
      transport: refusedQueue.transport,
      signedIn: true,
      token: 'opaque-session',
    });
    await openCatalog(refused.target, refusedQueue.boxes);
    (refused.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => refusedQueue.boxes.length >= 2, 'refused search');
    take(refusedQueue.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(refusedQueue.boxes, (path) => path.startsWith('/availability?')).resolve(availability('r_anker', ['t_1']));
    await settle();
    (refused.target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement).click();
    await settle();
    (refused.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => refusedQueue.boxes.some((box) => box.path === '/reservations'), 'refused post');
    take(refusedQueue.boxes, (path) => path === '/reservations').reject(
      new ApiError('That table was just taken.', 'table_unavailable', 409),
    );
    await settle();
    expect(refused.target.querySelector('[data-testid="booking-error"]')?.textContent).toContain('taken');
    expect(refused.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(refused.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(scrolls.at(-1)?.top).toBe(2000 + 48 - (900 - 16));
    await refused.cleanup();
  });
});
