import { a11yGate } from '@nrynss/chaaya/testing';
import { theme } from '@nrynss/chaaya/theme';
import { afterEach, describe, expect, it } from 'vitest';
import Harness from './Harness.svelte';
import { render } from './render';

const cleanups: Array<() => Promise<void>> = [];

afterEach(async () => {
  theme.set('system');
  while (cleanups.length > 0) {
    const cleanup = cleanups.pop();
    if (cleanup) await cleanup();
  }
  document.body.innerHTML = '';
});

async function expectAccessible(props: Record<string, unknown>): Promise<void> {
  const view = render(Harness, props);
  cleanups.push(view.cleanup);
  try {
    theme.set('light');
    await a11yGate(view.target);
    theme.set('dark');
    await a11yGate(view.target);
    expect(view.target.querySelector('[title]')).toBeNull();
  } finally {
    await view.cleanup();
    cleanups.pop();
  }
}

describe('accessibility gate', () => {
  it('passes the search presentations in light and dark', async () => {
    for (const presentation of ['results', 'selected', 'loading', 'empty', 'refused', 'uncertain', 'confirmed']) {
      await expectAccessible({ presentation });
    }
  });

  it('passes signup, login and lookup in light and dark', async () => {
    await expectAccessible({ screen: 'signup' });
    await expectAccessible({ screen: 'signup', authError: 'An account with that email already exists.' });
    await expectAccessible({ screen: 'login' });
    await expectAccessible({ screen: 'login', authError: 'Those sign-in details were not accepted.' });
    await expectAccessible({ screen: 'lookup' });
    await expectAccessible({ screen: 'lookup', lookupError: 'No reservation matches that reference.' });
    await expectAccessible({ screen: 'search', user: 'River' });
  });
});
