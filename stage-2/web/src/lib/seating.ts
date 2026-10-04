/** Declared pairs and the seating set a diner can hold. Arrays that leave this module are copies. */

export interface SeatingOption {
  tableIds: readonly string[];
  capacity: number;
}

export function copyIds(ids: readonly string[]): string[] {
  return [...ids];
}

export function sameIds(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((id, index) => id === right[index]);
}

/** Order does not matter. A reversed pair is the same seating. */
export function sameMembers(left: readonly string[], right: readonly string[]): boolean {
  if (left.length !== right.length) return false;
  const pending = [...right];
  return left.every((id) => {
    const index = pending.indexOf(id);
    if (index < 0) return false;
    pending.splice(index, 1);
    return true;
  });
}

/** Declared combinable order when the ids are that pair. Otherwise a copy of the input. */
export function canonicalTableIds(combinable: readonly (readonly string[])[], ids: readonly string[]): string[] {
  const copy = copyIds(ids);
  if (copy.length !== 2) return copy;
  for (const pair of combinable) {
    if (pair.length === 2 && sameMembers(pair, copy)) return [pair[0], pair[1]];
  }
  return copy;
}

export function pairKey(ids: readonly string[]): string {
  return ids.join('+');
}

/** Grid hook for a declared pair: slot-{t_a}+{t_b}-{HH:MM}. */
export function pairSlotTestId(ids: readonly string[], time: string): string {
  return `slot-${pairKey(ids)}-${time}`;
}

export function pairPlanTestId(ids: readonly string[]): string {
  return `plan-${pairKey(ids)}`;
}

export function listedPairAvailable(
  options: readonly Pick<SeatingOption, 'tableIds'>[] | undefined,
  declared: readonly string[],
): boolean {
  if (!options || declared.length < 2) return false;
  return options.some((option) => option.tableIds.length === declared.length && sameMembers(option.tableIds, declared));
}
