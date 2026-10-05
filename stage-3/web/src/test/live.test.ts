import { ApiError } from '@nrynss/chaaya/keel';
import { tick } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CREDENTIAL_REFUSAL } from '../lib/auth';
import App from '../App.svelte';
import FloorPlan from '../lib/components/FloorPlan.svelte';
import LiveSearch from '../lib/components/LiveSearch.svelte';
import { layoutRoom } from '../lib/floor';
import { currentHold, rememberHold, rememberSearch } from '../lib/hold';
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

async function until(check: () => boolean, label: string): Promise<void> {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    if (check()) return;
    await Promise.resolve();
    await tick();
  }
  throw new Error(label);
}

function take(boxes: Box[], predicate: (path: string) => boolean): Box {
  const index = boxes.findIndex((box) => predicate(box.path));
  if (index < 0) throw new Error(`missing request; have ${boxes.map((box) => box.path).join(' | ')}`);
  return boxes.splice(index, 1)[0];
}

async function settle(): Promise<void> {
  for (let attempt = 0; attempt < 6; attempt += 1) {
    await Promise.resolve();
    await tick();
  }
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
    { id: 't_corner', label: 'Corner', capacity: 6 },
  ],
};

function slots(localTimes: string[], ids: string[]) {
  return {
    restaurant_id: 'r_anker',
    date: '2027-06-17',
    timezone: 'Europe/Berlin',
    slots: localTimes.map((starts) => ({
      starts_at_local: starts,
      starts_at: `${starts}:00+02:00`,
      available_table_ids: ids,
    })),
  };
}

