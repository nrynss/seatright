import { ApiError } from './client';
import { copyIds, type SeatingOption } from './seating';
import type { Transport } from './transport';

/** A Thursday far enough ahead that a later cancellation window can still be open. */
export const DEFAULT_SEARCH_DATE = '2027-06-17';

export interface RestaurantSummary {
  id: string;
  name: string;
  timezone: string;
}

export interface TableRecord {
  id: string;
  label: string;
  capacity: number;
}

export interface RestaurantDetail extends RestaurantSummary {
  tables: TableRecord[];
  /** Declared pairs in fixture order. Missing on a stage-1 detail means none. */
  combinable: string[][];
}

export interface AvailabilitySlot {
  starts_at_local: string;
  starts_at: string;
  availableTableIds: readonly string[];
  options: SeatingOption[];
}

export interface AvailabilityResult {
  restaurant_id: string;
  date: string;
  timezone: string;
  slots: AvailabilitySlot[];
}

export interface SearchQuery {
  restaurantId: string;
  date: string;
  partySize: number;
}

/** Immutable submitted search. Later edits to the form do not change this object. */
export interface CommittedSearch {
  query: SearchQuery;
  detail: RestaurantDetail;
  availability: AvailabilityResult;
  detailBody: unknown;
  availabilityBody: unknown;
}

const DATE = /^\d{4}-\d{2}-\d{2}$/;
const PARTY = /^[1-9]\d*$/;
const LOCAL = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})$/;

function record(value: unknown, message: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new ApiError(message, 'validation_failed', 200);
  }
  return value as Record<string, unknown>;
}

function text(source: Record<string, unknown>, key: string, message: string): string {
  const value = source[key];
  if (typeof value !== 'string') throw new ApiError(message, 'validation_failed', 200);
  return value;
}

function requiredText(source: Record<string, unknown>, key: string, message: string): string {
  const value = text(source, key, message);
  if (value === '') throw new ApiError(message, 'validation_failed', 200);
  return value;
}

function stringList(value: unknown, message: string): string[] {
  if (!Array.isArray(value)) throw new ApiError(message, 'validation_failed', 200);
  return value.map((item) => {
    if (typeof item !== 'string' || item === '') throw new ApiError(message, 'validation_failed', 200);
    return item;
  });
}

/** Absent means no pairs. A present value must be pairs of table ids, in declared order. */
function parseCombinable(value: unknown): string[][] {
  if (value === undefined) return [];
  if (!Array.isArray(value)) throw new ApiError('The restaurant detail was not usable.', 'validation_failed', 200);
  return value.map((item) => {
    const pair = stringList(item, 'The restaurant detail was not usable.');
    if (pair.length !== 2 || pair[0] === pair[1]) {
      throw new ApiError('The restaurant detail was not usable.', 'validation_failed', 200);
    }
    return pair;
  });
}

/** Clock suffix HH:MM from a bare local start. */
export function slotClock(startsAtLocal: string): string {
  const match = LOCAL.exec(startsAtLocal);
  if (!match) {
    throw new ApiError('A slot time was not a local date and clock.', 'validation_failed', 200);
  }
  return match[2];
}

export function parseRestaurants(body: unknown): RestaurantSummary[] {
  const root = record(body, 'The restaurant list was not usable.');
  if (!Array.isArray(root.restaurants)) {
    throw new ApiError('The restaurant list was not usable.', 'validation_failed', 200);
  }
  return root.restaurants.map((item) => {
    const entry = record(item, 'A restaurant entry was not usable.');
    return {
      id: requiredText(entry, 'id', 'A restaurant entry was not usable.'),
      name: text(entry, 'name', 'A restaurant entry was not usable.'),
      timezone: requiredText(entry, 'timezone', 'A restaurant entry was not usable.'),
    };
  });
}

export function parseDetail(body: unknown): RestaurantDetail {
  const root = record(body, 'The restaurant detail was not usable.');
  if (!Array.isArray(root.tables)) {
    throw new ApiError('The restaurant detail was not usable.', 'validation_failed', 200);
  }
  const tables = root.tables.map((item) => {
    const entry = record(item, 'A table entry was not usable.');
    const capacity = entry.capacity;
    if (typeof capacity !== 'number' || !Number.isInteger(capacity) || capacity < 0) {
      throw new ApiError('A table capacity was not usable.', 'validation_failed', 200);
    }
    return {
      id: requiredText(entry, 'id', 'A table entry was not usable.'),
      label: text(entry, 'label', 'A table entry was not usable.'),
      capacity,
    };
  });
  return {
    id: requiredText(root, 'id', 'The restaurant detail was not usable.'),
    name: text(root, 'name', 'The restaurant detail was not usable.'),
    timezone: requiredText(root, 'timezone', 'The restaurant detail was not usable.'),
    tables,
    combinable: parseCombinable(root.combinable),
  };
}

