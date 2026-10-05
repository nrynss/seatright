<script lang="ts">
  import { flip } from 'svelte/animate';
  import { roomForPairs, type PlacedTable } from '../floor';
  import { formatClock, tablePhrase } from '../format';
  import { motionDuration } from '../motion';
  import type { PreviewTable } from '../preview';
  import { pairPlanTestId, sameMembers } from '../seating';
  import PairMark from './PairMark.svelte';
  import PlanTable from './PlanTable.svelte';

  interface FloorPair {
    ids: readonly string[];
    labels: readonly string[];
    capacity: number;
    available: boolean;
  }

  let {
    tables,
    availableIds,
    selectedId = null,
    selectedIds = null,
    time,
    pairs = [],
    onSelect,
    onSelectPair = null,
  }: {
    tables: readonly PreviewTable[];
    availableIds: readonly string[];
    selectedId?: string | null;
    selectedIds?: readonly string[] | null;
    time: string;
    pairs?: readonly FloorPair[];
    onSelect: (tableId: string) => void;
    onSelectPair?: ((ids: readonly string[]) => void) | null;
  } = $props();

  const clock = $derived(formatClock(time));
  const narrowQuery = '(max-width: 700px)';

  function readNarrow(): boolean {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false;
    return window.matchMedia(narrowQuery).matches;
  }

  let narrow = $state(readNarrow());

  $effect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    const query = window.matchMedia(narrowQuery);
    const sync = () => {
      narrow = query.matches;
    };
    sync();
    query.addEventListener('change', sync);
    return () => query.removeEventListener('change', sync);
  });

  const fitted = $derived(
    roomForPairs(
      tables,
      pairs.map((pair) => ({ ids: pair.ids, caption: pair.labels.join(' · ') })),
      { maxColumns: narrow ? 1 : 0 },
    ),
  );
  const scene = $derived(fitted.scene);
  const pickedIds = $derived(selectedIds != null ? [...selectedIds] : selectedId ? [selectedId] : []);
  const ordered = $derived.by(() => {
    if (pickedIds.length === 0) return [...tables];
    const picked = tables.filter((table) => pickedIds.includes(table.id));
    const rest = tables.filter((table) => !pickedIds.includes(table.id));
    return [...picked, ...rest];
  });
  const heldTogether = $derived(pairs.find((pair) => sameMembers(pair.ids, pickedIds)) ?? null);

  function center(place: PlacedTable): { x: number; y: number } {
    return { x: place.x + place.drawing.size / 2, y: place.y + place.drawing.size / 2 };
  }

  const marks = $derived.by(() => {
    return pairs.flatMap((pair, index) => {
      const left = scene.tables.find((place) => place.id === pair.ids[0]);
      const right = scene.tables.find((place) => place.id === pair.ids[1]);
      const badge = fitted.badges[index];
      if (!left || !right || !badge) return [];
      const a = center(left);
      const b = center(right);
      const names = tablePhrase(pair.labels);
      const state = sameMembers(pair.ids, pickedIds) ? 'Selected' : pair.available ? 'Available' : 'Unavailable';
      return [
        {
          key: pair.ids.join('+'),
          ids: pair.ids,
          x1: a.x,
          y1: a.y,
          x2: b.x,
          y2: b.y,
          badge: { x: badge.x, y: badge.y, w: badge.w, h: badge.h },
          caption: badge.text,
          available: pair.available,
          selected: sameMembers(pair.ids, pickedIds),
          testId: pairPlanTestId(pair.ids),
          label: `${names} together, ${pair.capacity} seats, ${state.toLowerCase()} at ${clock}`,
        },
      ];
    });
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
        selected={pickedIds.includes(place.id)}
        {clock}
        {onSelect}
      />
    {/each}
    {#each marks as mark (mark.key)}
      <PairMark
        x1={mark.x1}
        y1={mark.y1}
        x2={mark.x2}
        y2={mark.y2}
        badge={mark.badge}
        caption={mark.caption}
        available={mark.available}
        selected={mark.selected}
        testId={mark.testId}
        label={mark.label}
        onSelect={() => onSelectPair?.(mark.ids)}
      />
    {/each}
  </svg>
  <p class="floor-caption">
    Floor at {clock}. Tables share one room and are drawn to their number of seats.
    {#if heldTogether}{tablePhrase(heldTogether.labels)} are held together.{/if}
  </p>
  <ul class="place-cards" aria-label="Tables at this time">
    {#each ordered as table (table.id)}
      <li animate:flip={{ duration: motionDuration(360) }} data-selected={pickedIds.includes(table.id) ? 'true' : 'false'}>
        Table {table.label} · {table.capacity} seats
      </li>
    {/each}
  </ul>
</section>
