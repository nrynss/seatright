import type { CommittedSearch } from './search';

/** Frozen choice for a later booking submit. Party size is the size that was searched. */
export interface HeldSelection {
  restaurant_id: string;
  table_id: string;
  starts_at_local: string;
  party_size: number;
  display: {
    restaurantName: string;
    tableLabel: string;
    timezone: string;
  };
}

let search: CommittedSearch | null = null;
let hold: HeldSelection | null = null;

export function rememberSearch(next: CommittedSearch | null): void {
  search = next;
}

export function rememberHold(next: HeldSelection | null): void {
  hold = next;
}

export function currentSearch(): CommittedSearch | null {
  return search;
}

export function currentHold(): HeldSelection | null {
  return hold;
}
