import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ApiError } from '@nrynss/chaaya/keel';
import { a11yGate } from '@nrynss/chaaya/testing';
import { theme } from '@nrynss/chaaya/theme';
import { tick } from 'svelte';
import { afterEach, describe, expect, it } from 'vitest';
import { bodyFromRecord, parseReservation } from '../lib/booking';
import LiveSearch from '../lib/components/LiveSearch.svelte';
import LookupScreen from '../lib/components/LookupScreen.svelte';
import Shell from '../lib/components/Shell.svelte';
import { rememberHold, rememberSearch } from '../lib/hold';
import { motionDuration, staggerDelay } from '../lib/motion';
import type { Transport, TransportRequest } from '../lib/transport';
import { render } from './render';

/**
 * Fixture transport for an applied seating repair and a server-returned series clock.
 * The screens read the returned JSON. They do not invent a closure, a policy, or a later date.
 */

interface Call {
  path: string;
  init?: TransportRequest;
  resolve: (value: unknown) => void;
  reject: (error: unknown) => void;
}

const DATE = '2027-06-17';
const CLOCK_DATE = '2027-06-24';
const REFERENCE = 'REPAIR01';
const CREATED_AT = '2026-10-05T08:15:00+00:00';
const CLOCKS = ['18:00', '19:00', '20:30'];

const policy0 = {
  policy_version: 0,
  slot_minutes: 30,
  reservation_duration_minutes: 90,
  cancellation_cutoff_minutes: 120,
  opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
  capacities: { t_1: 2, t_2: 4, t_3: 6 },
};

const clockTerms = {
  policy_version: 1,
  slot_minutes: 30,
  reservation_duration_minutes: 60,
  cancellation_cutoff_minutes: 120,
  opening_hours: [{ weekday: 'thu', opens: '18:00', closes: '23:00' }],
  capacities: { t_1: 2, t_2: 4, t_3: 6 },
};

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
    { id: 't_3', label: '3', capacity: 6 },
  ],
  combinable: [
    ['t_1', 't_2'],
    ['t_2', 't_3'],
  ],
};

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

function posted(call: Call): Record<string, unknown> {
  return call.init?.body as Record<string, unknown>;
}

function cell(target: ParentNode, id: string): HTMLButtonElement {
  const node = target.querySelector(`[data-testid="${id}"]`);
  if (!(node instanceof HTMLButtonElement)) throw new Error(`missing ${id}`);
  return node;
}

function textOf(target: ParentNode, id: string): string {
  return target.querySelector(`[data-testid="${id}"]`)?.textContent ?? '';
}

function elapsedMinutes(startsAt: string, endsAt: string): number {
  return (Date.parse(endsAt) - Date.parse(startsAt)) / 60_000;
}

/** The parsed record's start, end and accepted terms, not merely the key names on the fixture object. */
function expectAcceptedSpan(
  record: Record<string, unknown>,
  version: number,
  duration: number,
  startLocal: string,
  endStamp: string,
): void {
  const parsed = parseReservation(record);
  const terms = parsed.acceptedTerms ?? {};
  expect(Object.keys(terms).sort()).toEqual([
    'cancellation_cutoff_minutes',
    'capacities',
    'opening_hours',
    'policy_version',
    'reservation_duration_minutes',
    'slot_minutes',
  ]);
  expect(terms).not.toHaveProperty('effective_from');
  expect(terms.policy_version).toBe(version);
  expect(terms.reservation_duration_minutes).toBe(duration);
  expect(parsed.starts_at_local).toBe(startLocal);
  expect(parsed.starts_at).toBe(`${startLocal}:00+02:00`);
  expect(parsed.ends_at).toBe(endStamp);
  expect(elapsedMinutes(parsed.starts_at, parsed.ends_at)).toBe(duration);
}

function slot(date: string, time: string, availableIds: string[], options: { table_ids: string[]; capacity: number }[]) {
  return {
    starts_at_local: `${date}T${time}`,
    starts_at: `${date}T${time}:00+02:00`,
    available_table_ids: [...availableIds],
    available_options: options.map((option) => ({ table_ids: [...option.table_ids], capacity: option.capacity })),
  };
}

const openPair = [
  { table_ids: ['t_3'], capacity: 6 },
  { table_ids: ['t_1', 't_2'], capacity: 6 },
  { table_ids: ['t_2', 't_3'], capacity: 10 },
];

const afterPair = [
  { table_ids: ['t_3'], capacity: 6 },
  { table_ids: ['t_1', 't_2'], capacity: 6 },
  { table_ids: ['t_2', 't_3'], capacity: 10 },
];

