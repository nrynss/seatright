import { tick } from 'svelte';
import { describe, expect, it } from 'vitest';
import FloorPlan from '../lib/components/FloorPlan.svelte';
import { layoutRoom, pairBadgeCollision, pairCaptionWidth, roomForPairs } from '../lib/floor';
import { render } from './render';

const tables = [
  { id: 't_1', label: 'Window alcove', capacity: 2 },
  { id: 't_2', label: 'Garden corner', capacity: 4 },
  { id: 't_3', label: 'Hearth booth', capacity: 4 },
];

const captions = [
  { ids: ['t_1', 't_2'], caption: 'Window alcove · Garden corner' },
  { ids: ['t_2', 't_3'], caption: 'Garden corner · Hearth booth' },
];

describe('pair badge geometry', () => {
  it('estimates caption ink at or above the measured Source Serif width', () => {
    expect(pairCaptionWidth('1 · 2')).toBeGreaterThanOrEqual(25);
    expect(pairCaptionWidth('Window alcove · Garden corner')).toBeGreaterThanOrEqual(201);
    expect(pairCaptionWidth('Garden corner · Hearth booth')).toBeGreaterThanOrEqual(189);
    expect(pairCaptionWidth('North window · South garden')).toBeGreaterThanOrEqual(187);
  });

  it('keeps a short caption on the chip between the tables', () => {
    const fitted = roomForPairs(
      [
        { id: 'a', label: '1', capacity: 2 },
        { id: 'b', label: '2', capacity: 4 },
      ],
      [{ ids: ['a', 'b'], caption: '1 · 2' }],
    );
    expect(fitted.badges[0]?.text).toBe('1 · 2');
    expect(fitted.badges[0]?.w).toBeGreaterThanOrEqual(pairCaptionWidth('1 · 2'));
    expect(pairBadgeCollision(fitted.scene, fitted.badges)).toBeNull();
    expect(fitted.scene.width).toBe(layoutRoom([
      { id: 'a', label: '1', capacity: 2 },
      { id: 'b', label: '2', capacity: 4 },
    ]).width);
  });

  it('places long labels clear of tables, seats and names at both column counts', () => {
    for (const maxColumns of [0, 1]) {
      const fitted = roomForPairs(tables, captions, { maxColumns });
      expect(fitted.badges.map((badge) => badge.text), `columns ${maxColumns}`).toEqual(captions.map((pair) => pair.caption));
      expect(pairBadgeCollision(fitted.scene, fitted.badges), `columns ${maxColumns}`).toBeNull();
      for (const badge of fitted.badges) {
        expect(badge.w).toBeGreaterThanOrEqual(pairCaptionWidth(badge.text));
        expect(badge.h).toBeGreaterThanOrEqual(28);
      }
    }
  });
});

describe('pair badge control', () => {
  it('keeps full names on the button, the chip and the place labels', async () => {
    let picked: readonly string[] | null = null;
    const view = render(FloorPlan, {
      tables,
      availableIds: [],
      selectedIds: ['t_1', 't_2'],
      time: '18:00',
      pairs: [
        { ids: ['t_1', 't_2'], labels: ['Window alcove', 'Garden corner'], capacity: 6, available: true },
        { ids: ['t_2', 't_3'], labels: ['Garden corner', 'Hearth booth'], capacity: 8, available: false },
      ],
      onSelect: () => {},
      onSelectPair: (ids) => {
        picked = [...ids];
      },
    });
    const held = view.target.querySelector('[data-testid="plan-t_1+t_2"]');
    const other = view.target.querySelector('[data-testid="plan-t_2+t_3"]');
    expect(held?.getAttribute('data-selected')).toBe('true');
    expect(held?.getAttribute('data-available')).toBe('true');
    expect(held?.getAttribute('aria-pressed')).toBe('true');
    expect(held?.getAttribute('aria-label')).toContain('Window alcove');
    expect(held?.getAttribute('aria-label')).toContain('Garden corner');
    expect(held?.textContent).toContain('Window alcove');
    expect(held?.textContent).toContain('Garden corner');
    expect(held?.textContent).not.toContain('t_1');
    expect(other?.getAttribute('data-available')).toBe('false');
    expect(other?.getAttribute('data-selected')).toBe('false');
    expect(other?.textContent).toContain('Hearth booth');
    expect(view.target.querySelector('[data-testid="plan-t_1"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('[data-testid="plan-t_3"]')?.getAttribute('data-selected')).toBe('false');
    const cards = view.target.querySelector('.place-cards')?.textContent ?? '';
    expect(cards).toContain('Table Window alcove');
    expect(cards).toContain('Table Garden corner');
    expect(cards).toContain('Table Hearth booth');
    expect(view.target.querySelectorAll('svg.room-scene')).toHaveLength(1);
    const rect = held?.querySelector('rect');
    expect(Number(rect?.getAttribute('width'))).toBeGreaterThan(180);
    expect(Number(rect?.getAttribute('height'))).toBe(28);
    other?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    expect(picked).toEqual(['t_2', 't_3']);
    await view.cleanup();
  });
});
