import { ApiError } from '@nrynss/chaaya/keel';
import { a11yGate } from '@nrynss/chaaya/testing';
import { theme } from '@nrynss/chaaya/theme';
import { tick } from 'svelte';
import { afterEach, describe, expect, it } from 'vitest';
import LiveSearch from '../lib/components/LiveSearch.svelte';
import LookupScreen from '../lib/components/LookupScreen.svelte';
import { rememberHold, rememberSearch } from '../lib/hold';
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
  tables: [{ id: 't_window', label: 'Window', capacity: 2 }],
};

function slots(ids: string[], localTimes = ['2027-06-17T18:00', '2027-06-17T18:30']) {
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

function reservation(reference: string, party = 2, table = 't_1', starts = '2027-06-17T18:00') {
  return {
    reservation_id: `res_${reference}`,
    reference,
    restaurant_id: 'r_anker',
    table_id: table,
    party_size: party,
    status: 'confirmed' as const,
    starts_at_local: starts,
    starts_at: `${starts}:00+02:00`,
    ends_at: '2027-06-17T19:30:00+02:00',
    created_at: '2026-09-21T11:04:03+00:00',
  };
}

function posted(call: Call): { restaurant_id: string; table_id: string; starts_at_local: string; party_size: number } {
  return call.init?.body as {
    restaurant_id: string;
    table_id: string;
    starts_at_local: string;
    party_size: number;
  };
}

async function openMember(target: ParentNode, calls: Call[]) {
  await until(() => calls.some((call) => call.path === '/restaurants'), 'catalog');
  take(calls, (call) => call.path === '/restaurants').resolve({
    restaurants: [
      { id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' },
      { id: 'r_nord', name: 'Nordlicht', timezone: 'America/New_York' },
    ],
  });
  await until(() => target.querySelectorAll('[data-testid="restaurant-select"] option').length === 2, 'options');
  (target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
  await until(() => calls.length >= 2, 'search');
  take(calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
  take(calls, (call) => call.path.startsWith('/availability?')).resolve(slots(['t_1', 't_2']));
  await settle();
  (target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement).click();
  await tick();
}

afterEach(() => {
  rememberSearch(null);
  rememberHold(null);
  theme.set('system');
  document.body.innerHTML = '';
});

describe('live booking', () => {
  it('keeps the sent body when the party input changes and replays that same key', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openMember(view.target, queued.calls);
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'first booking');
    const first = take(queued.calls, (call) => call.path === '/reservations');
    const sent = posted(first);
    expect(sent).toEqual({
      restaurant_id: 'r_anker',
      table_id: 't_1',
      starts_at_local: '2027-06-17T18:00',
      party_size: 2,
    });
    expect(first.init?.idempotencyKey).toMatch(/^[0-9a-f]{32}$/);
    expect(first.init?.token).toBe('opaque-session');
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '5');
    await tick();
    expect(posted(first).party_size).toBe(2);
    first.resolve(reservation('RIVER7'));
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('RIVER7');
    expect(view.target.querySelector('[data-testid="confirmation-details"]')?.textContent).toContain('Zum Anker');
    expect(view.target.querySelector('[data-testid="confirmation-details"]')?.textContent).toContain('Table 1');
    expect(view.target.querySelector('[data-testid="confirmation-details"]')?.textContent).toContain('6:00 PM');
    expect(view.target.querySelector('[data-testid="confirmation-details"]')?.textContent).toContain('18:00');
    expect(view.target.querySelector('[data-testid="confirmation-tables"]')?.textContent).toContain('1');
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '2');
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'replay');
    const replay = take(queued.calls, (call) => call.path === '/reservations');
    expect(replay.init?.idempotencyKey).toBe(first.init?.idempotencyKey);
    expect(posted(replay)).toEqual(sent);
    replay.resolve(reservation('RIVER7'));
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('RIVER7');
    expect(queued.calls.filter((call) => call.path === '/reservations')).toHaveLength(0);
    await view.cleanup();
  });

  it('starts a new attempt when a submitted field changes and ignores the earlier response', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openMember(view.target, queued.calls);
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'first');
    const first = take(queued.calls, (call) => call.path === '/reservations');
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '5');
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'second');
    const second = take(queued.calls, (call) => call.path === '/reservations');
    expect(posted(first).party_size).toBe(2);
    expect(posted(second).party_size).toBe(5);
    expect(second.init?.idempotencyKey).not.toBe(first.init?.idempotencyKey);
    expect(view.target.textContent).toContain('Sending your request.');
    first.resolve(reservation('OLDREF', 2));
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(view.target.textContent).toContain('Sending your request.');
    second.reject(new ApiError('That table was just taken.', 'table_unavailable', 409));
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-error"]')?.textContent).toContain('just taken');
    expect(view.target.textContent).not.toContain('Sending your request.');
    await until(() => queued.calls.some((call) => call.path.startsWith('/availability?')), 'refresh');
    const refresh = take(queued.calls, (call) => call.path.startsWith('/availability?'));
    expect(refresh.path).toContain('party_size=2');
    expect(refresh.init?.token).toBeUndefined();
    refresh.resolve(slots(['t_2']));
    await settle();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('5');
    expect(view.target.querySelector('[data-testid="slot-t_1-18:00"]')?.getAttribute('data-available')).toBe('false');
    await view.cleanup();
  });

  it('shows uncertainty for a lost response and recovers the original reference with the same key', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openMember(view.target, queued.calls);
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'lost');
    const lost = take(queued.calls, (call) => call.path === '/reservations');
    lost.reject(new ApiError('The connection closed.', 'network', 0));
    await settle();
    const uncertain = view.target.querySelector('[data-testid="booking-uncertain"]');
    expect(uncertain?.textContent?.trim().length).toBeGreaterThan(0);
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '5');
    await tick();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '2');
    await tick();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeTruthy();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'retry');
    const retry = take(queued.calls, (call) => call.path === '/reservations');
    expect(retry.init?.idempotencyKey).toBe(lost.init?.idempotencyKey);
    expect(posted(retry)).toEqual(posted(lost));
    retry.resolve(reservation('RIVER7'));
    await settle();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('RIVER7');
    await view.cleanup();
  });

  it('treats a server failure and an unreadable success as uncertain, and a refusal as an error', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openMember(view.target, queued.calls);
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'server');
    take(queued.calls, (call) => call.path === '/reservations').reject(new ApiError('The book failed.', 'internal', 500));
    await settle();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'unreadable');
    take(queued.calls, (call) => call.path === '/reservations').resolve({ status: 'confirmed' });
    await settle();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '4');
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'refused');
    const refused = take(queued.calls, (call) => call.path === '/reservations');
    expect(posted(refused).party_size).toBe(4);
    expect(refused.init?.idempotencyKey).not.toBeUndefined();
    refused.reject(new ApiError('The party is too large for this table.', 'party_exceeds_capacity', 422));
    await settle();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-error"]')?.textContent).toContain('too large');
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('4');
    expect(queued.calls.some((call) => call.path.startsWith('/availability?'))).toBe(false);
    await view.cleanup();
  });

  it('keeps the held form when a taken table is refreshed and ignores a late refresh', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openMember(view.target, queued.calls);
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '4');
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'taken');
    take(queued.calls, (call) => call.path === '/reservations').reject(
      new ApiError('That table was just taken.', 'table_unavailable', 409),
    );
    await until(() => queued.calls.some((call) => call.path.startsWith('/availability?')), 'refresh started');
    const late = take(queued.calls, (call) => call.path.startsWith('/availability?'));
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect((view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('4');
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    setControl(view.target.querySelector('[data-testid="restaurant-select"]') as HTMLSelectElement, 'r_nord');
    (view.target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/restaurants/r_nord'), 'nord');
    take(queued.calls, (call) => call.path === '/restaurants/r_nord').resolve(nord);
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
    late.resolve({
      restaurant_id: 'r_anker',
      date: '2027-06-17',
      timezone: 'Europe/Berlin',
      slots: [
        {
          starts_at_local: '2027-06-17T18:00',
          starts_at: '2027-06-17T18:00:00+02:00',
          available_table_ids: ['t_late'],
        },
      ],
    });
    await settle();
    expect(view.target.querySelector('h1')?.textContent).toContain('Nordlicht');
    expect(view.target.querySelector('[data-testid="slot-t_window-17:00"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="slot-t_late-18:00"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    await view.cleanup();
  });

  it('drops an in-flight booking when another table is chosen', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openMember(view.target, queued.calls);
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'first table');
    const first = take(queued.calls, (call) => call.path === '/reservations');
    (view.target.querySelector('[data-testid="slot-t_2-18:30"]') as HTMLButtonElement).click();
    await tick();
    first.resolve(reservation('OLDREF'));
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 2');
    expect(view.target.textContent).not.toContain('Sending your request.');
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'second table');
    const second = take(queued.calls, (call) => call.path === '/reservations');
    expect(posted(second).table_id).toBe('t_2');
    expect(posted(second).starts_at_local).toBe('2027-06-17T18:30');
    expect(second.init?.idempotencyKey).not.toBe(first.init?.idempotencyKey);
    await view.cleanup();
  });
});

