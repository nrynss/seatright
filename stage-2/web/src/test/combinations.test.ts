import { ApiError } from '@nrynss/chaaya/keel';
import { a11yGate } from '@nrynss/chaaya/testing';
import { theme } from '@nrynss/chaaya/theme';
import { tick } from 'svelte';
import { afterEach, describe, expect, it } from 'vitest';
import { parseReservation, sameBooking } from '../lib/booking';
import LiveSearch from '../lib/components/LiveSearch.svelte';
import LookupScreen from '../lib/components/LookupScreen.svelte';
import { currentHold, rememberHold, rememberSearch } from '../lib/hold';
import { parseAvailability, parseDetail } from '../lib/search';
import { canonicalTableIds } from '../lib/seating';
import type { Transport, TransportRequest } from '../lib/transport';
import { render } from './render';

interface Call {
  path: string;
  init?: TransportRequest;
  resolve: (value: unknown) => void;
  reject: (error: unknown) => void;
}

function recordingTransport() {
  const calls: Call[] = [];
  const transport: Transport = (path, init) =>
    new Promise((resolve, reject) => {
      calls.push({ path, init, resolve, reject });
    });
  return { transport, calls };
}

function take(calls: Call[], predicate: (call: Call) => boolean): Call {
  const index = calls.findIndex(predicate);
  if (index < 0) throw new Error(`missing request; have ${calls.map((call) => call.path).join(' | ')}`);
  return calls.splice(index, 1)[0];
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
  for (let attempt = 0; attempt < 6; attempt += 1) {
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

const TIMES = ['18:00', '18:30', '19:00', '19:30', '20:00', '20:30', '21:00', '21:30'];

const anker = {
  id: 'r_anker',
  name: 'Zum Anker',
  timezone: 'Europe/Berlin',
  slot_minutes: 30,
  reservation_duration_minutes: 90,
  cancellation_cutoff_minutes: 120,
  opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
  tables: [
    { id: 't_1', label: '1', capacity: 2 },
    { id: 't_2', label: '2', capacity: 4 },
    { id: 't_3', label: '3', capacity: 4 },
  ],
  combinable: [
    ['t_1', 't_2'],
    ['t_2', 't_3'],
  ],
};

/** Free party-of-6 floor. The first option is reversed so the screen must restore declared order. */
function freeFloor() {
  return {
    restaurant_id: 'r_anker',
    date: '2027-06-17',
    timezone: 'Europe/Berlin',
    slots: TIMES.map((time) => ({
      starts_at_local: `2027-06-17T${time}`,
      starts_at: `2027-06-17T${time}:00+02:00`,
      available_table_ids: [] as string[],
      available_options: [
        { table_ids: ['t_2', 't_1'], capacity: 6 },
        { table_ids: ['t_2', 't_3'], capacity: 8 },
      ],
    })),
  };
}

/** t_1+t_2 from 18:00 occupies both pairs until 19:30. Later slots stay open. */
function afterPairBooked() {
  const floor = freeFloor();
  for (const slot of floor.slots) {
    const time = slot.starts_at_local.slice(11);
    if (time === '18:00' || time === '18:30' || time === '19:00') slot.available_options = [];
  }
  return floor;
}

function pairReceipt(status: 'confirmed' | 'cancelled' = 'confirmed') {
  return {
    reservation_id: 'res_pair',
    reference: 'PAIR01',
    restaurant_id: 'r_anker',
    table_ids: ['t_1', 't_2'],
    party_size: 6,
    status,
    starts_at_local: '2027-06-17T18:00',
    starts_at: '2027-06-17T18:00:00+02:00',
    ends_at: '2027-06-17T19:30:00+02:00',
    created_at: '2026-10-04T22:00:00+00:00',
  };
}

function posted(call: Call): Record<string, unknown> {
  return call.init?.body as Record<string, unknown>;
}

async function openPair(target: ParentNode, calls: Call[], availability: unknown = freeFloor()) {
  await until(() => calls.some((call) => call.path === '/restaurants'), 'catalog');
  take(calls, (call) => call.path === '/restaurants').resolve({
    restaurants: [
      { id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' },
      { id: 'r_nord', name: 'Nordlicht', timezone: 'America/New_York' },
    ],
  });
  await until(() => target.querySelectorAll('[data-testid="restaurant-select"] option').length === 2, 'options');
  setControl(target.querySelector('[data-testid="party-size-input"]') as HTMLInputElement, '6');
  (target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
  await until(() => calls.some((call) => call.path.startsWith('/availability?')), 'search');
  expect(calls.find((call) => call.path.startsWith('/availability?'))?.path).toContain('party_size=6');
  take(calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
  take(calls, (call) => call.path.startsWith('/availability?')).resolve(availability);
  await settle();
}

function cell(target: ParentNode, id: string): HTMLButtonElement {
  const node = target.querySelector(`[data-testid="${id}"]`);
  if (!(node instanceof HTMLButtonElement)) throw new Error(`missing ${id}`);
  return node;
}

afterEach(() => {
  rememberSearch(null);
  rememberHold(null);
  theme.set('system');
  document.body.innerHTML = '';
});

describe('pair seating data', () => {
  it('keeps declared order, summed capacity and a private copy of every id list', () => {
    const reversed = ['t_2', 't_1'];
    expect(canonicalTableIds(anker.combinable, reversed)).toEqual(['t_1', 't_2']);
    expect(canonicalTableIds(anker.combinable, reversed)).not.toBe(reversed);
    reversed.push('t_3');
    expect(canonicalTableIds(anker.combinable, ['t_2', 't_1'])).toEqual(['t_1', 't_2']);

    const detail = parseDetail(anker);
    expect(detail.combinable).toEqual([
      ['t_1', 't_2'],
      ['t_2', 't_3'],
    ]);
    expect(parseDetail({ ...anker, combinable: undefined }).combinable).toEqual([]);
    expect(() => parseDetail({ ...anker, combinable: [['t_1', 't_2', 't_3']] })).toThrow(ApiError);

    const sourceIds = ['t_2', 't_1'];
    const parsed = parseAvailability({
      restaurant_id: 'r_anker',
      date: '2027-06-17',
      timezone: 'Europe/Berlin',
      slots: [
        {
          starts_at_local: '2027-06-17T18:00',
          starts_at: '2027-06-17T18:00:00+02:00',
          available_table_ids: [],
          available_options: [{ table_ids: sourceIds, capacity: 6 }],
        },
      ],
    });
    expect(parsed.slots[0].availableTableIds).toEqual([]);
    expect(parsed.slots[0].options[0]).toEqual({ tableIds: ['t_2', 't_1'], capacity: 6 });
    sourceIds[0] = 't_9';
    expect(parsed.slots[0].options[0].tableIds).toEqual(['t_2', 't_1']);

    const legacyOnly = {
      restaurant_id: 'r_anker',
      date: '2027-06-17',
      timezone: 'Europe/Berlin',
      slots: [
        {
          starts_at_local: '2027-06-17T18:00',
          starts_at: '2027-06-17T18:00:00+02:00',
          available_table_ids: ['t_2'],
        },
      ],
    };
    const legacy = parseAvailability(legacyOnly);
    expect(legacy.slots[0].options).toEqual([{ tableIds: ['t_2'], capacity: 0 }]);

    const wire = ['t_1', 't_2'];
    const record = parseReservation({ ...pairReceipt(), table_ids: wire });
    expect(record.table_id).toBe('');
    expect(record.tableIds).toEqual(['t_1', 't_2']);
    expect(record.tableIds).not.toBe(wire);
    wire.reverse();
    expect(record.tableIds).toEqual(['t_1', 't_2']);
    expect(record.reference).toBe('PAIR01');
    const old = parseReservation({
      reservation_id: 'res_7',
      reference: 'RIVER7',
      restaurant_id: 'r_anker',
      table_id: 't_1',
      party_size: 2,
      status: 'confirmed',
      starts_at_local: '2027-06-17T18:00',
      starts_at: '2027-06-17T18:00:00+02:00',
      ends_at: '2027-06-17T19:30:00+02:00',
      created_at: '2026-09-21T11:04:03+00:00',
    });
    expect(old.table_id).toBe('t_1');
    expect(old.tableIds).toEqual(['t_1']);
    expect(sameBooking(
      { restaurant_id: 'r_anker', table_ids: ['t_1', 't_2'], starts_at_local: '2027-06-17T18:00', party_size: 6 },
      { restaurant_id: 'r_anker', table_id: 't_1', starts_at_local: '2027-06-17T18:00', party_size: 6 },
    )).toBe(false);
  });
});

describe('combined table screen', () => {
  it('offers declared pairs by label, ignores an unavailable pair, and posts the canonical set once', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openPair(view.target, queued.calls);
    expect(view.target.querySelector('[data-testid="zero-available"]')).toBeNull();
    expect(view.target.querySelectorAll('svg.room-scene')).toHaveLength(1);
    const singles = ['t_1', 't_2', 't_3'].flatMap((id) => TIMES.map((time) => `slot-${id}-${time}`));
    const pairs = ['t_1+t_2', 't_2+t_3'].flatMap((id) => TIMES.map((time) => `slot-${id}-${time}`));
    const hooks = [...view.target.querySelectorAll('[data-testid^="slot-"]')].map((node) => node.getAttribute('data-testid'));
    expect(hooks).toEqual([...singles, ...pairs]);
    for (const time of TIMES) {
      expect(cell(view.target, `slot-t_1-${time}`).getAttribute('data-available')).toBe('false');
      expect(cell(view.target, `slot-t_1+t_2-${time}`).getAttribute('data-available')).toBe('true');
      expect(cell(view.target, `slot-t_2+t_3-${time}`).getAttribute('data-available')).toBe('true');
    }
    const heads = [...view.target.querySelectorAll('.rowhead')].map((node) => node.textContent ?? '');
    expect(heads[3]).toContain('Table 1 and Table 2');
    expect(heads[3]).toContain('6 seats together');
    expect(heads[4]).toContain('Table 2 and Table 3');
    expect(heads[4]).toContain('8 seats together');
    expect(heads.join(' ')).not.toContain('t_1');
    expect(view.target.querySelector('[data-testid="plan-t_1"]')?.querySelectorAll('.seat')).toHaveLength(2);
    expect(view.target.querySelector('[data-testid="plan-t_2"]')?.querySelectorAll('.seat')).toHaveLength(4);
    expect(view.target.querySelector('[data-testid="plan-t_3"]')?.querySelectorAll('.seat')).toHaveLength(4);
    expect(view.target.querySelector('[data-testid="plan-t_1+t_2"]')?.textContent).toContain('1');
    expect(view.target.querySelector('[data-testid="plan-t_1+t_2"]')?.textContent).not.toContain('t_1');
    await view.cleanup();

    const narrowed = recordingTransport();
    const held = render(LiveSearch, { transport: narrowed.transport, signedIn: true, token: 'opaque-session' });
    const floor = freeFloor();
    floor.slots[0].available_options = [{ table_ids: ['t_2', 't_1'], capacity: 6 }];
    await openPair(held.target, narrowed.calls, floor);
    expect(cell(held.target, 'slot-t_2+t_3-18:00').getAttribute('data-available')).toBe('false');
    expect(cell(held.target, 'slot-t_1+t_2-18:00').getAttribute('data-available')).toBe('true');
    cell(held.target, 'slot-t_2+t_3-18:00').click();
    await tick();
    expect(held.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(held.target.querySelector('[data-testid="auth-error"]')).toBeNull();
    expect(currentHold()).toBeNull();
    cell(held.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    expect(cell(held.target, 'slot-t_1+t_2-18:00').getAttribute('data-selected')).toBe('true');
    expect(cell(held.target, 'slot-t_1-18:00').getAttribute('data-selected')).toBe('false');
    expect(held.target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected')).toBe('true');
    expect(held.target.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-selected')).toBe('true');
    expect(held.target.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-selected')).toBe('false');
    expect(held.target.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('data-selected')).toBe('true');
    expect(held.target.querySelector('[data-testid="plan-t_2+t_3"]')?.getAttribute('data-selected')).toBe('false');
    const summary = held.target.querySelector('[data-testid="booking-summary"]')?.textContent ?? '';
    expect(summary).toContain('Zum Anker');
    expect(summary).toContain('Table 1 and Table 2');
    expect(summary).toContain('6:00 PM');
    expect(summary).toContain('18:00');
    expect(summary).not.toContain('t_1');
    expect((held.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('6');
    setControl(held.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '8');
    await tick();
    cell(held.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    expect((held.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('8');
    setControl(held.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '6');
    await tick();
    expect(currentHold()?.tableIds).toEqual(['t_1', 't_2']);
    const optionIds = floor.slots[0].available_options[0].table_ids;
    optionIds.push('t_9');
    expect(currentHold()?.tableIds).toEqual(['t_1', 't_2']);

    (held.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => narrowed.calls.some((call) => call.path === '/reservations'), 'create');
    const created = take(narrowed.calls, (call) => call.path === '/reservations');
    expect(posted(created)).toEqual({
      restaurant_id: 'r_anker',
      table_ids: ['t_1', 't_2'],
      starts_at_local: '2027-06-17T18:00',
      party_size: 6,
    });
    expect(posted(created)).not.toHaveProperty('table_id');
    expect(created.init?.idempotencyKey).toMatch(/^[0-9a-f]{32}$/);
    const sentIds = (posted(created).table_ids as string[]);
    sentIds.push('t_9');
    created.resolve(pairReceipt());
    await settle();
    expect(held.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('PAIR01');
    expect(held.target.querySelector('[data-testid="confirmation-details"]')?.textContent).toContain('Zum Anker');
    expect(held.target.querySelector('[data-testid="confirmation-details"]')?.textContent).toContain('Table 1');
    expect(held.target.querySelector('[data-testid="confirmation-details"]')?.textContent).toContain('Table 2');
    expect(held.target.querySelector('[data-testid="confirmation-details"]')?.textContent).toContain('18:00');
    expect(held.target.querySelector('[data-testid="confirmation-tables"]')?.textContent).toContain('Table 1');
    expect(held.target.querySelector('[data-testid="confirmation-tables"]')?.textContent).toContain('Table 2');
    expect(held.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(held.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(held.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();

    (held.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => narrowed.calls.some((call) => call.path === '/reservations'), 'replay');
    const replay = take(narrowed.calls, (call) => call.path === '/reservations');
    expect(replay.init?.idempotencyKey).toBe(created.init?.idempotencyKey);
    expect(posted(replay)).toEqual({
      restaurant_id: 'r_anker',
      table_ids: ['t_1', 't_2'],
      starts_at_local: '2027-06-17T18:00',
      party_size: 6,
    });
    replay.resolve(pairReceipt());
    await settle();
    expect(held.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('PAIR01');

    setControl(held.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '8');
    await tick();
    (held.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => narrowed.calls.some((call) => call.path === '/reservations'), 'changed');
    const changed = take(narrowed.calls, (call) => call.path === '/reservations');
    expect(changed.init?.idempotencyKey).not.toBe(created.init?.idempotencyKey);
    expect(posted(changed).party_size).toBe(8);
    expect(posted(changed).table_ids).toEqual(['t_1', 't_2']);
    changed.reject(new TypeError('socket closed'));
    await settle();
    expect(held.target.querySelector('[data-testid="booking-uncertain"]')?.textContent?.length).toBeGreaterThan(0);
    expect(held.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(held.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    (held.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => narrowed.calls.some((call) => call.path === '/reservations'), 'recover');
    const recovered = take(narrowed.calls, (call) => call.path === '/reservations');
    expect(recovered.init?.idempotencyKey).toBe(changed.init?.idempotencyKey);
    expect(posted(recovered)).toEqual(posted(changed));
    recovered.resolve({ ...pairReceipt(), party_size: 8, reference: 'PAIR01' });
    await settle();
    expect(held.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(held.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(held.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('PAIR01');
    await held.cleanup();
  });

  it('refreshes a taken pair without confirming it and keeps the edited party', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openPair(view.target, queued.calls);
    cell(view.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '7');
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'conflict');
    take(queued.calls, (call) => call.path === '/reservations').reject(
      new ApiError('Those tables were just taken.', 'table_unavailable', 409),
    );
    await until(() => queued.calls.some((call) => call.path.startsWith('/availability?')), 'refresh');
    const refresh = take(queued.calls, (call) => call.path.startsWith('/availability?'));
    expect(refresh.path).toContain('party_size=6');
    expect(view.target.querySelector('[data-testid="booking-error"]')?.textContent).toContain('just taken');
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('7');
    refresh.resolve(afterPairBooked());
    await settle();
    expect(cell(view.target, 'slot-t_1+t_2-18:00').getAttribute('data-available')).toBe('false');
    expect(cell(view.target, 'slot-t_2+t_3-18:00').getAttribute('data-available')).toBe('false');
    expect(cell(view.target, 'slot-t_1+t_2-19:30').getAttribute('data-available')).toBe('true');
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('7');
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="zero-available"]')).toBeNull();
    await view.cleanup();
  });

  it('asks a signed-out diner to sign in and does not invent a pair when nothing is free', async () => {
    const guest = recordingTransport();
    const signedOut = render(LiveSearch, { transport: guest.transport, signedIn: false });
    await openPair(signedOut.target, guest.calls);
    cell(signedOut.target, 'slot-t_1-18:00').click();
    await tick();
    expect(signedOut.target.querySelector('[data-testid="auth-error"]')).toBeNull();
    cell(signedOut.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    expect(signedOut.target.querySelector('[data-testid="auth-error"]')?.textContent).toContain('Sign in to hold a table.');
    expect(signedOut.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(signedOut.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    await signedOut.cleanup();

    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openPair(view.target, queued.calls, {
      restaurant_id: 'r_anker',
      date: '2027-06-17',
      timezone: 'Europe/Berlin',
      slots: [
        {
          starts_at_local: '2027-06-17T18:00',
          starts_at: '2027-06-17T18:00:00+02:00',
          available_table_ids: [],
          available_options: [],
        },
      ],
    });
    expect(view.target.querySelector('[data-testid="zero-available"]')).toBeTruthy();
    expect(view.target.querySelectorAll('[data-testid^="slot-"]')).toHaveLength(5);
    for (const node of view.target.querySelectorAll('[data-testid^="slot-"]')) {
      expect(node.getAttribute('data-available')).toBe('false');
    }
    cell(view.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="auth-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    await view.cleanup();
  });

  it('keeps the later search when an earlier pair result or refusal arrives afterwards', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openPair(view.target, queued.calls);
    cell(view.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'pair post');
    const booking = take(queued.calls, (call) => call.path === '/reservations');
    setControl(view.target.querySelector('[data-testid="restaurant-select"]') as HTMLSelectElement, 'r_nord');
    setControl(view.target.querySelector('[data-testid="party-size-input"]') as HTMLInputElement, '2');
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/restaurants/r_nord'), 'nord');
    take(queued.calls, (call) => call.path === '/restaurants/r_nord').resolve({
      id: 'r_nord',
      name: 'Nordlicht',
      timezone: 'America/New_York',
      tables: [{ id: 't_window', label: 'Window', capacity: 2 }],
    });
    take(queued.calls, (call) => call.path.includes('restaurant_id=r_nord')).resolve({
      restaurant_id: 'r_nord',
      date: '2027-06-17',
      timezone: 'America/New_York',
      slots: [
        {
          starts_at_local: '2027-06-17T17:00',
          starts_at: '2027-06-17T17:00:00-04:00',
          available_table_ids: ['t_window'],
        },
      ],
    });
    await settle();
    booking.reject(new ApiError('Those tables were just taken.', 'table_unavailable', 409));
    await settle();
    expect(view.target.querySelector('h1')?.textContent).toContain('Nordlicht');
    expect(view.target.querySelector('[data-testid="slot-t_window-17:00"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="slot-t_1+t_2-18:00"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="search-error"]')).toBeNull();
    await view.cleanup();
  });

  it('selects a pair immediately when motion is reduced', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openPair(view.target, queued.calls);
    const previous = window.matchMedia;
    window.matchMedia = (query: string) =>
      ({
        matches: query.includes('prefers-reduced-motion'),
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
    const chosen = cell(view.target, 'slot-t_2+t_3-19:30');
    chosen.click();
    await tick();
    expect(chosen.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_2+t_3"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 2 and Table 3');
    window.matchMedia = previous;
    await view.cleanup();
  });

  it('looks up and cancels a pair with every table label', async () => {
    const queued = recordingTransport();
    const view = render(LookupScreen, { transport: queued.transport, token: 'opaque-session' });
    setControl(view.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, 'PAIR01');
    (view.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations/PAIR01'), 'lookup');
    take(queued.calls, (call) => call.path === '/reservations/PAIR01').resolve(pairReceipt());
    await until(() => queued.calls.some((call) => call.path === '/restaurants/r_anker'), 'detail');
    take(queued.calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
    await settle();
    expect(view.target.querySelector('[data-testid="reservation-status"]')?.textContent).toBe('confirmed');
    const tables = view.target.querySelector('[data-testid="reservation-tables"]')?.textContent ?? '';
    expect(tables).toContain('Table 1');
    expect(tables).toContain('Table 2');
    expect(tables).not.toContain('t_1');
    expect(view.target.querySelector('[data-testid="reservation-detail"]')?.textContent).toContain('Zum Anker');
    expect(view.target.querySelector('[data-testid="reservation-detail"]')?.textContent).toContain('18:00');
    (view.target.querySelector('[data-testid="reservation-cancel-button"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path.endsWith('/cancel')), 'cancel');
    take(queued.calls, (call) => call.path.endsWith('/cancel')).resolve(pairReceipt('cancelled'));
    await settle();
    expect(view.target.querySelector('[data-testid="reservation-status"]')?.textContent).toBe('cancelled');
    expect(view.target.querySelector('[data-testid="reservation-cancel-button"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="reservation-tables"]')?.textContent).toContain('Table 2');
    await view.cleanup();
  });

  it('passes pair confirmed, refused, uncertain and lookup through the contrast gate', async () => {
    async function gate(target: ParentNode): Promise<void> {
      theme.set('light');
      await a11yGate(target as HTMLElement);
      theme.set('dark');
      await a11yGate(target as HTMLElement);
    }

    const confirmed = recordingTransport();
    const confirmedView = render(LiveSearch, { transport: confirmed.transport, signedIn: true, token: 'opaque-session' });
    await openPair(confirmedView.target, confirmed.calls);
    cell(confirmedView.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    (confirmedView.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => confirmed.calls.some((call) => call.path === '/reservations'), 'confirm');
    take(confirmed.calls, (call) => call.path === '/reservations').resolve(pairReceipt());
    await settle();
    expect(confirmedView.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('PAIR01');
    await gate(confirmedView.target);
    await confirmedView.cleanup();

    const refused = recordingTransport();
    const refusedView = render(LiveSearch, { transport: refused.transport, signedIn: true, token: 'opaque-session' });
    await openPair(refusedView.target, refused.calls);
    cell(refusedView.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    (refusedView.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => refused.calls.some((call) => call.path === '/reservations'), 'refuse');
    take(refused.calls, (call) => call.path === '/reservations').reject(
      new ApiError('Those tables were just taken.', 'table_unavailable', 409),
    );
    await settle();
    expect(refusedView.target.querySelector('[data-testid="booking-error"]')).toBeTruthy();
    await gate(refusedView.target);
    await refusedView.cleanup();

    const lost = recordingTransport();
    const lostView = render(LiveSearch, { transport: lost.transport, signedIn: true, token: 'opaque-session' });
    await openPair(lostView.target, lost.calls);
    cell(lostView.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    (lostView.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => lost.calls.some((call) => call.path === '/reservations'), 'lost');
    take(lost.calls, (call) => call.path === '/reservations').reject(new ApiError('closed', 'network', 0));
    await settle();
    expect(lostView.target.querySelector('[data-testid="booking-uncertain"]')).toBeTruthy();
    await gate(lostView.target);
    await lostView.cleanup();

    const lookup = recordingTransport();
    const lookupView = render(LookupScreen, { transport: lookup.transport, token: 'opaque-session' });
    setControl(lookupView.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, 'PAIR01');
    (lookupView.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => lookup.calls.some((call) => call.path === '/reservations/PAIR01'), 'lookup gate');
    take(lookup.calls, (call) => call.path === '/reservations/PAIR01').resolve(pairReceipt());
    await until(() => lookup.calls.some((call) => call.path === '/restaurants/r_anker'), 'lookup detail');
    take(lookup.calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
    await settle();
    await gate(lookupView.target);
    await lookupView.cleanup();
  }, 60000);
});
