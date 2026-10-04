<script lang="ts">
  import { fly } from 'svelte/transition';
  import { formatClock, formatLongDate } from '../format';
  import { motionDuration, staggerDelay } from '../motion';
  import { slotTestId, tableIsAvailable, type PreviewSlot, type PreviewTable } from '../preview';

  let {
    tables,
    slots,
    date,
    selectedId = null,
    selectedTime = null,
    onSelect,
  }: {
    tables: readonly PreviewTable[];
    slots: readonly PreviewSlot[];
    date: string;
    selectedId?: string | null;
    selectedTime?: string | null;
    onSelect: (tableId: string, time: string, available: boolean) => void;
  } = $props();
</script>

<div class="matrix-wrap" data-testid="availability-grid">
  <p class="grid-caption" id="grid-caption">{formatLongDate(date)}</p>
  <div class="matrix" style:--cols={slots.length} role="group" aria-labelledby="grid-caption">
    <div class="corner" aria-hidden="true"></div>
    {#each slots as slot (slot.time)}
      <div class="colhead">
        <div>{formatClock(slot.time)}</div>
        <div class="raw">{slot.time}</div>
      </div>
    {/each}
    {#each tables as table, row (table.id)}
      <div class="rowhead">
        <span>Table {table.label}</span>
        <span class="seats">{table.capacity} seats</span>
      </div>
      {#each slots as slot, column (`${table.id}-${slot.time}`)}
        {@const available = tableIsAvailable(slot, table.id)}
        {@const selected = selectedId === table.id && selectedTime === slot.time}
        {@const word = selected ? 'Selected' : available ? 'Available' : 'Unavailable'}
        {@const short = selected ? 'Held' : available ? 'Free' : 'Taken'}
        <button
          type="button"
          class="cell"
          data-testid={slotTestId(table.id, slot.time)}
          data-available={available ? 'true' : 'false'}
          data-selected={selected ? 'true' : 'false'}
          aria-label={`Table ${table.label} at ${formatClock(slot.time)} (${slot.time}), ${word}`}
          in:fly={{ y: 8, opacity: 1, duration: motionDuration(240), delay: staggerDelay(row * slots.length + column) }}
          onclick={() => onSelect(table.id, slot.time, available)}
        >
          <span class="state-word">{short}</span>
          <span class="sr-only">Table {table.label} {slot.time}</span>
        </button>
      {/each}
    {/each}
  </div>
</div>