describe('live lookup', () => {
  it('shows the owner record, cancels it, and keeps the record when cancellation is refused', async () => {
    const queued = recordingTransport();
    const view = render(LookupScreen, { transport: queued.transport, token: 'opaque-session' });
    setControl(view.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, 'RIVER7');
    (view.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations/RIVER7'), 'lookup');
    const lookup = take(queued.calls, (call) => call.path === '/reservations/RIVER7');
    expect(lookup.init?.token).toBe('opaque-session');
    expect(lookup.init?.method).toBeUndefined();
    lookup.resolve(reservation('RIVER7'));
    await until(() => queued.calls.some((call) => call.path === '/restaurants/r_anker'), 'detail');
    take(queued.calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
    await settle();
    expect(view.target.querySelector('[data-testid="reservation-status"]')?.textContent).toBe('confirmed');
    expect(view.target.querySelector('[data-testid="reservation-tables"]')?.textContent).toContain('1');
    expect(view.target.querySelector('[data-testid="reservation-detail"]')?.textContent).toContain('Zum Anker');
    expect(view.target.querySelector('[data-testid="reservation-cancel-button"]')).toBeTruthy();
    expect(view.target.querySelector('[data-testid="reservation-error"]')).toBeNull();
    (view.target.querySelector('[data-testid="reservation-cancel-button"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations/RIVER7/cancel'), 'cancel');
    const cancel = take(queued.calls, (call) => call.path === '/reservations/RIVER7/cancel');
    expect(cancel.init?.method).toBe('POST');
    expect(cancel.init?.token).toBe('opaque-session');
    cancel.reject(new ApiError('The cutoff has passed.', 'cutoff_passed', 409));
    await settle();
    expect(view.target.querySelector('[data-testid="reservation-error"]')?.textContent).toContain('cutoff');
    expect(view.target.querySelector('[data-testid="reservation-status"]')?.textContent).toBe('confirmed');
    expect(view.target.querySelector('[data-testid="reservation-cancel-button"]')).toBeTruthy();
    (view.target.querySelector('[data-testid="reservation-cancel-button"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations/RIVER7/cancel'), 'cancel again');
    take(queued.calls, (call) => call.path === '/reservations/RIVER7/cancel').resolve({
      ...reservation('RIVER7'),
      status: 'cancelled',
    });
    await settle();
    expect(view.target.querySelector('[data-testid="reservation-status"]')?.textContent).toBe('cancelled');
    expect(view.target.querySelector('[data-testid="reservation-cancel-button"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="reservation-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="reservation-tables"]')?.textContent).toContain('1');
    await view.cleanup();
  });

  it('hides unknown reservations and does not call the service without a reference or a session', async () => {
    const queued = recordingTransport();
    const view = render(LookupScreen, { transport: queued.transport, token: 'opaque-session' });
    (view.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await settle();
    expect(view.target.querySelector('[data-testid="reservation-error"]')?.textContent).toContain('Enter a reservation reference.');
    expect(view.target.querySelector('[data-testid="reservation-detail"]')).toBeNull();
    expect(queued.calls).toHaveLength(0);
    setControl(view.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, 'OTHER1');
    (view.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.length === 1, 'missing');
    take(queued.calls, (call) => call.path === '/reservations/OTHER1').reject(
      new ApiError('No reservation matches that reference.', 'not_found', 404),
    );
    await settle();
    expect(view.target.querySelector('[data-testid="reservation-detail"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="reservation-error"]')?.textContent).toContain('No reservation');
    await view.cleanup();

    const guest = recordingTransport();
    const signedOut = render(LookupScreen, { transport: guest.transport, token: null });
    setControl(signedOut.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, 'RIVER7');
    (signedOut.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await settle();
    expect(signedOut.target.querySelector('[data-testid="reservation-error"]')?.textContent).toContain('Sign in');
    expect(signedOut.target.querySelector('[data-testid="reservation-detail"]')).toBeNull();
    expect(guest.calls).toHaveLength(0);
    await signedOut.cleanup();
  });

  it('keeps the later lookup when an earlier one finishes afterwards', async () => {
    const queued = recordingTransport();
    const view = render(LookupScreen, { transport: queued.transport, token: 'opaque-session' });
    setControl(view.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, 'FIRST1');
    (view.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.length === 1, 'first lookup');
    const first = take(queued.calls, (call) => call.path === '/reservations/FIRST1');
    setControl(view.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, 'SECOND');
    (view.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.length === 1, 'second lookup');
    const second = take(queued.calls, (call) => call.path === '/reservations/SECOND');
    second.resolve(reservation('SECOND', 2, 't_2'));
    await until(() => queued.calls.some((call) => call.path === '/restaurants/r_anker'), 'second detail');
    take(queued.calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
    await settle();
    first.resolve(reservation('FIRST1'));
    await settle();
    expect(view.target.querySelector('[data-testid="reservation-status"]')?.textContent).toBe('confirmed');
    expect(view.target.querySelector('[data-testid="reservation-tables"]')?.textContent).toContain('2');
    expect(view.target.querySelector('[data-testid="reservation-detail"]')?.textContent).not.toContain('Table 1');
    await view.cleanup();
  });
});

describe('live accessibility', () => {
  async function gate(target: ParentNode): Promise<void> {
    theme.set('light');
    await a11yGate(target as HTMLElement);
    theme.set('dark');
    await a11yGate(target as HTMLElement);
  }

  it('passes confirmed, uncertain, refused and lookup presentations', async () => {
    const queued = recordingTransport();
    const view = render(LiveSearch, { transport: queued.transport, signedIn: true, token: 'opaque-session' });
    await openMember(view.target, queued.calls);
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'uncertain gate');
    take(queued.calls, (call) => call.path === '/reservations').reject(new ApiError('closed', 'network', 0));
    await settle();
    await gate(view.target);
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'confirmed gate');
    take(queued.calls, (call) => call.path === '/reservations').resolve(reservation('RIVER7'));
    await settle();
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('RIVER7');
    await gate(view.target);
    setControl(view.target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement, '4');
    await tick();
    (view.target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    await until(() => queued.calls.some((call) => call.path === '/reservations'), 'refused gate');
    take(queued.calls, (call) => call.path === '/reservations').reject(
      new ApiError('That table was just taken.', 'table_unavailable', 409),
    );
    await settle();
    await gate(view.target);
    await view.cleanup();

    const lookup = recordingTransport();
    const found = render(LookupScreen, { transport: lookup.transport, token: 'opaque-session' });
    setControl(found.target.querySelector('[data-testid="lookup-reference-input"]') as HTMLInputElement, 'RIVER7');
    (found.target.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    await until(() => lookup.calls.length === 1, 'lookup gate');
    take(lookup.calls, (call) => call.path === '/reservations/RIVER7').resolve(reservation('RIVER7'));
    await until(() => lookup.calls.some((call) => call.path === '/restaurants/r_anker'), 'lookup detail');
    take(lookup.calls, (call) => call.path === '/restaurants/r_anker').resolve(anker);
    await settle();
    await gate(found.target);
    await found.cleanup();
  });
});
