/** Explicit visual preview. This is not a live availability result and must not
 * stand in for a successful API response. */

export interface PreviewTable {
  id: string;
  label: string;
  capacity: number;
}

export interface PreviewOption {
  tableIds: readonly string[];
  capacity: number;
}

export interface PreviewSlot {
  time: string;
  availableTableIds: readonly string[];
  options?: readonly PreviewOption[];
}

export interface PreviewFixture {
  restaurant: {
    id: string;
    name: string;
    timezone: string;
  };
  date: string;
  partySize: number;
  tables: readonly PreviewTable[];
  slots: readonly PreviewSlot[];
}

export const previewFixture: PreviewFixture = {
  restaurant: {
    id: 'r_anker',
    name: 'Zum Anker',
    timezone: 'Europe/Berlin',
  },
  date: '2026-09-24',
  partySize: 2,
  tables: [
    { id: 't_1', label: '1', capacity: 2 },
    { id: 't_2', label: '2', capacity: 4 },
  ],
  slots: [
    { time: '18:00', availableTableIds: ['t_1', 't_2'] },
    { time: '18:30', availableTableIds: ['t_1', 't_2'] },
    { time: '19:00', availableTableIds: ['t_1'] },
  ],
};

export function tableById(fixture: PreviewFixture, id: string): PreviewTable | undefined {
  return fixture.tables.find((table) => table.id === id);
}

export function slotByTime(fixture: PreviewFixture, time: string): PreviewSlot | undefined {
  return fixture.slots.find((slot) => slot.time === time);
}

export function tableIsAvailable(slot: PreviewSlot, tableId: string): boolean {
  return slot.availableTableIds.includes(tableId);
}

/** Grid hook required by the booking screen: slot-{table_id}-{HH:MM}. */
export function slotTestId(tableId: string, time: string): string {
  return `slot-${tableId}-${time}`;
}
