import { ApiError } from './client';
import { copyIds, sameIds } from './seating';
import type { Transport } from './transport';

/**
 * Fields submitted to create a booking. Copied at send time and then left unchanged.
 * A single table keeps the legacy `table_id` field so an older pending request still matches.
 * A pair sends `table_ids` only.
 */
export interface BookingBody {
  restaurant_id: string;
  table_id?: string;
  table_ids?: string[];
  starts_at_local: string;
  party_size: number;
}

/** One in-flight or retryable create. The key stays with this body for the life of the attempt. */
export interface BookingAttempt {
  key: string;
  body: BookingBody;
}

export interface ReservationRecord {
  reservation_id: string;
  reference: string;
  restaurant_id: string;
  /** Set for a single table. Empty when the reservation holds a pair. */
  table_id: string;
  /** Copy of the reserved set. One id for a legacy receipt that only named `table_id`. */
  tableIds: string[];
  party_size: number;
  status: 'confirmed' | 'cancelled';
  starts_at_local: string;
  starts_at: string;
  ends_at: string;
  created_at: string;
  /** Present only when the response itself named a positive integer revision. */
  revision?: number;
  /** Opaque copy of the response's accepted_terms. Absent when that field is absent. */
  acceptedTerms?: Record<string, unknown>;
}

export type BookingOutcome =
  | { kind: 'confirmed'; reservation: ReservationRecord }
  | { kind: 'rejected'; code: string; message: string }
  | { kind: 'uncertain'; message: string };

export const UNCERTAIN_MESSAGE =
  'We could not confirm that request. It may have reached the restaurant. Submit this same table, time and party size again to recover the original confirmation.';

function pairIds(body: BookingBody): string[] {
  if (!body.table_ids || body.table_ids.length < 2) return [];
  return copyIds(body.table_ids);
}

export function sameBooking(left: BookingBody, right: BookingBody): boolean {
  return (
    left.restaurant_id === right.restaurant_id &&
    (left.table_id ?? '') === (right.table_id ?? '') &&
    sameIds(pairIds(left), pairIds(right)) &&
    left.starts_at_local === right.starts_at_local &&
    left.party_size === right.party_size
  );
}

function snapshot(body: BookingBody): BookingBody {
  const next: BookingBody = {
    restaurant_id: body.restaurant_id,
    starts_at_local: body.starts_at_local,
    party_size: body.party_size,
  };
  const ids = pairIds(body);
  if (ids.length > 1) next.table_ids = ids;
  else if (body.table_id) next.table_id = body.table_id;
  return next;
}

/** The request shape of a receipt, without rewriting the server's reference or times. */
export function bodyFromRecord(record: ReservationRecord): BookingBody {
  const body: BookingBody = {
    restaurant_id: record.restaurant_id,
    starts_at_local: record.starts_at_local,
    party_size: record.party_size,
  };
  if (record.tableIds.length > 1) body.table_ids = copyIds(record.tableIds);
  else body.table_id = record.table_id || record.tableIds[0];
  return body;
}

/**
 * Reuse the previous key and the body that was already sent when every submitted field matches.
 * A changed field starts a new attempt. The returned body is a copy, so later edits cannot rewrite a sent request.
 */
export function attemptFor(previous: BookingAttempt | null, body: BookingBody, key: string): BookingAttempt {
  if (previous && sameBooking(previous.body, body)) {
    return { key: previous.key, body: snapshot(previous.body) };
  }
  return { key, body: snapshot(body) };
}

export function newIdempotencyKey(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

function record(value: unknown, message: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new ApiError(message, 'validation_failed', 200);
  }
  return value as Record<string, unknown>;
}

function requiredText(source: Record<string, unknown>, key: string, message: string): string {
  const value = source[key];
  if (typeof value !== 'string' || value === '') throw new ApiError(message, 'validation_failed', 200);
  return value;
}

/** A private copy of a current response object. Missing or non-object values stay omitted. */
function opaqueObject(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const clone = JSON.parse(JSON.stringify(value)) as unknown;
  if (!clone || typeof clone !== 'object' || Array.isArray(clone)) return undefined;
  return clone as Record<string, unknown>;
}

