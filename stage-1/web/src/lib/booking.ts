import { ApiError } from './client';
import type { Transport } from './transport';

/** Fields submitted to create one table booking. Copied at send time and then left unchanged. */
export interface BookingBody {
  restaurant_id: string;
  table_id: string;
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
  table_id: string;
  party_size: number;
  status: 'confirmed' | 'cancelled';
  starts_at_local: string;
  starts_at: string;
  ends_at: string;
  created_at: string;
}

export type BookingOutcome =
  | { kind: 'confirmed'; reservation: ReservationRecord }
  | { kind: 'rejected'; code: string; message: string }
  | { kind: 'uncertain'; message: string };

export const UNCERTAIN_MESSAGE =
  'We could not confirm that request. It may have reached the restaurant. Submit this same table, time and party size again to recover the original confirmation.';

export function sameBooking(left: BookingBody, right: BookingBody): boolean {
  return (
    left.restaurant_id === right.restaurant_id &&
    left.table_id === right.table_id &&
    left.starts_at_local === right.starts_at_local &&
    left.party_size === right.party_size
  );
}

function snapshot(body: BookingBody): BookingBody {
  return {
    restaurant_id: body.restaurant_id,
    table_id: body.table_id,
    starts_at_local: body.starts_at_local,
    party_size: body.party_size,
  };
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
  return {
    reservation_id: requiredText(root, 'reservation_id', 'The reservation response was not usable.'),
    reference: requiredText(root, 'reference', 'The reservation response was not usable.'),
    restaurant_id: requiredText(root, 'restaurant_id', 'The reservation response was not usable.'),
    table_id: requiredText(root, 'table_id', 'The reservation response was not usable.'),
    party_size: party,
    status,
    starts_at_local: requiredText(root, 'starts_at_local', 'The reservation response was not usable.'),
    starts_at: requiredText(root, 'starts_at', 'The reservation response was not usable.'),
    ends_at: requiredText(root, 'ends_at', 'The reservation response was not usable.'),
    created_at: requiredText(root, 'created_at', 'The reservation response was not usable.'),
  };
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