export function parseAvailability(body: unknown): AvailabilityResult {
  const root = record(body, 'The availability response was not usable.');
  if (!Array.isArray(root.slots)) {
    throw new ApiError('The availability response was not usable.', 'validation_failed', 200);
  }
  const slots = root.slots.map((item) => {
    const entry = record(item, 'A slot was not usable.');
    const availableTableIds = stringList(entry.available_table_ids, 'A slot was not usable.');
    const startsAtLocal = requiredText(entry, 'starts_at_local', 'A slot was not usable.');
    slotClock(startsAtLocal);
    let options: SeatingOption[];
    if (Array.isArray(entry.available_options)) {
      options = entry.available_options.map((option) => {
        const choice = record(option, 'A slot was not usable.');
        const capacity = choice.capacity;
        if (typeof capacity !== 'number' || !Number.isInteger(capacity)) {
          throw new ApiError('A slot was not usable.', 'validation_failed', 200);
        }
        return { tableIds: stringList(choice.table_ids, 'A slot was not usable.'), capacity };
      });
    } else {
      options = availableTableIds.map((id) => ({ tableIds: [id], capacity: 0 }));
    }
    return {
      starts_at_local: startsAtLocal,
      starts_at: requiredText(entry, 'starts_at', 'A slot was not usable.'),
      availableTableIds: copyIds(availableTableIds),
      options,
    };
  });
  return {
    restaurant_id: requiredText(root, 'restaurant_id', 'The availability response was not usable.'),
    date: requiredText(root, 'date', 'The availability response was not usable.'),
    timezone: requiredText(root, 'timezone', 'The availability response was not usable.'),
    slots,
  };
}

export function parsePartySize(raw: string): number {
  if (!PARTY.test(raw)) {
    throw new ApiError('Party size must be a whole number.', 'validation_failed', 422);
  }
  return Number(raw);
}

export function parseSearchDate(raw: string): string {
  if (!DATE.test(raw)) {
    throw new ApiError('Choose a calendar date.', 'validation_failed', 422);
  }
  return raw;
}

export function partyText(value: number | null | undefined): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '';
  return String(value);
}

export function availabilityPath(query: SearchQuery): string {
  const params = new URLSearchParams({
    restaurant_id: query.restaurantId,
    date: query.date,
    party_size: String(query.partySize),
  });
  return `/availability?${params.toString()}`;
}

export function detailPath(id: string): string {
  return `/restaurants/${encodeURIComponent(id)}`;
}

export type SearchLoad =
  | { status: 'ready'; search: CommittedSearch }
  | { status: 'stale' }
  | { status: 'error'; error: unknown };

/**
 * Load restaurant detail and availability together.
 * `isCurrent` is checked after both settle, including when one of them fails,
 * so an older search cannot replace a newer one.
 */
export async function loadSearch(
  transport: Transport,
  query: SearchQuery,
  isCurrent: () => boolean,
): Promise<SearchLoad> {
  const detailRequest = transport(detailPath(query.restaurantId)).then(
    (value) => ({ ok: true as const, value }),
    (error: unknown) => ({ ok: false as const, error }),
  );
  const availabilityRequest = transport(availabilityPath(query)).then(
    (value) => ({ ok: true as const, value }),
    (error: unknown) => ({ ok: false as const, error }),
  );
  const [detailOutcome, availabilityOutcome] = await Promise.all([detailRequest, availabilityRequest]);
  if (!isCurrent()) return { status: 'stale' };
  if (!detailOutcome.ok) return { status: 'error', error: detailOutcome.error };
  if (!availabilityOutcome.ok) return { status: 'error', error: availabilityOutcome.error };
  try {
    const detail = parseDetail(detailOutcome.value);
    const availability = parseAvailability(availabilityOutcome.value);
    if (!isCurrent()) return { status: 'stale' };
    return {
      status: 'ready',
      search: {
        query,
        detail,
        availability,
        detailBody: detailOutcome.value,
        availabilityBody: availabilityOutcome.value,
      },
    };
  } catch (error) {
    if (!isCurrent()) return { status: 'stale' };
    return { status: 'error', error };
  }
}

export function errorText(error: unknown): string {
  if (error instanceof ApiError && error.message) return error.message;
  if (error instanceof Error && error.message) return error.message;
  return 'The search could not be completed.';
}
