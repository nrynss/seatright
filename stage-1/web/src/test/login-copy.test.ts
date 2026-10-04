import { ApiError } from '@nrynss/chaaya/keel';
import { tick } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CREDENTIAL_REFUSAL, authMessage, loginAccount, loginFailureMessage } from '../lib/auth';
import LoginScreen from '../lib/components/LoginScreen.svelte';
import { liveTransport } from '../lib/transport';
import { render } from './render';

const SERVER_401 = 'missing or invalid bearer token';

function setControl(element: HTMLInputElement, value: string): void {
  const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
  descriptor?.set?.call(element, value);
  element.dispatchEvent(new Event('input', { bubbles: true }));
  element.dispatchEvent(new Event('change', { bubbles: true }));
}

async function settle(): Promise<void> {
  for (let attempt = 0; attempt < 8; attempt += 1) {
    await Promise.resolve();
    await tick();
  }
}

function envelope(status: number, code: string, message: string): Response {
  return new Response(JSON.stringify({ error: { code, message } }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

async function submit(target: ParentNode, email: string, password: string): Promise<void> {
  setControl(target.querySelector('[data-testid="login-email"]') as HTMLInputElement, email);
  setControl(target.querySelector('[data-testid="login-password"]') as HTMLInputElement, password);
  (target.querySelector('[data-testid="login-submit"]') as HTMLButtonElement).click();
  await settle();
}

describe('login credential copy', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('leaves the Keel error intact while the screen says the password was not accepted', () => {
    const failure = new ApiError(SERVER_401, 'unauthenticated', 401);
    expect(loginFailureMessage(failure)).toBe('Email or password is incorrect.');
    expect(CREDENTIAL_REFUSAL).toBe('Email or password is incorrect.');
    expect(failure.code).toBe('unauthenticated');
    expect(failure.status).toBe(401);
    expect(failure.message).toBe(SERVER_401);
    expect(authMessage(failure)).toBe(SERVER_401);
  });

  it('submits a wrong password and an unknown email through the Keel adapter', async () => {
    const seen: unknown[] = [];
    vi.stubGlobal('fetch', async (_path: string, init: RequestInit = {}) => {
      const body = JSON.parse(String(init.body)) as { email?: string };
      if (body.email === 'guest@example.com') {
        return envelope(422, 'validation_failed', 'A required field is missing.');
      }
      if (body.email === 'ada@example.com' || body.email === 'nobody@example.com') {
        return envelope(401, 'unauthenticated', SERVER_401);
      }
      return envelope(500, 'http_error', 'unexpected email');
    });
    const view = render(LoginScreen, {
      onAccount: async (form: FormData) => {
        try {
          await loginAccount(liveTransport, String(form.get('email') ?? ''), String(form.get('password') ?? ''));
        } catch (error) {
          seen.push(error);
          throw error;
        }
      },
    });

    await submit(view.target, 'ada@example.com', 'wrong password');
    const wrong = view.target.querySelector('[data-testid="auth-error"]')?.textContent ?? '';
    expect(wrong).toBe('Email or password is incorrect.');
    expect(wrong.toLowerCase()).not.toContain('bearer');
    expect(wrong.toLowerCase()).not.toContain('token');
    expect(seen[0]).toBeInstanceOf(ApiError);
    expect(seen[0]).toMatchObject({ code: 'unauthenticated', status: 401, message: SERVER_401 });

    await submit(view.target, 'nobody@example.com', 'correct horse');
    const unknown = view.target.querySelector('[data-testid="auth-error"]')?.textContent ?? '';
    expect(unknown).toBe(wrong);
    expect(seen[1]).toBeInstanceOf(ApiError);
    expect(seen[1]).toMatchObject({ code: 'unauthenticated', status: 401, message: SERVER_401 });
    expect((seen[0] as ApiError).message).toBe(SERVER_401);
    expect((seen[1] as ApiError).status).toBe(401);

    await submit(view.target, 'guest@example.com', 'correct horse');
    const other = view.target.querySelector('[data-testid="auth-error"]')?.textContent ?? '';
    expect(seen).toHaveLength(3);
    expect(other).toBe('A required field is missing.');
    expect(other).not.toBe('Email or password is incorrect.');
    expect(seen[2]).toMatchObject({ code: 'validation_failed', status: 422, message: 'A required field is missing.' });

    await view.cleanup();
  });

  it('does not call a lost connection a wrong password', async () => {
    const seen: unknown[] = [];
    vi.stubGlobal('fetch', async () => {
      throw new TypeError('Failed to fetch');
    });
    const view = render(LoginScreen, {
      onAccount: async (form: FormData) => {
        try {
          await loginAccount(liveTransport, String(form.get('email') ?? ''), String(form.get('password') ?? ''));
        } catch (error) {
          seen.push(error);
          throw error;
        }
      },
    });
    await submit(view.target, 'ada@example.com', 'correct horse');
    const text = view.target.querySelector('[data-testid="auth-error"]')?.textContent ?? '';
    expect(text).toBe('The request could not reach the server.');
    expect(text).not.toBe('Email or password is incorrect.');
    expect(text.toLowerCase()).not.toContain('password');
    expect(text.toLowerCase()).not.toContain('bearer');
    expect(seen[0]).toMatchObject({ code: 'network', status: 0 });
    await view.cleanup();
  });

  it('signs in when the password is accepted and clears any earlier refusal', async () => {
    let mode: 'refuse' | 'accept' = 'refuse';
    vi.stubGlobal('fetch', async () => {
      if (mode === 'refuse') return envelope(401, 'unauthenticated', SERVER_401);
      return new Response(JSON.stringify({ user_id: 'u_ada', display_name: 'Ada', token: 'opaque-session' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    let displayName = '';
    const view = render(LoginScreen, {
      onAccount: async (form: FormData) => {
        const session = await loginAccount(
          liveTransport,
          String(form.get('email') ?? ''),
          String(form.get('password') ?? ''),
        );
        displayName = session.displayName;
      },
    });
    await submit(view.target, 'ada@example.com', 'wrong password');
    expect(view.target.querySelector('[data-testid="auth-error"]')?.textContent).toBe('Email or password is incorrect.');
    mode = 'accept';
    await submit(view.target, 'ada@example.com', 'correct horse');
    expect(displayName).toBe('Ada');
    expect(view.target.querySelector('[data-testid="auth-error"]')).toBeNull();
    expect(view.target.querySelector('[data-testid="current-user"]')).toBeNull();
    await view.cleanup();
  });
});