async function openCatalog(target: ParentNode, boxes: Box[]) {
  await until(() => boxes.some((box) => box.path === '/restaurants'), 'catalog request');
  take(boxes, (path) => path === '/restaurants').resolve({
    restaurants: [
      { id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' },
      { id: 'r_nord', name: 'Nordlicht', timezone: 'America/New_York' },
    ],
  });
  await until(
    () => target.querySelectorAll('[data-testid="restaurant-select"] option').length === 2,
    'restaurant options',
  );
}

function setControl(element: HTMLInputElement | HTMLSelectElement, value: string): void {
  const prototype = element instanceof HTMLSelectElement ? HTMLSelectElement.prototype : HTMLInputElement.prototype;
  const descriptor = Object.getOwnPropertyDescriptor(prototype, 'value');
  descriptor?.set?.call(element, value);
  element.dispatchEvent(new Event('input', { bubbles: true }));
  element.dispatchEvent(new Event('change', { bubbles: true }));
}

afterEach(() => {
  installMedia(() => false);
  rememberSearch(null);
  rememberHold(null);
  localStorage.clear();
  vi.unstubAllGlobals();
  window.history.pushState({}, '', '/');
});

describe('live search', () => {
  it('fills restaurant ids and compares every cell with the returned membership', async () => {
    const queued = queueTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: false });
    await openCatalog(view.target, queued.boxes);
    const options = [...view.target.querySelectorAll('[data-testid="restaurant-select"] option')] as HTMLOptionElement[];
    expect(options.map((option) => option.value)).toEqual(['r_anker', 'r_nord']);
    expect((view.target.querySelector('[data-testid="date-input"]') as HTMLInputElement).value).toBe('2027-06-17');
    setControl(view.target.querySelector('[data-testid="party-size-input"]') as HTMLInputElement, '4');
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.length >= 2, 'search requests');
    const detailCall = take(queued.boxes, (path) => path === '/restaurants/r_anker');
    const availabilityCall = take(queued.boxes, (path) => path.startsWith('/availability?'));
    expect(availabilityCall.path).toContain('party_size=4');
    expect(availabilityCall.path).toContain('date=2027-06-17');
    expect(availabilityCall.path).toContain('restaurant_id=r_anker');
    detailCall.resolve(anker);
    availabilityCall.resolve(slots(['2027-06-17T18:00', '2027-06-17T18:30'], ['t_2']));
    await settle();
    expect(view.target.querySelector('h1')?.textContent).toContain('Zum Anker');
    expect(view.target.querySelector('.lede')?.textContent).toContain('party of 4');
    const cells = [...view.target.querySelectorAll('[data-testid^="slot-"]')];
    expect(cells.map((cell) => cell.getAttribute('data-testid'))).toEqual([
      'slot-t_1-18:00',
      'slot-t_1-18:30',
      'slot-t_2-18:00',
      'slot-t_2-18:30',
    ]);
    for (const cell of cells) {
      const id = cell.getAttribute('data-testid') ?? '';
      const available = id.startsWith('slot-t_2-');
      expect(cell.getAttribute('data-available')).toBe(available ? 'true' : 'false');
    }
    expect(view.target.querySelector('[data-preview]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    setControl(view.target.querySelector('[data-testid="restaurant-select"]') as HTMLSelectElement, 'r_nord');
    await tick();
    expect(view.target.querySelector('h1')?.textContent).toContain('Zum Anker');
    expect(view.target.querySelector('[data-testid="slot-t_window-17:00"]')).toBeNull();
    await view.cleanup();
  });

  it('keeps the later restaurant when an earlier detail and availability finish afterwards', async () => {
    const queued = queueTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true });
    await openCatalog(view.target, queued.boxes);
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.some((box) => box.path === '/restaurants/r_anker'), 'first detail');
    const firstDetail = take(queued.boxes, (path) => path === '/restaurants/r_anker');
    const firstAvailability = take(queued.boxes, (path) => path.includes('restaurant_id=r_anker'));
    setControl(view.target.querySelector('[data-testid="restaurant-select"]') as HTMLSelectElement, 'r_nord');
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.some((box) => box.path === '/restaurants/r_nord'), 'second detail');
    const secondDetail = take(queued.boxes, (path) => path === '/restaurants/r_nord');
    const secondAvailability = take(queued.boxes, (path) => path.includes('restaurant_id=r_nord'));
    secondDetail.resolve(nord);
    secondAvailability.resolve({
      restaurant_id: 'r_nord',
      date: '2027-06-17',
      timezone: 'America/New_York',
      slots: [
        {
          starts_at_local: '2027-06-17T17:00',
          starts_at: '2027-06-17T17:00:00-04:00',
          available_table_ids: ['t_window', 't_corner'],
        },
      ],
    });
    await settle();
    expect(view.target.querySelector('h1')?.textContent).toContain('Nordlicht');
    expect(view.target.querySelector('[data-testid="slot-t_window-17:00"]')?.getAttribute('data-available')).toBe('true');
    expect(view.target.querySelector('[data-testid="slot-t_1-18:00"]')).toBeNull();
    firstDetail.resolve(anker);
    firstAvailability.resolve(slots(['2027-06-17T18:00'], ['t_1', 't_2']));
    await settle();
    expect(view.target.querySelector('h1')?.textContent).toContain('Nordlicht');
    expect(view.target.querySelector('[data-testid="slot-t_window-17:00"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="slot-t_1-18:00"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="search-error"]')).toBeNull();
    await view.cleanup();
  });

  it('ignores a late failure from the earlier search', async () => {
    const queued = queueTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: false });
    await openCatalog(view.target, queued.boxes);
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.length >= 2, 'first pair');
    const firstDetail = take(queued.boxes, (path) => path === '/restaurants/r_anker');
    const firstAvailability = take(queued.boxes, (path) => path.includes('restaurant_id=r_anker'));
    setControl(view.target.querySelector('[data-testid="restaurant-select"]') as HTMLSelectElement, 'r_nord');
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.some((box) => box.path === '/restaurants/r_nord'), 'second pair');
    take(queued.boxes, (path) => path === '/restaurants/r_nord').resolve(nord);
    take(queued.boxes, (path) => path.includes('restaurant_id=r_nord')).resolve({
      restaurant_id: 'r_nord',
      date: '2027-06-17',
      timezone: 'America/New_York',
      slots: [
        {
          starts_at_local: '2027-06-17T17:00',
          starts_at: '2027-06-17T17:00:00-04:00',
          available_table_ids: ['t_corner'],
        },
      ],
    });
    await settle();
    firstDetail.reject(new ApiError('gone', 'not_found', 404));
    firstAvailability.reject(new ApiError('gone', 'not_found', 404));
    await settle();
    expect(view.target.querySelector('h1')?.textContent).toContain('Nordlicht');
    expect(view.target.querySelector('[data-testid="search-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="slot-t_corner-17:00"]')?.getAttribute('data-available')).toBe('true');
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    await view.cleanup();
  });

  it('shows closed days and zero availability without inventing tables', async () => {
    const queued = queueTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: false });
    await openCatalog(view.target, queued.boxes);
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.length >= 2, 'closed search');
    take(queued.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(queued.boxes, (path) => path.startsWith('/availability?')).resolve({
      restaurant_id: 'r_anker',
      date: '2027-06-13',
      timezone: 'Europe/Berlin',
      slots: [],
    });
    await settle();
    expect(view.target.querySelector('[data-testid="no-slots"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="availability-grid"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="floor-plan"]')).toBeNull();
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.length >= 2, 'zero search');
    take(queued.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(queued.boxes, (path) => path.startsWith('/availability?')).resolve(slots(['2027-06-17T18:00'], []));
    await settle();
    expect(view.target.querySelector('[data-testid="no-slots"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="availability-grid"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="zero-available"]')).toBeTruthy();
    const cells = [...view.target.querySelectorAll('[data-testid^="slot-"]')];
    expect(cells.length).toBe(2);
    for (const cell of cells) expect(cell.getAttribute('data-available')).toBe('false');
    (cells[0] as HTMLButtonElement).click();
    await tick();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="auth-error"]')).toBeNull();
    await view.cleanup();
  });

  it('refuses a signed-out hold and freezes the searched party for a signed-in hold', async () => {
    const signedOut = queueTransport();
    const guest = render(LiveSearch, { transport: signedOut.transport, signedIn: false });
    await openCatalog(guest.target, signedOut.boxes);
    (guest.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => signedOut.boxes.length >= 2, 'guest search');
    take(signedOut.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(signedOut.boxes, (path) => path.startsWith('/availability?')).resolve(slots(['2027-06-17T18:00'], ['t_1']));
    await settle();
    (guest.target.querySelector('[data-testid="slot-t_2-18:00"]') as HTMLButtonElement).click();
    await tick();
    expect(guest.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(guest.target.querySelector('[data-testid="auth-error"]')).toBeNull();
    (guest.target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement).click();
    await tick();
    expect(guest.target.querySelector('[data-testid="auth-error"]')?.textContent).toContain('Sign in to hold a table.');
    expect(guest.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(currentHold()).toBeNull();
    await guest.cleanup();

    const signedIn = queueTransport();
    const member = render(LiveSearch, { transport: signedIn.transport, signedIn: true });
    await openCatalog(member.target, signedIn.boxes);
    (member.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => signedIn.boxes.length >= 2, 'member search');
    take(signedIn.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(signedIn.boxes, (path) => path.startsWith('/availability?')).resolve(
      slots(['2027-06-17T18:00', '2027-06-17T18:30'], ['t_1', 't_2']),
    );
    await settle();
    setControl(member.target.querySelector('[data-testid="party-size-input"]') as HTMLInputElement, '9');
    await tick();
    const cell = member.target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement;
    cell.click();
    await tick();
    expect(cell.getAttribute('data-selected')).toBe('true');
    expect(member.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(member.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Zum Anker');
    expect(member.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 1');
    expect(member.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('18:00');
    expect(member.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('6:00 PM');
    expect((member.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('2');
    expect(member.target.querySelector('[data-testid="booking-form"]')?.textContent).not.toContain('Booking submission is pending.');
    expect(member.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(member.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(member.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(currentHold()).toMatchObject({
      restaurant_id: 'r_anker',
      table_id: 't_1',
      starts_at_local: '2027-06-17T18:00',
      party_size: 2,
      display: { restaurantName: 'Zum Anker', tableLabel: '1', timezone: 'Europe/Berlin' },
    });
    const plan = member.target.querySelector('[data-testid="plan-t_2"]') as SVGElement;
    plan.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    expect(plan.getAttribute('data-selected')).toBe('true');
    expect(member.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 2');
    await member.cleanup();
  });

  it('stacks the room on a narrow viewport and selects immediately under reduced motion', async () => {
    const tables = Array.from({ length: 6 }, (_, index) => ({
      id: `t_${index + 1}`,
      label: `Window ${index + 1}`,
      capacity: 4,
    }));
    installMedia((query) => query.includes('max-width: 700px'));
    const narrow = render(FloorPlan, {
      tables,
      availableIds: ['t_1'],
      time: '18:00',
      onSelect: () => {},
    });
    await tick();
    const stacked = layoutRoom(tables, { maxColumns: 1 });
    expect(narrow.target.querySelector('svg.room-scene')?.getAttribute('viewBox')).toBe(
      `0 0 ${stacked.width} ${stacked.height}`,
    );
    expect(narrow.target.querySelectorAll('svg.room-scene')).toHaveLength(1);
    await narrow.cleanup();

    installMedia((query) => query.includes('prefers-reduced-motion'));
    const queued = queueTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true });
    await openCatalog(view.target, queued.boxes);
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.boxes.length >= 2, 'motion search');
    take(queued.boxes, (path) => path === '/restaurants/r_anker').resolve(anker);
    take(queued.boxes, (path) => path.startsWith('/availability?')).resolve(slots(['2027-06-17T18:00'], ['t_1']));
    await settle();
    const cell = view.target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement;
    cell.click();
    await tick();
    expect(cell.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected')).toBe('true');
    await view.cleanup();
  });

  it('shows an empty catalogue and retries a failed catalogue', async () => {
    const empty = queueTransport();
    const blank = render(LiveSearch, { transport: empty.transport });
    await until(() => empty.boxes.length === 1, 'empty catalogue');
    empty.boxes[0].resolve({ restaurants: [] });
    await settle();
    expect(blank.target.querySelector('[data-testid="no-restaurants"]')).toBeTruthy();
    expect(blank.target.querySelectorAll('[data-testid="restaurant-select"] option')).toHaveLength(0);
    await blank.cleanup();

    const failed = queueTransport();
    const broken = render(LiveSearch, { transport: failed.transport });
    await until(() => failed.boxes.length === 1, 'failed catalogue');
    failed.boxes.shift()?.reject(new ApiError('The book is unavailable.', 'network', 0));
    await settle();
    expect(broken.target.querySelector('[data-testid="search-error"]')?.textContent).toContain('unavailable');
    (broken.target.querySelector('[data-testid="catalog-retry"]') as HTMLButtonElement).click();
    await until(() => failed.boxes.length === 1, 'retried catalogue');
    failed.boxes[0].resolve({
      restaurants: [{ id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' }],
    });
    await settle();
    expect(broken.target.querySelector('[data-testid="search-error"]')).toBeNull();
    expect((broken.target.querySelector('[data-testid="restaurant-select"] option') as HTMLOptionElement).value).toBe(
      'r_anker',
    );
    await broken.cleanup();
  });
});

describe('live shell', () => {
  it('signs up, keeps the display name across routes, and does not invent a booking', async () => {
    window.history.pushState({}, '', '/?demo=confirmed');
    const calls: { path: string; init: RequestInit }[] = [];
    vi.stubGlobal('fetch', async (path: string, init: RequestInit = {}) => {
      calls.push({ path: String(path), init });
      if (path === '/restaurants') {
        return new Response(JSON.stringify({ restaurants: [] }), { status: 200 });
      }
      if (path === '/auth/signup') {
        const body = JSON.parse(String(init.body)) as { display_name?: string; password?: string };
        if ((body.password ?? '').length < 8) {
          return new Response(
            JSON.stringify({ error: { code: 'validation_failed', message: 'Password is too short.' } }),
            { status: 422 },
          );
        }
        return new Response(
          JSON.stringify({ user_id: 'u_ada', display_name: body.display_name, token: 'opaque-session' }),
          { status: 201 },
        );
      }
      if (path === '/auth/login') {
        return new Response(
          JSON.stringify({ error: { code: 'unauthenticated', message: 'missing or invalid bearer token' } }),
          { status: 401 },
        );
      }
      return new Response(JSON.stringify({ error: { code: 'not_found', message: 'missing' } }), { status: 404 });
    });
    const view = render(App);
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(view.target.textContent).not.toContain('Sample seating');
    expect(view.target.querySelector('[data-testid="current-user"]')).toBeNull();
    (view.target.querySelector('a[href="/signup"]') as HTMLAnchorElement).click();
    await tick();
    setControl(view.target.querySelector('[data-testid="signup-email"]') as HTMLInputElement, 'ada@example.com');
    setControl(view.target.querySelector('[data-testid="signup-password"]') as HTMLInputElement, 'short');
    setControl(view.target.querySelector('[data-testid="signup-display-name"]') as HTMLInputElement, 'Ada');
    (view.target.querySelector('[data-testid="signup-submit"]') as HTMLButtonElement).click();
    await settle();
    expect(view.target.querySelector('[data-testid="auth-error"]')?.textContent).toContain('too short');
    expect(view.target.querySelector('[data-testid="current-user"]')).toBeNull();
    setControl(view.target.querySelector('[data-testid="signup-password"]') as HTMLInputElement, 'correct horse');
    (view.target.querySelector('[data-testid="signup-submit"]') as HTMLButtonElement).click();
    await settle();
    expect(view.target.querySelector('[data-testid="current-user"]')?.textContent).toContain('Ada');
    expect(view.target.querySelector('[data-testid="auth-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="logout-button"]')).toBeTruthy();
    const stored = JSON.parse(localStorage.getItem('tablekeeper-session') ?? '{}') as { token?: string; displayName?: string };
    expect(stored.displayName).toBe('Ada');
    expect(typeof stored.token).toBe('string');
    expect((stored.token ?? '').length).toBeGreaterThan(8);
    (view.target.querySelector('a[href="/lookup"]') as HTMLAnchorElement).click();
    await tick();
    expect(view.target.querySelector('[data-testid="current-user"]')?.textContent).toContain('Ada');
    expect(view.target.querySelector('[data-testid="lookup-reference-input"]')).toBeTruthy();
    (view.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    expect(view.target.querySelector('[data-testid="reservation-detail"]')).toBeNull();
    (view.target.querySelector('[data-testid="logout-button"]') as HTMLButtonElement).click();
    await tick();
    expect(view.target.querySelector('[data-testid="current-user"]')).toBeNull();
    expect(localStorage.getItem('tablekeeper-session')).toBeNull();
    (view.target.querySelector('a[href="/login"]') as HTMLAnchorElement).click();
    await tick();
    setControl(view.target.querySelector('[data-testid="login-email"]') as HTMLInputElement, 'ada@example.com');
    setControl(view.target.querySelector('[data-testid="login-password"]') as HTMLInputElement, 'wrong horse');
    (view.target.querySelector('[data-testid="login-submit"]') as HTMLButtonElement).click();
    await settle();
    const loginError = view.target.querySelector('[data-testid="auth-error"]')?.textContent ?? '';
    expect(loginError).toBe(CREDENTIAL_REFUSAL);
    expect(loginError.toLowerCase()).not.toContain('bearer');
    expect(loginError.toLowerCase()).not.toContain('token');
    expect(view.target.querySelector('[data-testid="current-user"]')).toBeNull();
    expect(calls.some((call) => String(call.path).includes('/reservations'))).toBe(false);
    await view.cleanup();
  });
});
