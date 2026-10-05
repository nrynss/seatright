import { describe, expect, it } from 'vitest';
import FloorPlan from '../lib/components/FloorPlan.svelte';
import {
  captionLayout,
  drawTable,
  labelCollision,
  layoutRoom,
  pairBadgeCollision,
  plateLayout,
  roomForPairs,
} from '../lib/floor';
import { render } from './render';

const longTables = [
  { id: 't_1', label: 'Window alcove', capacity: 2 },
  { id: 't_2', label: 'Garden corner', capacity: 4 },
  { id: 't_3', label: 'Hearth booth', capacity: 4 },
];

const longCaptions = [
  { ids: ['t_1', 't_2'], caption: 'Window alcove · Garden corner' },
  { ids: ['t_2', 't_3'], caption: 'Garden corner · Hearth booth' },
];

describe('floor plate geometry', () => {
  it('keeps a short numeral on one large line', () => {
    const two = drawTable(2);
    const four = drawTable(4);
    const plate = plateLayout('1', two);
    const name = captionLayout('1', two);
    expect(plate.lines).toEqual(['1']);
    expect(plate.fontSize).toBeGreaterThanOrEqual(two.labelSize - 0.01);
    expect(name.lines).toEqual(['Table 1']);
    expect(name.fontSize).toBe(16);
    expect(plateLayout('2', four).lines).toEqual(['2']);
    expect(plateLayout('2', four).fontSize).toBeGreaterThanOrEqual(four.labelSize - 0.01);
    expect(captionLayout('2', four).lines).toEqual(['Table 2']);
    const scene = layoutRoom([
      { id: 't_1', label: '1', capacity: 2 },
      { id: 't_2', label: '2', capacity: 4 },
    ]);
    expect(labelCollision(scene)).toBeNull();
  });

  it('fits long names on their own tops without moving the room or the pair caption', () => {
    for (const table of longTables) {
      const drawing = drawTable(table.capacity);
      const plate = plateLayout(table.label, drawing);
      const caption = captionLayout(table.label, drawing);
      expect(plate.lines.join(' '), table.label).toBe(table.label);
      expect(caption.lines.join(' '), table.label).toBe(`Table ${table.label}`);
      expect(plate.fontSize, table.label).toBeGreaterThanOrEqual(13);
      expect(caption.fontSize, table.label).toBeGreaterThanOrEqual(13);
      expect(plate.lines.length, table.label).toBeGreaterThan(1);
    }
    for (const maxColumns of [0, 1]) {
      const scene = layoutRoom(longTables, { maxColumns });
      expect(labelCollision(scene), `columns ${maxColumns}`).toBeNull();
      const fitted = roomForPairs(longTables, longCaptions, { maxColumns });
      expect(fitted.scene.width, `columns ${maxColumns}`).toBe(scene.width);
      expect(fitted.scene.height, `columns ${maxColumns}`).toBe(scene.height);
      expect(fitted.badges.map((badge) => badge.text), `columns ${maxColumns}`).toEqual(
        longCaptions.map((pair) => pair.caption),
      );
      expect(pairBadgeCollision(fitted.scene, fitted.badges), `columns ${maxColumns}`).toBeNull();
    }
  });

  it('fits other names from the drawing width rather than a fixed example', () => {
    const labels = ['1', '12', 'Bar', 'North window', 'Hearth booth'];
    for (const capacity of [2, 4, 6, 8]) {
      for (const label of labels) {
        const drawing = drawTable(capacity);
        const plate = plateLayout(label, drawing);
        expect(plate.lines.join(' '), `${capacity} ${label}`).toBe(label);
        expect(plate.fontSize, `${capacity} ${label}`).toBeGreaterThanOrEqual(11);
        expect(captionLayout(label, drawing).lines.join(' ')).toBe(`Table ${label}`);
        const scene = layoutRoom([{ id: 'seat', label, capacity }]);
        expect(labelCollision(scene), `${capacity} ${label}`).toBeNull();
      }
    }
    const single = layoutRoom([{ id: 'seat', label: 'Bar', capacity: 1 }]);
    expect(plateLayout('Bar', drawTable(1)).lines).toEqual(['Bar']);
    expect(labelCollision(single)).toBeNull();
  });
});

describe('floor plate control', () => {
  it('shows the short numeral on the plate and the full name beside it', async () => {
    const view = render(FloorPlan, {
      tables: [
        { id: 't_1', label: '1', capacity: 2 },
        { id: 't_2', label: '2', capacity: 4 },
      ],
      availableIds: ['t_1'],
      selectedIds: [],
      time: '18:00',
      pairs: [{ ids: ['t_1', 't_2'], labels: ['1', '2'], capacity: 6, available: true }],
      onSelect: () => {},
    });
    const first = view.target.querySelector('[data-testid="plan-t_1"]');
    const plates = [...(first?.querySelectorAll('.plate-label') ?? [])].map((node) => node.textContent?.trim());
    const names = [...(first?.querySelectorAll('.plan-name') ?? [])].map((node) => node.textContent?.trim());
    expect(plates).toEqual(['1']);
    expect(names).toEqual(['Table 1']);
    expect(first?.getAttribute('aria-label')).toContain('Table 1');
    expect(first?.getAttribute('data-available')).toBe('true');
    expect(first?.getAttribute('data-selected')).toBe('false');
    const badge = view.target.querySelector('[data-testid="plan-t_1+t_2"]');
    expect(badge?.textContent).toContain('1');
    expect(badge?.textContent).not.toContain('t_1');
    const cards = view.target.querySelector('.place-cards')?.textContent ?? '';
    expect(cards).toContain('Table 1');
    expect(cards).toContain('Table 2');
    await view.cleanup();
  });

  it('wraps a long plate name and keeps that name on the button, caption and place card', async () => {
    const view = render(FloorPlan, {
      tables: longTables,
      availableIds: [],
      selectedIds: ['t_1', 't_2'],
      time: '18:00',
      pairs: [
        { ids: ['t_1', 't_2'], labels: ['Window alcove', 'Garden corner'], capacity: 6, available: true },
        { ids: ['t_2', 't_3'], labels: ['Garden corner', 'Hearth booth'], capacity: 8, available: false },
      ],
      onSelect: () => {},
    });
    const first = view.target.querySelector('[data-testid="plan-t_1"]');
    const plate = [...(first?.querySelectorAll('.plate-label') ?? [])].map((node) => node.textContent?.trim()).join(' ');
    const caption = [...(first?.querySelectorAll('.plan-name') ?? [])].map((node) => node.textContent?.trim()).join(' ');
    expect(plate).toBe('Window alcove');
    expect(caption).toBe('Table Window alcove');
    expect(Number(first?.querySelector('.plate-label')?.getAttribute('font-size'))).toBeGreaterThanOrEqual(13);
    expect(first?.getAttribute('aria-label')).toContain('Window alcove');
    expect(first?.getAttribute('data-selected')).toBe('true');
    expect(first?.getAttribute('data-available')).toBe('false');
    expect(view.target.querySelector('[data-testid="plan-t_2"]')?.getAttribute('data-selected')).toBe('true');
    expect(view.target.querySelector('.place-cards')?.textContent).toContain('Table Window alcove');
    expect(view.target.querySelector('.place-cards')?.textContent).toContain('Table Garden corner');
    expect(view.target.querySelector('[data-testid="plan-t_1+t_2"]')?.textContent).toContain('Window alcove');
    await view.cleanup();
  });
});