/** Before a plan is applied, party 6 can hold table 3 or either declared pair. */
function beforeFloor(date = DATE) {
  return {
    restaurant_id: 'r_anker',
    date,
    timezone: 'Europe/Berlin',
    slots: CLOCKS.map((time) => slot(date, time, ['t_3'], openPair)),
  };
}

/**
 * After table 2 is closed across 19:00 and the repaired booking holds table 3.
 * 20:30 is the first returned slot after that closure.
 */
function afterFloor(date = DATE) {
  return {
    restaurant_id: 'r_anker',
    date,
    timezone: 'Europe/Berlin',
    slots: [
      slot(date, '18:00', [], []),
      slot(date, '19:00', [], []),
      slot(date, '20:30', ['t_3'], afterPair),
    ],
  };
}

function partyTwoFloor() {
  return {
    restaurant_id: 'r_anker',
    date: DATE,
    timezone: 'Europe/Berlin',
    slots: [slot(DATE, '18:00', ['t_1'], [{ table_ids: ['t_1'], capacity: 2 }])],
  };
}

function pairRecord(overrides: Record<string, unknown> = {}) {
  return {
    reservation_id: 'res_repair',
    reference: REFERENCE,
    restaurant_id: 'r_anker',
    table_ids: ['t_1', 't_2'],
    party_size: 6,
    status: 'confirmed',
    starts_at_local: `${DATE}T19:00`,
    starts_at: `${DATE}T19:00:00+02:00`,
    ends_at: `${DATE}T20:30:00+02:00`,
    created_at: CREATED_AT,
    revision: 1,
    accepted_terms: policy0,
    ...overrides,
  };
}

/** The applied repair keeps identity, party, times and terms, and moves the set to table 3. */
function repairedRecord() {
  return {
    reservation_id: 'res_repair',
    reference: REFERENCE,
    restaurant_id: 'r_anker',
    table_id: 't_3',
    table_ids: ['t_3'],
    party_size: 6,
    status: 'confirmed',
    starts_at_local: `${DATE}T19:00`,
    starts_at: `${DATE}T19:00:00+02:00`,
    ends_at: `${DATE}T20:30:00+02:00`,
    created_at: CREATED_AT,
    revision: 2,
    accepted_terms: policy0,
  };
}

function clockRecord(overrides: Record<string, unknown> = {}) {
  return pairRecord({
    starts_at_local: `${CLOCK_DATE}T20:00`,
    starts_at: `${CLOCK_DATE}T20:00:00+02:00`,
    ends_at: `${CLOCK_DATE}T21:00:00+02:00`,
    revision: 3,
    accepted_terms: clockTerms,
    ...overrides,
  });
}

function originalClockReceipt() {
  return pairRecord({
    starts_at_local: `${CLOCK_DATE}T19:00`,
    starts_at: `${CLOCK_DATE}T19:00:00+02:00`,
    ends_at: `${CLOCK_DATE}T20:30:00+02:00`,
    revision: 1,
    accepted_terms: policy0,
  });
}

