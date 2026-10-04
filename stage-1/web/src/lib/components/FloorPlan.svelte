<script lang="ts">
  import { flip } from 'svelte/animate';
  import { layoutRoom } from '../floor';
  import { formatClock } from '../format';
  import { motionDuration } from '../motion';
  import type { PreviewTable } from '../preview';
  import PlanTable from './PlanTable.svelte';

  let {
    tables,
    availableIds,
    selectedId = null,
    time,
    onSelect,
  }: {
    tables: readonly PreviewTable[];
    availableIds: readonly string[];
    selectedId?: string | null;
    time: string;
    onSelect: (tableId: string) => void;
  } = $props();

  const clock = $derived(formatClock(time));
  const scene = $derived(layoutRoom(tables));
  const ordered = $derived.by(() => {
    if (!selectedId) return [...tables];
    const picked = tables.filter((table) => table.id === selectedId);
    const rest = tables.filter((table) => table.id !== selectedId);
    return [...picked, ...rest];
  });
</script>

<section class="room" data-testid="floor-plan" aria-label="Restaurant floor plan">
  <svg
    class="room-scene"
    viewBox={`0 0 ${scene.width} ${scene.height}`}
    role="group"
    aria-label={`Dining room at ${clock}`}
  >
    <rect class="room-floor" x="1" y="1" width={scene.width - 2} height={scene.height - 2} rx="28" />
    <rect class="room-wall" x="14" y="14" width={scene.width - 28} height={scene.height - 28} rx="20" />
    {#each scene.windows as window, index (`window-${index}`)}
      <rect class="room-window" x={window.x} y={window.y} width={window.w} height={window.h} rx="4" />
    {/each}
    <rect class="room-aisle" x={scene.aisle.x} y={scene.aisle.y} width={scene.aisle.w} height={scene.aisle.h} rx="8" />
    <rect class="room-bar" x={scene.bar.x} y={scene.bar.y} width={scene.bar.w} height={scene.bar.h} rx="6" />
    <rect class="room-door" x={scene.door.x} y={scene.door.y} width={scene.door.w} height={scene.door.h} rx="4" />
    <text class="room-note" x={scene.aisle.x + scene.aisle.w / 2} y={scene.aisle.y - 10} text-anchor="middle">Aisle</text>
    <text class="room-note" x={scene.door.x + scene.door.w / 2} y={scene.door.y - 8} text-anchor="middle">Entrance</text>
    {#each scene.tables as place (place.id)}
      <PlanTable
        {place}
        available={availableIds.includes(place.id)}
        selected={selectedId === place.id}
        {clock}
        {onSelect}
      />
    {/each}
  </svg>
  <p class="floor-caption">Floor at {clock}. Tables share one room and are drawn to their number of seats.</p>
  <ul class="place-cards" aria-label="Tables at this time">
    {#each ordered as table (table.id)}
      <li animate:flip={{ duration: motionDuration(360) }} data-selected={selectedId === table.id ? 'true' : 'false'}>
        Table {table.label} · {table.capacity} seats
      </li>
    {/each}
  </ul>
</section>
