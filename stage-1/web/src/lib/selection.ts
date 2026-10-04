import type { PreviewFixture, PreviewSlot } from './preview';
import { tableIsAvailable } from './preview';

export interface Selection {
  tableId: string;
  time: string;
}

/** Unavailable clicks keep the current selection. Available clicks replace it at once. */
export function chooseCell(
  current: Selection | null,
  tableId: string,
  time: string,
  available: boolean,
): Selection | null {
  if (!available) return current;
  return { tableId, time };
}

export function activeTime(slots: readonly Pick<PreviewSlot, 'time'>[], selected: Selection | null): string | null {
  if (selected) return selected.time;
  return slots[0]?.time ?? null;
}

export function firstAvailable(fixture: PreviewFixture): Selection | null {
  for (const slot of fixture.slots) {
    for (const table of fixture.tables) {
      if (tableIsAvailable(slot, table.id)) return { tableId: table.id, time: slot.time };
    }
  }
  return null;
}
