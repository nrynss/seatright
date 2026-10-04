import { ApiError } from '@nrynss/chaaya/keel';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { loginAccount, registerAccount } from '../lib/auth';
import { currentHold, currentSearch, rememberHold, rememberSearch } from '../lib/hold';
import { clearSession, loadSession, saveSession, type Session } from '../lib/session';
import {
  availabilityPath,
  detailPath,
  loadSearch,
  parseAvailability,
  parsePartySize,
  parseRestaurants,
  parseSearchDate,
  type SearchQuery,
} from '../lib/search';
import { liveTransport, type Transport } from '../lib/transport';

const ankerQuery: SearchQuery = { restaurantId: 'r_anker', date: '2027-06-17', partySize: 2 };
const nordQuery: SearchQuery = { restaurantId: 'r_nord', date: '2027-06-17', partySize: 2 };

function detail(id: string, name: string, timezone: string) {
  return {
    id,
    name,
    timezone,
    tables: [
      { id: id === 'r_nord' ? 't_window' : 't_1', label: id === 'r_nord' ? 'Window' : '1', capacity: 2 },
    ],
  };
}

function availability(id: string, timezone: string, local: string) {
  return {
    restaurant_id: id,
    date: '2027-06-17',
    timezone,
    slots: [
      {
        starts_at_local: local,
        starts_at: `${local}:00+02:00`,
        available_table_ids: [id === 'r_nord' ? 't_window' : 't_1'],
      },
    ],
  };
}

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (error: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function memoryStorage(): Storage {
  const data = new Map<string, string>();
  return {
    get length() {
      return data.size;
    },
    clear() {
      data.clear();
    },
    getItem(key: string) {
      return data.get(key) ?? null;
    },
    key() {
      return null;
    },
    setItem(key: string, value: string) {
      data.set(key, value);
    },
    removeItem(key: string) {
      data.delete(key);
    },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  rememberSearch(null);
  rememberHold(null);
});

describe('search parsing', () => {
  it('keeps fixture order and rejects non-canonical party text', () => {
    const restaurants = parseRestaurants({
      restaurants: [
        { id: 'r_anker', name: 'Zum Anker', timezone: 'Europe/Berlin' },
        { id: 'r_nord', name: 'Nordlicht', timezone: 'America/New_York' },
      ],
    });
    expect(restaurants.map((item) => item.id)).toEqual(['r_anker', 'r_nord']);
    const parsed = parseAvailability(availability('r_anker', 'Europe/Berlin', '2027-06-17T18:00'));
    expect(parsed.slots[0].availableTableIds).toEqual(['t_1']);
    expect(parsed.slots[0].starts_at_local).toBe('2027-06-17T18:00');
    expect(parsePartySize('4')).toBe(4);
    expect(parseSearchDate('2027-06-17')).toBe('2027-06-17');
    for (const bad of ['4.0', '+4', '1e2', '0', '04', '']) {
      expect(() => parsePartySize(bad)).toThrow(ApiError);
    }
    expect(availabilityPath(ankerQuery)).toBe('/availability?restaurant_id=r_anker&date=2027-06-17&party_size=2');
    expect(detailPath('r nord')).toBe('/restaurants/r%20nord');
  });
});

describe('latest search', () => {
  it('ignores a slower earlier success after a later search is ready', async () => {
    const aDetail = deferred<unknown>();
    const aAvailability = deferred<unknown>();
    const bDetail = deferred<unknown>();
    const bAvailability = deferred<unknown>();
    const queue = [aDetail, aAvailability, bDetail, bAvailability];
    let cursor = 0;
    const transport: Transport = () => queue[cursor++].promise;
    let generation = 0;
    const start = (query: SearchQuery) => {
      generation += 1;
      const mine = generation;
      return loadSearch(transport, query, () => mine === generation);
    };
    const first = start(ankerQuery);
    const second = start(nordQuery);
    bDetail.resolve(detail('r_nord', 'Nordlicht', 'America/New_York'));
    bAvailability.resolve(availability('r_nord', 'America/New_York', '2027-06-17T17:00'));
    const ready = await second;
    expect(ready.status).toBe('ready');
    if (ready.status === 'ready') expect(ready.search.detail.name).toBe('Nordlicht');
    aDetail.resolve(detail('r_anker', 'Zum Anker', 'Europe/Berlin'));
    aAvailability.resolve(availability('r_anker', 'Europe/Berlin', '2027-06-17T18:00'));
    await expect(first).resolves.toEqual({ status: 'stale' });
  });

  it('ignores a slower earlier failure after a later search is ready', async () => {
    const aDetail = deferred<unknown>();
    const aAvailability = deferred<unknown>();
    const bDetail = deferred<unknown>();
    const bAvailability = deferred<unknown>();
    const queue = [aDetail, aAvailability, bDetail, bAvailability];
    let cursor = 0;
    const transport: Transport = () => queue[cursor++].promise;
    let generation = 0;
    const start = (query: SearchQuery) => {
      generation += 1;
      const mine = generation;
      return loadSearch(transport, query, () => mine === generation);
    };
    const first = start(ankerQuery);
    const second = start(nordQuery);
    bDetail.resolve(detail('r_nord', 'Nordlicht', 'America/New_York'));
    bAvailability.resolve(availability('r_nord', 'America/New_York', '2027-06-17T17:00'));
    await expect(second).resolves.toMatchObject({ status: 'ready' });
    aDetail.reject(new ApiError('gone', 'not_found', 404));
    aAvailability.resolve(availability('r_anker', 'Europe/Berlin', '2027-06-17T18:00'));
    await expect(first).resolves.toEqual({ status: 'stale' });
  });
});

describe('account client', () => {
  it('stores only a session the service returned', async () => {
    const calls: unknown[] = [];
    const transport: Transport = (_path, init) => {
      calls.push(init?.body);
      return Promise.resolve({ user_id: 'u_1', display_name: 'Ada', token: 'opaque-session' });
    };
    const session = await registerAccount(transport, {
      email: 'ada@example.com',
      password: 'correct horse',
      displayName: 'Ada',
    });
    expect(calls[0]).toEqual({
      email: 'ada@example.com',
      password: 'correct horse',
      display_name: 'Ada',
    });
    expect(session).toEqual({ userId: 'u_1', displayName: 'Ada', token: 'opaque-session' });
    const broken: Transport = () => Promise.resolve({ user_id: 'u_1' });
    await expect(loginAccount(broken, 'ada@example.com', 'correct horse')).rejects.toMatchObject({
      code: 'validation_failed',
    });
    expect(currentHold()).toBeNull();
    expect(currentSearch()).toBeNull();
  });

  it('keeps the refusal code from the Keel adapter', async () => {
    vi.stubGlobal(
      'fetch',
      async () =>
        new Response(JSON.stringify({ error: { code: 'email_taken', message: 'Email already registered.' } }), {
          status: 409,
          headers: { 'Content-Type': 'application/json' },
        }),
    );
    await expect(
      registerAccount(liveTransport, {
        email: 'ada@example.com',
        password: 'correct horse',
        displayName: 'Ada',
      }),
    ).rejects.toMatchObject({ code: 'email_taken', status: 409 });
  });

  it('sends same-origin JSON and a bearer header only with a token', async () => {
    const calls: { path: string; init: RequestInit }[] = [];
    vi.stubGlobal('fetch', async (path: string, init: RequestInit = {}) => {
      calls.push({ path: String(path), init });
      return new Response('{}', { status: 200 });
    });
    await liveTransport('/restaurants');
    await liveTransport('/reservations', { token: 'opaque-session' });
    expect(calls[0].path).toBe('/restaurants');
    expect(new Headers(calls[0].init.headers).get('Authorization')).toBeNull();
    expect(calls[0].init.method).toBe('GET');
    const signed = new Headers(calls[1].init.headers);
    expect(signed.get('Authorization')).toBe('Bearer opaque-session');
    expect(signed.get('Accept')).toBe('application/json');
    expect(calls[1].init.body).toBeUndefined();
  });
});

describe('session storage', () => {
  it('round-trips a token and clears it on logout', () => {
    const storage = memoryStorage();
    expect(loadSession(storage)).toBeNull();
    const session: Session = { token: 'opaque-session', displayName: 'Ada', userId: 'u_1' };
    saveSession(session, storage);
    expect(loadSession(storage)).toEqual(session);
    clearSession(storage);
    expect(loadSession(storage)).toBeNull();
    storage.setItem('tablekeeper-session', '{');
    expect(loadSession(storage)).toBeNull();
  });
});
