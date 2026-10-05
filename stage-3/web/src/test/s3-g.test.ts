import { ApiError } from '@nrynss/chaaya/keel';
import { theme } from '@nrynss/chaaya/theme';
import { tick } from 'svelte';
import { afterEach, describe, expect, it } from 'vitest';
import { attemptFor, bodyFromRecord, parseReservation } from '../lib/booking';
import LiveSearch from '../lib/components/LiveSearch.svelte';
import Shell from '../lib/components/Shell.svelte';
import { rememberHold, rememberSearch } from '../lib/hold';
import { parseAvailability, parseDetail } from '../lib/search';
import { explicitPairCapacity } from '../lib/seating';
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

const CLOCKS = ['18:00', '19:00', '20:00', '21:00', '22:00'];

const acceptedTerms = {
  policy_version: 1,
  slot_minutes: 60,
  reservation_duration_minutes: 60,
  cancellation_cutoff_minutes: 60,
  opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
  capacities: { t_1: 4, t_2: 6, t_3: 4 },
};

/** Original fixture detail. Publication must not rewrite this object. */
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

function policyFloor(party: 4 | 8) {
  const singles =
    party === 4
      ? [
          { table_ids: ['t_1'], capacity: 4 },
          { table_ids: ['t_2'], capacity: 6 },
          { table_ids: ['t_3'], capacity: 4 },
        ]
      : [];
  const available = party === 4 ? ['t_1', 't_2', 't_3'] : [];
  return {
    restaurant_id: 'r_anker',
    date: '2027-06-17',
    timezone: 'Europe/Berlin',
    slots: CLOCKS.map((time) => ({
      starts_at_local: `2027-06-17T${time}`,
      starts_at: `2027-06-17T${time}:00+02:00`,
      available_table_ids: [...available],
      available_options: [
        ...singles.map((item) => ({ table_ids: [...item.table_ids], capacity: item.capacity })),
        { table_ids: ['t_1', 't_2'], capacity: 10 },
        { table_ids: ['t_2', 't_3'], capacity: 10 },
      ],
    })),
  };
}

function policyReceipt(revision = 1, terms: Record<string, unknown> = acceptedTerms) {
  return {
    reservation_id: 'res_pair',
    reference: 'POLICY01',
    restaurant_id: 'r_anker',
    table_ids: ['t_1', 't_2'],
    party_size: 8,
    status: 'confirmed' as const,
    starts_at_local: '2027-06-17T18:00',
    starts_at: '2027-06-17T18:00:00+02:00',
    ends_at: '2027-06-17T19:00:00+02:00',
    created_at: '2026-10-05T05:00:00+00:00',
    revision,
    accepted_terms: terms,
  };
}

function legacyReceipt() {
  return {
    reservation_id: 'res_7',
    reference: 'RIVER7',
    restaurant_id: 'r_anker',
    table_id: 't_1',
    party_size: 2,
    status: 'confirmed' as const,
    starts_at_local: '2027-06-17T18:00',
    starts_at: '2027-06-17T18:00:00+02:00',
    ends_at: '2027-06-17T19:30:00+02:00',
    created_at: '2026-09-21T11:04:03+00:00',
  };
}

function posted(call: Call): Record<string, unknown> {
  return call.init?.body as Record<string, unknown>;
}

function cell(target: ParentNode, id: string): HTMLButtonElement {
  const node = target.querySelector(`[data-testid="${id}"]`);
  if (!(node instanceof HTMLButtonElement)) throw new Error(`missing ${id}`);
  return node;
}

