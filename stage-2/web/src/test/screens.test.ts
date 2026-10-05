import { theme } from '@nrynss/chaaya/theme';
import { tick } from 'svelte';
import { afterEach, describe, expect, it } from 'vitest';
import Confirmation from '../lib/components/Confirmation.svelte';
import Harness from './Harness.svelte';
import { formatClock } from '../lib/format';
import { previewFixture, tableIsAvailable } from '../lib/preview';
import { render } from './render';

const cleanups: Array<() => Promise<void>> = [];

afterEach(async () => {
  theme.set('system');
  document.documentElement.removeAttribute('data-theme');
  while (cleanups.length > 0) {
    const cleanup = cleanups.pop();
    if (cleanup) await cleanup();
  }
  document.body.innerHTML = '';
});

function mount(props: Record<string, unknown> = {}) {
  const view = render(Harness, props);
  cleanups.push(view.cleanup);
  return view.target;
}

describe('search floor and grid', () => {
  it('mirrors the preview fixture and keeps state ahead of motion', async () => {
    const target = mount();
    expect(target.textContent).toContain('Zum Anker');
    expect(target.textContent).toContain('Thursday 24 September 2026');
    expect(target.textContent).toContain('Central European Summer Time');
    expect(target.textContent).toContain('Sample seating');
    expect((target.querySelector('[data-testid="restaurant-select"]') as HTMLSelectElement).value).toBe('r_anker');
    expect((target.querySelector('[data-testid="date-input"]') as HTMLInputElement).value).toBe('2026-09-24');
    expect((target.querySelector('[data-testid="party-size-input"]') as HTMLInputElement).value).toBe('2');
    for (const id of ['restaurant-select', 'date-input', 'party-size-input']) {
      expect(target.querySelector(`label[for="${id}"]`)).toBeTruthy();
    }

    for (const slot of previewFixture.slots) {
      for (const table of previewFixture.tables) {
        const cell = target.querySelector(`[data-testid="slot-${table.id}-${slot.time}"]`);
        expect(cell, `${table.id} ${slot.time}`).toBeTruthy();
        expect(cell?.getAttribute('data-available')).toBe(String(tableIsAvailable(slot, table.id)));
        const free = tableIsAvailable(slot, table.id);
        expect(cell?.textContent).toContain(`Table ${table.label}`);
        expect(cell?.textContent).toContain(slot.time);
        expect(cell?.textContent).toContain(free ? 'Free' : 'Taken');
        expect(cell?.getAttribute('aria-label')).toContain(free ? 'Available' : 'Unavailable');
        expect(cell?.getAttribute('aria-label')).toContain(formatClock(slot.time));
      }
    }
    const heads = [...target.querySelectorAll('.colhead')].map((node) => node.textContent ?? '');
    for (const slot of previewFixture.slots) {
      expect(heads.some((text) => text.includes(formatClock(slot.time)) && text.includes(slot.time))).toBe(true);
    }

    expect(target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-available')).toBe('true');
    expect(target.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-available')).toBe('true');
    expect(target.querySelector('[data-testid="plan-t_1"]')?.querySelectorAll('.seat')).toHaveLength(2);
    expect(target.querySelector('[data-testid="plan-t_2"]')?.querySelectorAll('.seat')).toHaveLength(4);
    const scene = target.querySelector('[data-testid="floor-plan"] svg.room-scene');
    expect(scene?.namespaceURI).toBe('http://www.w3.org/2000/svg');
    expect(target.querySelectorAll('svg.room-scene')).toHaveLength(1);
    expect(scene?.contains(target.querySelector('[data-testid="plan-t_1"]'))).toBe(true);
    expect(scene?.contains(target.querySelector('[data-testid="plan-t_2"]'))).toBe(true);
    expect(target.querySelector('[data-testid="plan-t_1"]')?.namespaceURI).toBe('http://www.w3.org/2000/svg');
    expect(scene?.querySelector('.room-wall')).toBeTruthy();
    expect(scene?.querySelector('.room-aisle')).toBeTruthy();
    expect(target.querySelector('.table-card')).toBeNull();
    expect(target.querySelector('[data-testid="booking-form"]')).toBeNull();
    expect(target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(target.querySelector('[data-testid="current-user"]')).toBeNull();

    const open = target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement;
    open.click();
    await tick();
    expect(target.querySelector('[data-testid="slot-t_1-18:00"]')?.getAttribute('data-selected')).toBe('true');
    const summary = target.querySelector('[data-testid="booking-summary"]')?.textContent ?? '';
    expect(summary).toContain('Table 1');
    expect(summary).toContain('18:00');
    expect(summary).toContain('6:00 PM');
    expect(summary).toContain('Zum Anker');
    expect((target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('2');
    expect(target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected')).toBe('true');

    (target.querySelector('[data-testid="booking-submit"]') as HTMLButtonElement).click();
    expect(target.querySelector('[data-testid="confirmation"]')).toBeNull();
    expect(target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();

    const taken = target.querySelector('[data-testid="slot-t_2-19:00"]') as HTMLButtonElement;
    taken.click();
    await tick();
    expect(target.querySelector('[data-testid="slot-t_2-19:00"]')?.getAttribute('data-selected')).toBe('false');
    expect(target.querySelector('[data-testid="slot-t_1-18:00"]')?.getAttribute('data-selected')).toBe('true');
    expect(target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('18:00');

    const later = target.querySelector('[data-testid="slot-t_1-19:00"]') as HTMLButtonElement;
    later.click();
    await tick();
    expect(target.querySelector('[data-testid="slot-t_1-19:00"]')?.getAttribute('data-selected')).toBe('true');
    expect(target.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-available')).toBe('false');
    expect(target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-available')).toBe('true');
    expect(target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected')).toBe('true');
    (target.querySelector('[data-testid="plan-t_2"]') as HTMLButtonElement).click();
    await tick();
    expect(target.querySelector('[data-testid="slot-t_1-19:00"]')?.getAttribute('data-selected')).toBe('true');
    expect(target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('19:00');

    const date = target.querySelector('[data-testid="date-input"]') as HTMLInputElement;
    date.value = '2026-10-01';
    date.dispatchEvent(new Event('input', { bubbles: true }));
    date.dispatchEvent(new Event('change', { bubbles: true }));
    expect(target.querySelector('[data-testid="slot-t_2-19:00"]')?.getAttribute('data-available')).toBe('false');
    expect(target.textContent).toContain('Thursday 24 September 2026');

    (target.querySelector('[data-testid="search-button"]') as HTMLButtonElement).click();
    expect(target.querySelector('[data-testid="availability-grid"]')).toBeTruthy();
    expect(target.querySelector('[title]')).toBeNull();
  });

  it('selects a floor table from the keyboard and leaves an unavailable table alone', async () => {
    const target = mount();
    const first = target.querySelector('[data-testid="plan-t_1"]') as SVGElement;
    first.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    expect(first.getAttribute('data-selected')).toBe('true');
    expect(target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 1');

    const later = target.querySelector('[data-testid="slot-t_1-19:00"]') as HTMLButtonElement;
    later.click();
    await tick();
    const taken = target.querySelector('[data-testid="plan-t_2"]') as SVGElement;
    expect(taken.getAttribute('data-available')).toBe('false');
    taken.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true }));
    await tick();
    expect(target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected')).toBe('true');
    expect(taken.getAttribute('data-selected')).toBe('false');
    expect(target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 1');
    expect(target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('19:00');
  });
});

describe('named presentations', () => {
  it('shows loading without a grid', () => {
    const target = mount({ presentation: 'loading' });
    expect(target.querySelector('[data-testid="loading-state"]')?.textContent).toContain('Zum Anker');
    expect(target.querySelector('[data-testid="availability-grid"]')).toBeNull();
    expect(target.querySelector('[data-testid="no-slots"]')).toBeNull();
    expect(target.querySelector('[data-testid="confirmation"]')).toBeNull();
  });

  it('shows an empty day instead of the grid', () => {
    const target = mount({ presentation: 'empty' });
    expect(target.querySelector('[data-testid="no-slots"]')?.textContent).toContain('Zum Anker');
    expect(target.querySelector('[data-testid="no-slots"]')?.textContent).toContain('No seating times');
    expect(target.querySelector('[data-testid="availability-grid"]')).toBeNull();
    expect(target.querySelector('[data-testid="floor-plan"]')).toBeNull();
  });

  it('shows a refusal and keeps the form', () => {
    const target = mount({ presentation: 'refused' });
    const error = target.querySelector('[data-testid="booking-error"]')?.textContent ?? '';
    expect(error.trim().length).toBeGreaterThan(0);
    expect(target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(target.querySelector('[data-testid="booking-summary"]')?.textContent).toContain('Table 1');
    expect(target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
    expect(target.querySelector('[data-testid="confirmation"]')).toBeNull();
  });

  it('shows an uncertain result without an error or a confirmation', () => {
    const target = mount({ presentation: 'uncertain' });
    const note = target.querySelector('[data-testid="booking-uncertain"]')?.textContent ?? '';
    expect(note.trim().length).toBeGreaterThan(0);
    expect(target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect((target.querySelector('[data-testid="booking-party-size"]') as HTMLInputElement).value).toBe('2');
    expect(target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(target.querySelector('[data-testid="confirmation"]')).toBeNull();
  });

  it('shows a confirmation without inventing a reference', () => {
    const target = mount({ presentation: 'confirmed' });
    const details = target.querySelector('[data-testid="confirmation-details"]')?.textContent ?? '';
    expect(details).toContain('Zum Anker');
    expect(details).toContain('Table 1');
    expect(details).toContain('18:00');
    expect(details).toContain('6:00 PM');
    expect(target.querySelector('[data-testid="confirmation-tables"]')?.textContent).toContain('1');
    expect(target.querySelector('[data-testid="confirmation-reference"]')).toBeNull();
    expect(target.querySelector('[data-testid="booking-form"]')).toBeTruthy();
    expect(target.querySelector('[data-testid="booking-error"]')).toBeNull();
    expect(target.querySelector('[data-testid="booking-uncertain"]')).toBeNull();
  });
});

describe('account shells', () => {
  it('renders signup, login and lookup hooks without a live session', () => {
    const signup = mount({ screen: 'signup' });
    for (const id of ['signup-email', 'signup-password', 'signup-display-name']) {
      expect(signup.querySelector(`label[for="${id}"]`)).toBeTruthy();
      expect(signup.querySelector(`[data-testid="${id}"]`)).toBeTruthy();
    }
    expect(signup.querySelector('[data-testid="signup-submit"]')).toBeTruthy();
    expect(signup.querySelector('[data-testid="auth-error"]')).toBeNull();
    (signup.querySelector('[data-testid="signup-submit"]') as HTMLButtonElement).click();
    expect(signup.querySelector('[data-testid="auth-error"]')).toBeNull();

    const login = mount({ screen: 'login' });
    expect(login.querySelector('label[for="login-email"]')).toBeTruthy();
    expect(login.querySelector('[data-testid="login-password"]')).toBeTruthy();
    expect(login.querySelector('[data-testid="login-submit"]')).toBeTruthy();
    expect(login.querySelector('[data-testid="auth-error"]')).toBeNull();

    const refused = mount({ screen: 'login', authError: 'Those sign-in details were not accepted.' });
    expect(refused.querySelector('[data-testid="auth-error"]')?.textContent).toContain('not accepted');

    const lookup = mount({ screen: 'lookup' });
    expect(lookup.querySelector('label[for="lookup-reference-input"]')).toBeTruthy();
    expect(lookup.querySelector('[data-testid="lookup-submit"]')).toBeTruthy();
    expect(lookup.querySelector('[data-testid="reservation-detail"]')).toBeNull();
    expect(lookup.querySelector('[data-testid="reservation-error"]')).toBeNull();
    (lookup.querySelector('[data-testid="lookup-submit"]') as HTMLButtonElement).click();
    expect(lookup.querySelector('[data-testid="reservation-detail"]')).toBeNull();

    const missing = mount({ screen: 'lookup', lookupError: 'No reservation matches that reference.' });
    expect(missing.querySelector('[data-testid="reservation-error"]')?.textContent).toContain('No reservation');
    expect(missing.querySelector('[data-testid="reservation-detail"]')).toBeNull();
  });

  it('shows the signed-in name and a lookup record only when one is supplied', () => {
    const target = mount({ screen: 'lookup', user: 'River' });
    expect(target.querySelector('[data-testid="current-user"]')?.textContent).toContain('River');
    expect(target.querySelector('[data-testid="logout-button"]')).toBeTruthy();

    const found = mount({
      screen: 'lookup',
      reservation: {
        reference: 'RIVER7',
        status: 'confirmed',
        summary: 'Zum Anker · Table 1 · Thursday 24 September 2026 at 6:00 PM (18:00)',
        tables: 'Table 1',
      },
    });
    expect(found.querySelector('[data-testid="reservation-status"]')?.textContent).toBe('confirmed');
    expect(found.querySelector('[data-testid="reservation-tables"]')?.textContent).toContain('1');
    expect(found.querySelector('[data-testid="reservation-cancel-button"]')).toBeTruthy();

    const cancelled = mount({
      screen: 'lookup',
      reservation: {
        reference: 'RIVER7',
        status: 'cancelled',
        summary: 'Zum Anker · Table 1',
        tables: 'Table 1',
      },
    });
    expect(cancelled.querySelector('[data-testid="reservation-status"]')?.textContent).toBe('cancelled');
    expect(cancelled.querySelector('[data-testid="reservation-cancel-button"]')).toBeNull();
  });

  it('writes a confirmation reference exactly when the caller supplies one', () => {
    const view = render(Confirmation, {
      details: 'Zum Anker · Table 1 · Thursday 24 September 2026 at 6:00 PM (18:00)',
      tables: 'Table 1',
      reference: 'RIVER7',
    });
    cleanups.push(view.cleanup);
    expect(view.target.querySelector('[data-testid="confirmation-reference"]')?.textContent).toBe('RIVER7');
  });
});

describe('theme control', () => {
  it('sets light, dark and system on the document', async () => {
    const target = mount();
    (target.querySelector('[data-testid="theme-dark"]') as HTMLButtonElement).click();
    await tick();
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(target.querySelector('[data-testid="theme-dark"]')?.getAttribute('aria-pressed')).toBe('true');
    expect(target.querySelector('[data-testid="theme-light"]')?.getAttribute('aria-pressed')).toBe('false');
    (target.querySelector('[data-testid="theme-light"]') as HTMLButtonElement).click();
    await tick();
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(target.querySelector('[data-testid="theme-light"]')?.getAttribute('aria-pressed')).toBe('true');
    (target.querySelector('[data-testid="theme-system"]') as HTMLButtonElement).click();
    await tick();
    expect(document.documentElement.dataset.theme).toBeUndefined();
    expect(target.querySelector('[data-testid="theme-system"]')?.getAttribute('aria-pressed')).toBe('true');
    expect(theme.mode).toBe('system');
  });
});
