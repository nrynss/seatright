import { afterEach, describe, expect, it } from 'vitest';
import AvailabilityGrid from '../lib/components/AvailabilityGrid.svelte';
import type { PreviewSlot, PreviewTable } from '../lib/preview';
import { render } from './render';

const OPENS = 18 * 60;
const CLOSES = 23 * 60;
const DURATION = 90;

const tables: PreviewTable[] = [
  { id: 't_1', label: 'Window alcove', capacity: 2 },
  { id: 't_2', label: 'Garden corner', capacity: 4 },
  { id: 't_3', label: 'Hearth booth', capacity: 4 },
];

const pairs = [
  { ids: ['t_1', 't_2'], labels: ['Window alcove', 'Garden corner'], capacity: 6 },
  { ids: ['t_2', 't_3'], labels: ['Garden corner', 'Hearth booth'], capacity: 8 },
];

function timesFor(step: number): string[] {
  const times: string[] = [];
  for (let minute = OPENS; minute + DURATION <= CLOSES; minute += step) {
    const hh = String(Math.floor(minute / 60)).padStart(2, '0');
    const mm = String(minute % 60).padStart(2, '0');
    times.push(`${hh}:${mm}`);
  }
  return times;
}

function slotsFor(step: number): PreviewSlot[] {
  return timesFor(step).map((time) => ({
    time,
    availableTableIds: [],
    options: [
      { tableIds: ['t_1', 't_2'], capacity: 6 },
      { tableIds: ['t_2', 't_3'], capacity: 8 },
    ],
  }));
}

const cleanups: Array<() => Promise<void>> = [];

afterEach(async () => {
  while (cleanups.length > 0) {
    const cleanup = cleanups.pop();
    if (cleanup) await cleanup();
  }
});

describe('dense availability grids', () => {
  it.each([
    [15, 15, 75],
    [5, 43, 215],
  ])('renders every returned %i-minute slot', (step, slotCount, cellCount) => {
    const times = timesFor(step);
    expect(times).toHaveLength(slotCount);
    expect(times[0]).toBe('18:00');
    expect(times[times.length - 1]).toBe('21:30');
    const picked: Array<{ ids: readonly string[]; time: string; available: boolean }> = [];
    const view = render(AvailabilityGrid, {
      tables,
      slots: slotsFor(step),
      date: '2027-06-17',
      pairs,
      onSelect: (id, time, available) => picked.push({ ids: [id], time, available }),
      onSelectPair: (ids, time, available) => picked.push({ ids, time, available }),
    });
    cleanups.push(view.cleanup);

    const cells = [...view.target.querySelectorAll('.cell')] as HTMLButtonElement[];
    expect(cells).toHaveLength(cellCount);
    const ids = cells.map((cell) => cell.getAttribute('data-testid'));
    expect(new Set(ids).size).toBe(cellCount);
    for (const time of times) {
      for (const id of ['t_1', 't_2', 't_3']) {
        const cell = view.target.querySelector(`[data-testid="slot-${id}-${time}"]`);
        expect(cell?.getAttribute('data-available')).toBe('false');
      }
      expect(view.target.querySelector(`[data-testid="slot-t_1+t_2-${time}"]`)?.getAttribute('data-available')).toBe('true');
      expect(view.target.querySelector(`[data-testid="slot-t_2+t_3-${time}"]`)?.getAttribute('data-available')).toBe('true');
    }

    const heads = [...view.target.querySelectorAll('.rowhead')].map((head) => head.textContent ?? '');
    expect(heads.some((head) => head.includes('Table Window alcove and Table Garden corner'))).toBe(true);
    expect(heads.some((head) => head.includes('Table Garden corner and Table Hearth booth'))).toBe(true);
    expect(heads.join(' ')).not.toContain('…');
    expect(view.target.querySelector('.grid-caption')?.textContent).toBe('Thursday 17 June 2027');

    (view.target.querySelector('[data-testid="slot-t_1-18:00"]') as HTMLButtonElement).click();
    (view.target.querySelector('[data-testid="slot-t_1+t_2-21:30"]') as HTMLButtonElement).click();
    expect(picked).toEqual([
      { ids: ['t_1'], time: '18:00', available: false },
      { ids: ['t_1', 't_2'], time: '21:30', available: true },
    ]);
  });
});