async function openSearch(target: ParentNode, calls: Call[], party: number, availability: unknown) {
  await until(() => calls.some((call) => call.path === '/restaurants'), 'catalog');
  take(calls, (call) => call.path === '/restaurants').resolve({
    restaurants: [{ id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' }],
  });
  await until(() => target.querySelectorAll('[data-testid="restaurant-select"] option').length === 1, 'options');
  setControl(target.querySelector('[data-testid="party-size-input"]') as HTMLInputElement, String(party));
  (target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
  await until(() => calls.some((call) => call.path.startsWith('/availability?')), 'search');
  const availabilityCall = calls.find((call) => call.path.startsWith('/availability?'));
  expect(availabilityCall?.path).toContain(`party_size=${party}`);
  expect(availabilityCall?.path).not.toContain('explain');
  expect(calls.some((call) => call.path.includes('/policies') || call.path.includes('/series') || call.path.includes('replan'))).toBe(
    false,
  );
  take(calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
  take(calls, (call) => call.path.startsWith('/availability?')).resolve(availability);
  await settle();
}

afterEach(() => {
  rememberSearch(null);
  rememberHold(null);
  theme.set('system');
  document.body.innerHTML = '';
});

describe('policy-shaped responses on the existing screens', () => {
  it('keeps the original detail and lets returned slots and pair capacity win', () => {
    const detail = parseDetail(anker);
    expect(detail.tables.map((table) => table.capacity)).toEqual([2, 4, 4]);
    expect(detail.combinable).toEqual([
      ['t_1', 't_2'],
      ['t_2', 't_3'],
    ]);
    expect(anker.slot_minutes).toBe(30);
    expect(anker.reservation_duration_minutes).toBe(90);
    expect(anker.cancellation_cutoff_minutes).toBe(120);

    const party8 = parseAvailability(policyFloor(8));
    expect(party8.slots.map((slot) => slot.starts_at_local.slice(11))).toEqual(CLOCKS);
    expect(party8.slots.every((slot) => slot.availableTableIds.length === 0)).toBe(true);
    expect(party8.slots[0].options.map((option) => ({ ids: option.tableIds, capacity: option.capacity }))).toEqual([
      { ids: ['t_1', 't_2'], capacity: 10 },
      { ids: ['t_2', 't_3'], capacity: 10 },
    ]);

    const party4 = parseAvailability(policyFloor(4));
    expect(party4.slots[0].availableTableIds).toEqual(['t_1', 't_2', 't_3']);
    expect(party4.slots[0].options.map((option) => option.capacity)).toEqual([4, 6, 4, 10, 10]);

    expect(explicitPairCapacity(party8.slots, ['t_2', 't_1'], 6)).toBe(10);
    expect(explicitPairCapacity([{ options: [{ tableIds: ['t_1'], capacity: 0 }] }], ['t_1', 't_2'], 6)).toBe(6);
    expect(explicitPairCapacity([], ['t_1', 't_2'], 6)).toBe(6);
  });

  it('keeps current terms as a snapshot and leaves a legacy receipt table-id only', () => {
    const source = policyReceipt();
    const current = parseReservation(source);
    expect(current.ends_at).toBe('2027-06-17T19:00:00+02:00');
    expect(current.revision).toBe(1);
    expect(current.acceptedTerms).toEqual(acceptedTerms);
    expect(current.acceptedTerms).not.toHaveProperty('effective_from');
    expect(current.table_id).toBe('');
    expect(current.tableIds).toEqual(['t_1', 't_2']);
    source.accepted_terms = { ...acceptedTerms, slot_minutes: 15 };
    source.revision = 9;
    expect(current.acceptedTerms).toEqual(acceptedTerms);
    expect(current.revision).toBe(1);

    const sent = bodyFromRecord(current);
    expect(sent).toEqual({
      restaurant_id: 'r_anker',
      table_ids: ['t_1', 't_2'],
      starts_at_local: '2027-06-17T18:00',
      party_size: 8,
    });
    expect(sent).not.toHaveProperty('revision');
    expect(sent).not.toHaveProperty('accepted_terms');
    expect(sent).not.toHaveProperty('table_id');

    const laterTerms = {
      ...acceptedTerms,
      slot_minutes: 45,
      reservation_duration_minutes: 120,
      capacities: { t_1: 2, t_2: 2, t_3: 2 },
    };
    const later = parseReservation(policyReceipt(2, laterTerms));
    expect(later.revision).toBe(2);
    expect(later.acceptedTerms).toEqual(laterTerms);
    expect(later.ends_at).toBe('2027-06-17T19:00:00+02:00');
    const pending = attemptFor(null, sent, 'key-original');
    const reused = attemptFor(pending, bodyFromRecord(later), 'key-next');
    expect(reused.key).toBe('key-original');
    expect(reused.body).toEqual(sent);
    expect(pending.body).toEqual(sent);

    const legacy = parseReservation(legacyReceipt());
    expect(legacy.revision).toBeUndefined();
    expect(legacy.acceptedTerms).toBeUndefined();
    expect(Object.keys(legacy)).not.toContain('revision');
    expect(Object.keys(legacy)).not.toContain('acceptedTerms');
    expect(legacy.table_id).toBe('t_1');
    expect(legacy.tableIds).toEqual(['t_1']);
    expect(bodyFromRecord(legacy)).toEqual({
      restaurant_id: 'r_anker',
      table_id: 't_1',
      starts_at_local: '2027-06-17T18:00',
      party_size: 2,
    });
    const legacyAttempt = attemptFor(null, bodyFromRecord(legacy), 'legacy-key');
    const legacyReplay = attemptFor(legacyAttempt, bodyFromRecord(parseReservation(legacyReceipt())), 'other-key');
    expect(legacyReplay.key).toBe('legacy-key');
    expect(legacyReplay.body).toEqual(legacyAttempt.body);
    expect(legacyReplay.body).not.toHaveProperty('table_ids');
  });

  it('offers a party of 8 the returned pair even when the original sum is 6', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openSearch(view.target, queued.calls, 8, policyFloor(8));
    const clocks = [...view.target.querySelectorAll('.colhead')].map((node) => (node.textContent ?? '').trim());
    expect(clocks.some((text) => text.includes('18:30'))).toBe(false);
    expect(view.target.querySelector('[data-testid="slot-t_1-18:30"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="slot-t_1-22:00"]')).toBeTruthy();
    for (const time of CLOCKS) {
      expect(cell(view.target, `slot-t_1-${time}`).getAttribute('data-available')).toBe('false');
      expect(cell(view.target, `slot-t_1+t_2-${time}`).getAttribute('data-available')).toBe('true');
      expect(cell(view.target, `slot-t_2+t_3-${time}`).getAttribute('data-available')).toBe('true');
    }
    const heads = [...view.target.querySelectorAll('.rowhead')].map((node) => node.textContent ?? '');
    expect(heads[0]).toContain('2 seats');
    expect(heads[0]).not.toContain('4 seats');
    expect(heads[3]).toContain('10 seats together');
    expect(heads[3]).not.toContain('6 seats together');
    expect(heads[4]).toContain('10 seats together');
    expect(view.target.querySelector('[data-testid="plan-t_1"]')?.querySelectorAll('.seat')).toHaveLength(2);
    expect(view.target.querySelector('[data-testid="plan-t_2"]')?.querySelectorAll('.seat')).toHaveLength(4);
    expect(view.target.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('aria-label')).toContain('10 seats');
    cell(view.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 1');
    expect(view.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 2');
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('8');
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'create');
    const created = take(queued.calls, (call) => call.path === '/reservations');
    expect(posted(created)).toEqual({
      restaurant_id: 'r_anker',
      table_ids: ['t_1', 't_2'],
      starts_at_local: '2027-06-17T18:00',
      party_size: 8,
    });
    expect(created.init?.idempotencyKey).toMatch(/^[0-9a-f]{32}$/);
    created.resolve(policyReceipt());
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('POLICY01');
    const confirmation = view.target.querySelector('[data-testid="confirmation"]')?.textContent ?? '';
    expect(confirmation).not.toContain('policy_version');
    expect(confirmation).not.toContain('19:30');
    expect(confirmation).not.toContain('19:00:00');
    expect(cell(view.target, 'slot-t_1+t_2-18:00').getAttribute('data-available')).toBe('true');
    expect(cell(view.target, 'slot-t_1+t_2-18:00').getAttribute('data-selected')).toBe('true');
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'replay');
    const replay = take(queued.calls, (call) => call.path === '/reservations');
    expect(replay.init?.idempotencyKey).toBe(created.init?.idempotencyKey);
    expect(posted(replay)).toEqual(posted(created));
    replay.resolve(policyReceipt());
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('POLICY01');
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    await view.cleanup();
  });

  it('shows party-of-4 membership from the response and leaves single table sizes on the detail', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openSearch(view.target, queued.calls, 4, policyFloor(4));
    for (const time of CLOCKS) {
      expect(cell(view.target, `slot-t_1-${time}`).getAttribute('data-available')).toBe('true');
      expect(cell(view.target, `slot-t_2-${time}`).getAttribute('data-available')).toBe('true');
      expect(cell(view.target, `slot-t_3-${time}`).getAttribute('data-available')).toBe('true');
    }
    const heads = [...view.target.querySelectorAll('.rowhead')].map((node) => node.textContent ?? '');
    expect(heads[0]).toContain('2 seats');
    expect(heads[1]).toContain('4 seats');
    expect(heads[1]).not.toContain('6 seats');
    expect(heads[2]).toContain('4 seats');
    expect(heads[3]).toContain('10 seats together');
    expect(view.target.querySelector('[data-testid="plan-t_2"]')?.querySelectorAll('.seat')).toHaveLength(4);
    await view.cleanup();
  });

  it('preserves the edited party on a conflict and refreshes the searched party', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openSearch(view.target, queued.calls, 8, policyFloor(8));
    cell(view.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '9');
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'conflict');
    const refused = take(queued.calls, (call) => call.path === '/reservations');
    expect(posted(refused).party_size).toBe(9);
    expect(posted(refused).table_ids).toEqual(['t_1', 't_2']);
    refused.reject(new ApiError('That seating was just taken.', 'table_unavailable', 409));
    await until(() => queued.calls.some((call) => call.path.startsWith('/availability?')), 'refresh');
    const refresh = take(queued.calls, (call) => call.path.startsWith('/availability?'));
    expect(refresh.path).toContain('party_size=8');
    expect(refresh.path).not.toContain('explain');
    refresh.resolve(policyFloor(8));
    await settle();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('9');
    expect(view.target.querySelector('[data-testid="booking-error"]')?.textContent).toContain('just taken');
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    await view.cleanup();
  });

  it('retries a lost pair response with the same body and key', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openSearch(view.target, queued.calls, 8, policyFloor(8));
    cell(view.target, 'slot-t_1+t_2-18:00').click();
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'lost');
    const lost = take(queued.calls, (call) => call.path === '/reservations');
    lost.reject(new ApiError('The connection closed.', 'network', 0));
    await settle();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')?.textContent?.trim().length).toBeGreaterThan(0);
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'retry');
    const retry = take(queued.calls, (call) => call.path === '/reservations');
    expect(retry.init?.idempotencyKey).toBe(lost.init?.idempotencyKey);
    expect(posted(retry)).toEqual(posted(lost));
    retry.resolve(policyReceipt());
    await settle();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('POLICY01');
    await view.cleanup();
  });

  it('still posts and replays a legacy single-table receipt without new fields', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openSearch(view.target, queued.calls, 2, {
      restaurant_id: 'r_anker',
      date: '2027-06-17',
      timezone: 'Europe/Berlin',
      slots: [
        {
          starts_at_local: '2027-06-17T18:00',
          starts_at: '2027-06-17T18:00:00+02:00',
          available_table_ids: ['t_1'],
        },
      ],
    });
    const heads = [...view.target.querySelectorAll('.rowhead')].map((node) => node.textContent ?? '');
    expect(heads[3]).toContain('6 seats together');
    cell(view.target, 'slot-t_1-18:00').click();
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'legacy create');
    const created = take(queued.calls, (call) => call.path === '/reservations');
    expect(posted(created)).toEqual({
      restaurant_id: 'r_anker',
      table_id: 't_1',
      starts_at_local: '2027-06-17T18:00',
      party_size: 2,
    });
    created.resolve(legacyReceipt());
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('RIVER7');
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'legacy replay');
    const replay = take(queued.calls, (call) => call.path === '/reservations');
    expect(replay.init?.idempotencyKey).toBe(created.init?.idempotencyKey);
    expect(posted(replay)).toEqual(posted(created));
    replay.resolve(legacyReceipt());
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('RIVER7');
    await view.cleanup();
  });

  it('keeps signed-in chrome for a blank display name', async () => {
    const blank = render(Shell, { path: '/', user: '' });
    const labels = [...blank.target.querySelectorAll('nav[aria-label="Primary"] a')].map((node) => (node.textContent ?? '').trim());
    expect(labels).toEqual(['Search', 'Look up']);
    expect(blank.target.querySelector('[data-testid="current-user"]')?.textContent).toBe('');
    expect(blank.target.querySelector('[data-testid="logout-button"]')).toBeTruthy();
    expect(blank.target.querySelector('a[href="/login"]')).toBeNull();
    await blank.cleanup();
  });
});