async function openCatalog(target: ParentNode, calls: Call[]): Promise<void> {
  await until(() => calls.some((call) => call.path === '/restaurants'), 'catalog');
  const catalog = take(calls, (call) => call.path === '/restaurants');
  expect(catalog.init?.token ?? null).toBeNull();
  catalog.resolve({
    restaurants: [{ id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' }],
  });
  await until(() => target.querySelectorAll('[data-testid="restaurant-select"] option').length === 1, 'options');
}

async function searchParty(target: ParentNode, calls: Call[], party: number, date = DATE): Promise<void> {
  setControl(target.querySelector('[data-testid="date-input"]') as HTMLInputElement, date);
  setControl(target.querySelector('[data-testid="party-size-input"]') as HTMLInputElement, String(party));
  (target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
  await until(
    () => calls.filter((call) => call.path.startsWith('/availability?')).length > 0 && calls.some((call) => call.path === '/restaurants/r_anker'),
    'search',
  );
}

function assertPublicRead(call: Call): void {
  expect(call.init?.token ?? null).toBeNull();
  expect(call.path.includes('/policies') || call.path.includes('/series') || call.path.includes('replan')).toBe(false);
}

async function finishSearch(calls: Call[], availability: unknown): Promise<void> {
  const detail = take(calls, (call) => call.path === '/restaurants/r_anker');
  const availabilityCall = take(calls, (call) => call.path.startsWith('/availability?'));
  assertPublicRead(detail);
  assertPublicRead(availabilityCall);
  expect(availabilityCall.path).not.toContain('explain');
  detail.resolve(anker);
  availabilityCall.resolve(availability);
  await settle();
}

async function gate(target: HTMLElement): Promise<void> {
  theme.set('light');
  await a11yGate(target);
  theme.set('dark');
  await a11yGate(target);
}

afterEach(() => {
  rememberSearch(null);
  rememberHold(null);
  theme.set('system');
  document.body.innerHTML = '';
});

describe('applied repair and server clock on the existing screens', () => {
  it('keeps the original detail and the returned repair record apart from its receipt', () => {
    expect(anker.tables.map((table) => table.capacity)).toEqual([2, 4, 6]);
    expect(anker.slot_minutes).toBe(30);
    expect(anker.reservation_duration_minutes).toBe(90);
    expect(Object.keys(policy0).sort()).toEqual(Object.keys(clockTerms).sort());
    expect(policy0.policy_version).toBe(0);
    expect(policy0.reservation_duration_minutes).toBe(90);
    expect(clockTerms.policy_version).toBe(1);
    expect(clockTerms.reservation_duration_minutes).toBe(60);
    expect(clockTerms).not.toHaveProperty('effective_from');

    const before = parseReservation(pairRecord());
    const after = parseReservation(repairedRecord());
    expect(before.reference).toBe(REFERENCE);
    expect(after.reference).toBe(REFERENCE);
    expect(after.party_size).toBe(before.party_size);
    expect(after.starts_at_local).toBe(before.starts_at_local);
    expect(after.ends_at).toBe(before.ends_at);
    expect(after.acceptedTerms).toEqual(policy0);
    expect(after.acceptedTerms).not.toHaveProperty('effective_from');
    expect(after.revision).toBe(2);
    expect(after.table_id).toBe('t_3');
    expect(after.tableIds).toEqual(['t_3']);
    expect(before.table_id).toBe('');
    expect(before.tableIds).toEqual(['t_1', 't_2']);
    expect(bodyFromRecord(before)).toEqual({
      restaurant_id: 'r_anker',
      table_ids: ['t_1', 't_2'],
      starts_at_local: `${DATE}T19:00`,
      party_size: 6,
    });
    expect(bodyFromRecord(after)).toEqual({
      restaurant_id: 'r_anker',
      table_id: 't_3',
      starts_at_local: `${DATE}T19:00`,
      party_size: 6,
    });
    expect(pairRecord()).not.toHaveProperty('user_id');
    expect(repairedRecord()).not.toHaveProperty('user_id');

    expectAcceptedSpan(pairRecord(), 0, 90, `${DATE}T19:00`, `${DATE}T20:30:00+02:00`);
    expectAcceptedSpan(repairedRecord(), 0, 90, `${DATE}T19:00`, `${DATE}T20:30:00+02:00`);
    expectAcceptedSpan(clockRecord(), 1, 60, `${CLOCK_DATE}T20:00`, `${CLOCK_DATE}T21:00:00+02:00`);
    expectAcceptedSpan(originalClockReceipt(), 0, 90, `${CLOCK_DATE}T19:00`, `${CLOCK_DATE}T20:30:00+02:00`);
    const current = parseReservation(clockRecord());
    expect(current.acceptedTerms).toEqual(clockTerms);
    expect(current.tableIds).toEqual(['t_1', 't_2']);
    expect(current.revision).toBe(3);
    const receipt = parseReservation(originalClockReceipt());
    expect(receipt.acceptedTerms).toEqual(policy0);
    expect(bodyFromRecord(receipt).starts_at_local).toBe(`${CLOCK_DATE}T19:00`);
    expect(bodyFromRecord(receipt)).not.toEqual(bodyFromRecord(current));
  });

  it('mirrors returned closure membership on the floor and the grid', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(view.target, queued.calls);
    await searchParty(view.target, queued.calls, 6);
    const availability = queued.calls.find((call) => call.path.startsWith('/availability?'));
    expect(availability?.path).toContain('party_size=6');
    expect(availability?.path).toContain(`date=${DATE}`);
    await finishSearch(queued.calls, beforeFloor());

    for (const time of CLOCKS) {
      expect(cell(view.target, `slot-t_1-${time}`).getAttribute('data-available')).toBe('false');
      expect(cell(view.target, `slot-t_2-${time}`).getAttribute('data-available')).toBe('false');
      expect(cell(view.target, `slot-t_3-${time}`).getAttribute('data-available')).toBe('true');
      expect(cell(view.target, `slot-t_1+t_2-${time}`).getAttribute('data-available')).toBe('true');
      expect(cell(view.target, `slot-t_2+t_3-${time}`).getAttribute('data-available')).toBe('true');
    }
    const heads = [...view.target.querySelectorAll('.rowhead')].map((node) => node.textContent ?? '');
    expect(heads[0]).toContain('2 seats');
    expect(heads[1]).toContain('4 seats');
    expect(heads[2]).toContain('6 seats');
    expect(heads[3]).toContain('6 seats together');
    expect(heads[4]).toContain('10 seats together');
    expect(view.target.querySelector('[data-testid="plan-t_1"]')?.querySelectorAll('.seat')).toHaveLength(2);
    expect(view.target.querySelector('[data-testid="plan-t_3"]')?.querySelectorAll('.seat')).toHaveLength(6);

    cell(view.target, 'slot-t_1-19:00').click();
    cell(view.target, 'slot-t_2-19:00').click();
    await tick();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeNull();

    cell(view.target, 'slot-t_1+t_2-19:00').click();
    await tick();
    const summary = textOf(view.target, 'booking-summary');
    expect(summary).toContain('Table 1');
    expect(summary).toContain('Table 2');
    expect(summary).toContain('Zum Anker');
    expect(summary).toContain('Thursday 17 June 2027');
    expect(summary).toContain('7:00 PM');
    expect(summary).toContain('(19:00)');
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('6');
    expect(view.target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-selected')).toBe('false');
    expect(view.target.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-available')).toBe('true');
    expect(cell(view.target, 'slot-t_1+t_2-19:00').getAttribute('data-selected')).toBe('true');
    await view.cleanup();
  });

  it('lets a later search replace the grid and the form when an earlier floor arrives afterwards', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(view.target, queued.calls);
    await searchParty(view.target, queued.calls, 6);
    await searchParty(view.target, queued.calls, 2);
    const details = queued.calls.filter((call) => call.path === '/restaurants/r_anker');
    const availabilities = queued.calls.filter((call) => call.path.startsWith('/availability?'));
    expect(availabilities.map((call) => call.path)).toEqual([
      `/availability?restaurant_id=r_anker&date=${DATE}&party_size=6`,
      `/availability?restaurant_id=r_anker&date=${DATE}&party_size=2`,
    ]);
    for (const call of [...details, ...availabilities]) assertPublicRead(call);
    details[1].resolve(anker);
    availabilities[1].resolve(partyTwoFloor());
    await settle();
    expect(view.target.querySelector('[data-testid="slot-t_1-18:00"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="slot-t_1-19:00"]')).toBeNull();
    expect(view.target.querySelector('h1')?.textContent).toContain('Zum Anker');
    expect(view.target.textContent).toContain('party of 2');
    cell(view.target, 'slot-t_1-18:00').click();
    await tick();
    expect(textOf(view.target, 'booking-summary')).toContain('Table 1');
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('2');

    details[0].resolve(anker);
    availabilities[0].resolve(beforeFloor());
    await settle();
    expect(view.target.querySelector('[data-testid="slot-t_1-19:00"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="slot-t_1+t_2-19:00"]')).toBeNull();
    expect(textOf(view.target, 'booking-summary')).toContain('Table 1');
    expect(textOf(view.target, 'booking-summary')).not.toContain('Table 2');
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('2');
    expect(view.target.textContent).toContain('party of 2');
    await view.cleanup();
  });

  it('keeps the form on a conflict and refreshes the returned closure', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(view.target, queued.calls);
    await searchParty(view.target, queued.calls, 6);
    await finishSearch(queued.calls, beforeFloor());
    cell(view.target, 'slot-t_1+t_2-19:00').click();
    await tick();
    const selectedCapacity = ['t_1', 't_2'].reduce(
      (sum, id) => sum + (anker.tables.find((table) => table.id === id)?.capacity ?? 0),
      0,
    );
    const editedParty = 5;
    expect(selectedCapacity).toBeGreaterThanOrEqual(editedParty);
    expect(editedParty).not.toBe(6);
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, String(editedParty));
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'conflict');
    const refused = take(queued.calls, (call) => call.path === '/reservations');
    expect(posted(refused)).toEqual({
      restaurant_id: 'r_anker',
      table_ids: ['t_1', 't_2'],
      starts_at_local: `${DATE}T19:00`,
      party_size: editedParty,
    });
    expect(refused.init?.token).toBe('opaque-session');
    refused.reject(new ApiError('That seating was just taken.', 'table_unavailable', 409));
    await until(() => queued.calls.some((call) => call.path.startsWith('/availability?')), 'refresh');
    const refresh = take(queued.calls, (call) => call.path.startsWith('/availability?'));
    assertPublicRead(refresh);
    expect(refresh.path).toContain('party_size=6');
    expect(refresh.path).not.toContain('explain');
    refresh.resolve(afterFloor());
    await settle();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('5');
    expect(textOf(view.target, 'booking-summary')).toContain('Table 1');
    expect(textOf(view.target, 'booking-summary')).toContain('Table 2');
    expect(textOf(view.target, 'booking-error')).toContain('just taken');
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(cell(view.target, 'slot-t_1+t_2-19:00').getAttribute('data-available')).toBe('false');
    expect(cell(view.target, 'slot-t_3-19:00').getAttribute('data-available')).toBe('false');
    expect(cell(view.target, 'slot-t_2+t_3-19:00').getAttribute('data-available')).toBe('false');
    expect(cell(view.target, 'slot-t_3-20:30').getAttribute('data-available')).toBe('true');
    expect(cell(view.target, 'slot-t_1+t_2-20:30').getAttribute('data-available')).toBe('true');
    expect(cell(view.target, 'slot-t_1-20:30').getAttribute('data-available')).toBe('false');
    expect(cell(view.target, 'slot-t_2-20:30').getAttribute('data-available')).toBe('false');
    cell(view.target, 'slot-t_1+t_2-19:00').click();
    cell(view.target, 'slot-t_2-19:00').click();
    await tick();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('5');
    expect(textOf(view.target, 'booking-summary')).toContain('Table 1 and Table 2');
    expect(view.target.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('data-available')).toBe('false');
    expect(view.target.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-available')).toBe('false');
    await view.cleanup();
  });

  it('recovers a lost pair with the original body and key, not the applied table', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(view.target, queued.calls);
    await searchParty(view.target, queued.calls, 6);
    await finishSearch(queued.calls, beforeFloor());
    cell(view.target, 'slot-t_1+t_2-19:00').click();
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'lost');
    const lost = take(queued.calls, (call) => call.path === '/reservations');
    const sent = posted(lost);
    const key = lost.init?.idempotencyKey;
    expect(sent).toEqual({
      restaurant_id: 'r_anker',
      table_ids: ['t_1', 't_2'],
      starts_at_local: `${DATE}T19:00`,
      party_size: 6,
    });
    expect(sent).not.toHaveProperty('table_id');
    lost.reject(new ApiError('The connection closed.', 'network', 0));
    await settle();
    expect(textOf(view.target, 'booking-uncertain').trim().length).toBeGreaterThan(0);
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('6');

    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'retry');
    const retry = take(queued.calls, (call) => call.path === '/reservations');
    expect(retry.init?.idempotencyKey).toBe(key);
    expect(posted(retry)).toEqual(sent);
    expect(posted(retry)).not.toEqual(bodyFromRecord(parseReservation(repairedRecord())));
    retry.resolve(pairRecord());
    await settle();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(textOf(view.target, 'confirmation-reference')).toBe(REFERENCE);
    expect(textOf(view.target, 'confirmation-tables')).toContain('Table 1');
    expect(textOf(view.target, 'confirmation-tables')).toContain('Table 2');
    expect(textOf(view.target, 'confirmation-tables')).not.toBe('Table 3');
    expect(textOf(view.target, 'confirmation-details')).toContain('7:00 PM');
    expect(textOf(view.target, 'confirmation-details')).toContain('(19:00)');
    expect(textOf(view.target, 'confirmation')).not.toContain('policy_version');

    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'replay');
    const replay = take(queued.calls, (call) => call.path === '/reservations');
    expect(replay.init?.idempotencyKey).toBe(key);
    expect(posted(replay)).toEqual(sent);
    replay.resolve(pairRecord());
    await settle();
    expect(textOf(view.target, 'confirmation-reference')).toBe(REFERENCE);
    expect(textOf(view.target, 'confirmation-tables')).toContain('Table 1 and Table 2');
    expect(queued.calls.some((call) => call.path.startsWith('/reservations/'))).toBe(false);
    await view.cleanup();
  });

  it('shows the repaired table on lookup and leaves the earlier pair on the receipt screen', async () => {
    const before = recordingTransport();
    const beforeView = render(LookupScreen, { transport: before.transport, token: 'opaque-session' });
    setControl(beforeView.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, REFERENCE);
    (beforeView.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => before.calls.some((call) => call.path === `/reservations/${REFERENCE}`), 'before lookup');
    const beforeGet = take(before.calls, (call) => call.path === `/reservations/${REFERENCE}`);
    expect(beforeGet.init?.token).toBe('opaque-session');
    expect(beforeGet.init?.method ?? 'GET').not.toBe('POST');
    beforeGet.resolve(pairRecord());
    await until(() => before.calls.some((call) => call.path === '/restaurants/r_anker'), 'before detail');
    const beforeDetail = take(before.calls, (call) => call.path === '/restaurants/r_anker');
    expect(beforeDetail.init?.token ?? null).toBeNull();
    beforeDetail.resolve(anker);
    await settle();
    expect(textOf(beforeView.target, 'reservation-status')).toBe('confirmed');
    expect(textOf(beforeView.target, 'reservation-tables')).toBe('Table 1 and Table 2');
    const beforeDetailText = textOf(beforeView.target, 'reservation-detail');
    expect(beforeDetailText).toContain('Zum Anker');
    expect(beforeDetailText).toContain('Thursday 17 June 2027');
    expect(beforeDetailText).toContain('7:00 PM');
    expect(beforeDetailText).toContain('(19:00)');
    expect(beforeView.target.querySelector('[data-testid="reservation-cancel-button"]')).toBeTruthy();
    await beforeView.cleanup();

    const after = recordingTransport();
    const afterView = render(LookupScreen, { transport: after.transport, token: 'opaque-session' });
    setControl(afterView.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, REFERENCE);
    (afterView.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => after.calls.some((call) => call.path === `/reservations/${REFERENCE}`), 'after lookup');
    take(after.calls, (call) => call.path === `/reservations/${REFERENCE}`).resolve(repairedRecord());
    await until(() => after.calls.some((call) => call.path === '/restaurants/r_anker'), 'after detail');
    take(after.calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
    await settle();
    expect(textOf(afterView.target, 'reservation-status')).toBe('confirmed');
    expect(textOf(afterView.target, 'reservation-tables')).toBe('Table 3');
    const afterDetailText = textOf(afterView.target, 'reservation-detail');
    expect(afterDetailText).toContain('Table 3');
    expect(afterDetailText).not.toContain('Table 1');
    expect(afterDetailText).not.toContain('Table 2');
    expect(afterDetailText).toContain('Thursday 17 June 2027');
    expect(afterDetailText).toContain('7:00 PM');
    expect(afterDetailText).toContain('(19:00)');
    expect(afterDetailText).not.toContain('20:30');
    expect(after.calls.some((call) => call.path === '/reservations' || call.path.includes('replan'))).toBe(false);
    await afterView.cleanup();
  });

  it('shows the server clock for an amended, cancelled and exception record', async () => {
    async function lookup(record: Record<string, unknown>, reference = REFERENCE) {
      const queued = recordingTransport();
      const view = render(LookupScreen, { transport: queued.transport, token: 'opaque-session' });
      setControl(view.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, reference);
      (view.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
      await until(() => queued.calls.some((call) => call.path === `/reservations/${reference}`), 'clock lookup');
      take(queued.calls, (call) => call.path === `/reservations/${reference}`).resolve(record);
      await until(() => queued.calls.some((call) => call.path === '/restaurants/r_anker'), 'clock detail');
      const detail = take(queued.calls, (call) => call.path === '/restaurants/r_anker');
      expect(detail.init?.token ?? null).toBeNull();
      detail.resolve(anker);
      await settle();
      expect(queued.calls.some((call) => call.path.includes('/series') || call.path.includes('replan'))).toBe(false);
      return { view, calls: queued.calls };
    }

    const amended = await lookup(clockRecord());
    const amendedText = textOf(amended.view.target, 'reservation-detail');
    expect(textOf(amended.view.target, 'reservation-status')).toBe('confirmed');
    expect(textOf(amended.view.target, 'reservation-tables')).toBe('Table 1 and Table 2');
    expect(amendedText).toContain('Thursday 24 June 2027');
    expect(amendedText).toContain('8:00 PM');
    expect(amendedText).toContain('(20:00)');
    expect(amendedText).not.toContain('(19:00)');
    expect(amendedText).not.toContain('7:00 PM');
    expect(amendedText).not.toContain('21:00');
    const amendedRecord = clockRecord();
    expectAcceptedSpan(amendedRecord, 1, 60, `${CLOCK_DATE}T20:00`, `${CLOCK_DATE}T21:00:00+02:00`);
    expect(parseReservation(amendedRecord).tableIds).toEqual(['t_1', 't_2']);
    expectAcceptedSpan(originalClockReceipt(), 0, 90, `${CLOCK_DATE}T19:00`, `${CLOCK_DATE}T20:30:00+02:00`);
    await amended.view.cleanup();

    const cancelledRecord = clockRecord({ status: 'cancelled', revision: 4 });
    const cancelled = await lookup(cancelledRecord);
    const cancelledText = textOf(cancelled.view.target, 'reservation-detail');
    expect(textOf(cancelled.view.target, 'reservation-status')).toBe('cancelled');
    expect(cancelled.view.target.querySelector('[data-testid="reservation-cancel-button"]')).toBeNull();
    expect(cancelledText).toContain('(20:00)');
    expect(cancelledText).toContain('8:00 PM');
    expect(cancelledText).not.toContain('(19:00)');
    expect(textOf(cancelled.view.target, 'reservation-tables')).toBe('Table 1 and Table 2');
    expectAcceptedSpan(cancelledRecord, 1, 60, `${CLOCK_DATE}T20:00`, `${CLOCK_DATE}T21:00:00+02:00`);
    expect(parseReservation(cancelledRecord).status).toBe('cancelled');
    expect(parseReservation(cancelledRecord).revision).toBe(4);
    await cancelled.view.cleanup();

    const exceptionRecord = clockRecord({
      starts_at_local: `${CLOCK_DATE}T18:30`,
      starts_at: `${CLOCK_DATE}T18:30:00+02:00`,
      ends_at: `${CLOCK_DATE}T19:30:00+02:00`,
      exception: true,
    });
    const exception = await lookup(exceptionRecord);
    const exceptionText = textOf(exception.view.target, 'reservation-detail');
    expect(textOf(exception.view.target, 'reservation-status')).toBe('confirmed');
    expect(exceptionText).toContain('6:30 PM');
    expect(exceptionText).toContain('(18:30)');
    expect(exceptionText).not.toContain('(19:00)');
    expect(exceptionText).not.toContain('(20:00)');
    expect(exceptionText).toContain('Thursday 24 June 2027');
    expect(textOf(exception.view.target, 'reservation-tables')).toBe('Table 1 and Table 2');
    expect((exceptionRecord as Record<string, unknown>).exception).toBe(true);
    expect(exceptionRecord.ends_at).toBe(`${CLOCK_DATE}T19:30:00+02:00`);
    expectAcceptedSpan(exceptionRecord, 1, 60, `${CLOCK_DATE}T18:30`, `${CLOCK_DATE}T19:30:00+02:00`);
    expect(parseReservation(exceptionRecord).tableIds).toEqual(['t_1', 't_2']);
    expect(parseReservation(exceptionRecord).revision).toBe(3);
    await exception.view.cleanup();
  });

  it('keeps an old clock receipt when the current record would show a later time', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(view.target, queued.calls);
    await searchParty(view.target, queued.calls, 6, CLOCK_DATE);
    const availability = queued.calls.find((call) => call.path.startsWith('/availability?'));
    expect(availability?.path).toContain(`date=${CLOCK_DATE}`);
    expect(availability?.path).toContain('party_size=6');
    await finishSearch(queued.calls, beforeFloor(CLOCK_DATE));
    cell(view.target, 'slot-t_1+t_2-19:00').click();
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'clock create');
    const created = take(queued.calls, (call) => call.path === '/reservations');
    expect(posted(created).starts_at_local).toBe(`${CLOCK_DATE}T19:00`);
    expect(posted(created).table_ids).toEqual(['t_1', 't_2']);
    created.reject(new ApiError('The connection closed.', 'network', 0));
    await settle();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'clock retry');
    const retry = take(queued.calls, (call) => call.path === '/reservations');
    expect(retry.init?.idempotencyKey).toBe(created.init?.idempotencyKey);
    expect(posted(retry)).toEqual(posted(created));
    retry.resolve(originalClockReceipt());
    await settle();
    expect(textOf(view.target, 'confirmation-reference')).toBe(REFERENCE);
    expect(textOf(view.target, 'confirmation-details')).toContain('Thursday 24 June 2027');
    expect(textOf(view.target, 'confirmation-details')).toContain('7:00 PM');
    expect(textOf(view.target, 'confirmation-details')).toContain('(19:00)');
    expect(textOf(view.target, 'confirmation-details')).not.toContain('(20:00)');
    expect(textOf(view.target, 'confirmation-details')).not.toContain('8:00 PM');
    expect(textOf(view.target, 'confirmation-tables')).toContain('Table 1 and Table 2');
    expect(queued.calls.some((call) => call.path.startsWith('/reservations/') || call.path.includes('/series'))).toBe(false);
    await view.cleanup();
  });

  it('selects immediately when motion is reduced and keeps a blank signed-in name', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(view.target, queued.calls);
    await searchParty(view.target, queued.calls, 6);
    await finishSearch(queued.calls, beforeFloor());
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
    try {
      expect(motionDuration(240)).toBe(0);
      expect(staggerDelay(12)).toBe(0);
      const chosen = cell(view.target, 'slot-t_1+t_2-19:00');
      chosen.click();
      await tick();
      expect(chosen.getAttribute('data-selected')).toBe('true');
      expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
      expect(view.target.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('data-selected')).toBe('true');
    } finally {
      window.matchMedia = previous;
    }
    await view.cleanup();

    const blank = render(Shell, { path: '/', user: '' });
    const labels = [...blank.target.querySelectorAll('nav[aria-label="Primary"] a')].map((node) => (node.textContent ?? '').trim());
    expect(labels).toEqual(['Search', 'Look up']);
    expect(blank.target.querySelector('[data-testid="current-user"]')?.textContent).toBe('');
    expect(blank.target.querySelector('[data-testid="logout-button"]')).toBeTruthy();
    expect(blank.target.querySelector('a[href="/login"]')).toBeNull();
    await blank.cleanup();

    const webRoot = join(dirname(fileURLToPath(import.meta.url)), '../..');
    const app = readFileSync(join(webRoot, 'src/App.svelte'), 'utf8');
    expect(app).toContain("path === '/lookup'");
    expect(app).toContain("path === '/login'");
    expect(app).toContain("path === '/signup'");
    expect(app).not.toContain('/series');
    expect(app).not.toContain('replan');
  });

  it('passes repaired grid, confirmation and lookup through the contrast gate', async () => {
    const grid = recordingTransport();
    const gridView = render(LiveSearch, { transport: grid.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(gridView.target, grid.calls);
    await searchParty(gridView.target, grid.calls, 6);
    await finishSearch(grid.calls, afterFloor());
    expect(cell(gridView.target, 'slot-t_2-19:00').getAttribute('data-available')).toBe('false');
    cell(gridView.target, 'slot-t_3-20:30').click();
    await tick();
    expect(textOf(gridView.target, 'booking-summary')).toContain('Table 3');
    expect(textOf(gridView.target, 'booking-summary')).toContain('(20:30)');
    expect(gridView.target.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-available')).toBe('true');
    expect(gridView.target.querySelector('[data-testid="plan-t_1+t_2"]')?.getAttribute('data-available')).toBe('true');
    await gate(gridView.target);
    await gridView.cleanup();

    const confirmed = recordingTransport();
    const confirmedView = render(LiveSearch, { transport: confirmed.transport, signedIn: true, token: 'opaque-session' });
    await openCatalog(confirmedView.target, confirmed.calls);
    await searchParty(confirmedView.target, confirmed.calls, 6);
    await finishSearch(confirmed.calls, beforeFloor());
    cell(confirmedView.target, 'slot-t_1+t_2-19:00').click();
    await tick();
    (confirmedView.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => confirmed.calls.some((call) => call.path === '/reservations'), 'gate confirm');
    take(confirmed.calls, (call) => call.path === '/reservations').resolve(pairRecord());
    await settle();
    expect(textOf(confirmedView.target, 'confirmation-reference')).toBe(REFERENCE);
    await gate(confirmedView.target);
    await confirmedView.cleanup();

    const lookup = recordingTransport();
    const lookupView = render(LookupScreen, { transport: lookup.transport, token: 'opaque-session' });
    setControl(lookupView.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, REFERENCE);
    (lookupView.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => lookup.calls.some((call) => call.path === `/reservations/${REFERENCE}`), 'gate lookup');
    take(lookup.calls, (call) => call.path === `/reservations/${REFERENCE}`).resolve(repairedRecord());
    await until(() => lookup.calls.some((call) => call.path === '/restaurants/r_anker'), 'gate detail');
    take(lookup.calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
    await settle();
    expect(textOf(lookupView.target, 'reservation-tables')).toBe('Table 3');
    await gate(lookupView.target);
    await lookupView.cleanup();
  });
});