export function parseReservation(body: unknown): ReservationRecord {
  const root = record(body, 'The reservation response was not usable.');
  const status = requiredText(root, 'status', 'The reservation response was not usable.');
  if (status !== 'confirmed' && status !== 'cancelled') {
    throw new ApiError('The reservation response was not usable.', 'validation_failed', 200);
  }
  const party = root.party_size;
  if (typeof party !== 'number' || !Number.isInteger(party)) {
    throw new ApiError('The reservation response was not usable.', 'validation_failed', 200);
  }
  const listed = root.table_ids;
  let tableIds: string[];
  if (Array.isArray(listed)) {
    if (listed.length === 0 || listed.length > 2) {
      throw new ApiError('The reservation response was not usable.', 'validation_failed', 200);
    }
    tableIds = listed.map((id) => {
      if (typeof id !== 'string' || id === '') {
        throw new ApiError('The reservation response was not usable.', 'validation_failed', 200);
      }
      return id;
    });
    if (tableIds.length === 2 && tableIds[0] === tableIds[1]) {
      throw new ApiError('The reservation response was not usable.', 'validation_failed', 200);
    }
  } else {
    tableIds = [requiredText(root, 'table_id', 'The reservation response was not usable.')];
  }
  const single = tableIds.length === 1 ? tableIds[0] : '';
  const parsed: ReservationRecord = {
    reservation_id: requiredText(root, 'reservation_id', 'The reservation response was not usable.'),
    reference: requiredText(root, 'reference', 'The reservation response was not usable.'),
    restaurant_id: requiredText(root, 'restaurant_id', 'The reservation response was not usable.'),
    table_id: single,
    tableIds: copyIds(tableIds),
    party_size: party,
    status,
    starts_at_local: requiredText(root, 'starts_at_local', 'The reservation response was not usable.'),
    starts_at: requiredText(root, 'starts_at', 'The reservation response was not usable.'),
    ends_at: requiredText(root, 'ends_at', 'The reservation response was not usable.'),
    created_at: requiredText(root, 'created_at', 'The reservation response was not usable.'),
  };
  const revision = root.revision;
  if (typeof revision === 'number' && Number.isInteger(revision) && revision >= 1) parsed.revision = revision;
  const acceptedTerms = opaqueObject(root.accepted_terms);
  if (acceptedTerms) parsed.acceptedTerms = acceptedTerms;
  return parsed;
}

/** A completed 4xx is a refusal. A lost response, a 5xx, or an unreadable body is not. */
export function bookingFailureKind(error: unknown): 'rejected' | 'uncertain' {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return 'rejected';
  return 'uncertain';
}

export function failureMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.message) return error.message;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

export async function submitBooking(
  transport: Transport,
  token: string,
  attempt: BookingAttempt,
): Promise<BookingOutcome> {
  try {
    const body = await transport('/reservations', {
      method: 'POST',
      token,
      idempotencyKey: attempt.key,
      body: snapshot(attempt.body),
    });
    return { kind: 'confirmed', reservation: parseReservation(body) };
  } catch (error) {
    if (bookingFailureKind(error) === 'rejected') {
      return {
        kind: 'rejected',
        code: error instanceof ApiError ? error.code : 'validation_failed',
        message: failureMessage(error, 'The restaurant refused that request.'),
      };
    }
    return { kind: 'uncertain', message: UNCERTAIN_MESSAGE };
  }
}

export function reservationPath(reference: string): string {
  return `/reservations/${encodeURIComponent(reference)}`;
}

export async function loadReservation(
  transport: Transport,
  token: string,
  reference: string,
): Promise<ReservationRecord> {
  return parseReservation(await transport(reservationPath(reference), { token }));
}

export async function cancelReservation(
  transport: Transport,
  token: string,
  reference: string,
): Promise<ReservationRecord> {
  return parseReservation(
    await transport(`${reservationPath(reference)}/cancel`, {
      method: 'POST',
      token,
      body: {},
    }),
  );
}
