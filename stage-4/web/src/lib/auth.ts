import { ApiError } from './client';
import type { Session } from './session';
import type { Transport } from './transport';

export interface SignupInput {
  email: string;
  password: string;
  displayName: string;
}

function readSession(body: unknown): Session {
  if (!body || typeof body !== 'object') {
    throw new ApiError('The account response was incomplete.', 'validation_failed', 200);
  }
  const value = body as { user_id?: unknown; display_name?: unknown; token?: unknown };
  if (typeof value.user_id !== 'string' || value.user_id === '') {
    throw new ApiError('The account response was incomplete.', 'validation_failed', 200);
  }
  if (typeof value.display_name !== 'string' || typeof value.token !== 'string' || value.token === '') {
    throw new ApiError('The account response was incomplete.', 'validation_failed', 200);
  }
  return { userId: value.user_id, displayName: value.display_name, token: value.token };
}

export async function registerAccount(transport: Transport, input: SignupInput): Promise<Session> {
  const body = await transport('/auth/signup', {
    method: 'POST',
    body: {
      email: input.email,
      password: input.password,
      display_name: input.displayName,
    },
  });
  return readSession(body);
}

export async function loginAccount(transport: Transport, email: string, password: string): Promise<Session> {
  const body = await transport('/auth/login', {
    method: 'POST',
    body: { email, password },
  });
  return readSession(body);
}

export function authMessage(error: unknown): string {
  if (error instanceof ApiError && error.message) return error.message;
  if (error instanceof Error && error.message) return error.message;
  return 'The request could not be completed.';
}

/** Shown for a finished login credential check. The API code stays `unauthenticated`. */
export const CREDENTIAL_REFUSAL = 'Email or password is incorrect.';

/**
 * Login presentation only. A credential refusal uses one sentence for a wrong
 * password and an unknown email. Other codes, including a lost connection,
 * keep their own wording. The error object is not changed.
 */
export function loginFailureMessage(error: unknown): string {
  if (error instanceof ApiError && error.code === 'unauthenticated') return CREDENTIAL_REFUSAL;
  return authMessage(error);
}
