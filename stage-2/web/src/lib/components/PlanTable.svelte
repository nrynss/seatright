<svelte:options namespace="svg" />

<script lang="ts">
  import { Spring } from 'svelte/motion';
  import { captionLayout, plateLayout, type PlacedTable } from '../floor';
  import { motionDuration, prefersReducedMotion, springOptions } from '../motion';

  let {
    place,
    available,
    selected,
    clock,
    onSelect,
  }: {
    place: PlacedTable;
    available: boolean;
    selected: boolean;
    clock: string;
    onSelect: (tableId: string) => void;
  } = $props();

  const drawing = $derived(place.drawing);
  const lift = new Spring(1, springOptions());

  $effect(() => {
    const next = selected ? 1.06 : 1;
    void lift.set(next, { instant: prefersReducedMotion() || motionDuration(1) === 0 });
  });

  const stateWord = $derived(selected ? 'Selected' : available ? 'Available' : 'Unavailable');
  const name = $derived(
    `Table ${place.label}, ${place.capacity} seats, ${stateWord.toLowerCase()} at ${clock}`,
  );
  const plate = $derived(plateLayout(place.label, drawing));
  const caption = $derived(captionLayout(place.label, drawing));
  const seatsLabel = $derived(`${place.capacity} ${place.capacity === 1 ? 'seat' : 'seats'}`);
  const hitHeight = $derived(
    Math.max(drawing.size + 28, drawing.size + 6 + caption.lines.length * caption.lineHeight),
  );
  const transform = $derived.by(() => {
    const size = drawing.size;
    const cx = place.x + size / 2;
    const cy = place.y + size / 2;
    return `translate(${cx} ${cy}) scale(${lift.current}) translate(${-size / 2} ${-size / 2})`;
  });

  function activate(): void {
    onSelect(place.id);
  }

  function onKey(event: KeyboardEvent): void {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    activate();
  }
</script>

<g
  class="table-on-plan"
  data-testid={`plan-${place.id}`}
  data-available={available ? 'true' : 'false'}
  data-selected={selected ? 'true' : 'false'}
  data-capacity={place.capacity}
  role="button"
  tabindex="0"
  aria-label={name}
  aria-pressed={selected}
  {transform}
  onclick={activate}
  onkeydown={onKey}
>
  <rect class="table-hit" x="0" y="0" width={drawing.size} height={hitHeight} />
  {#if drawing.kind === 'round'}
    <circle class="table-top" cx={drawing.cx} cy={drawing.cy} r={drawing.radius} />
  {:else if drawing.rect}
    <rect
      class="table-top"
      x={drawing.rect.x}
      y={drawing.rect.y}
      width={drawing.rect.w}
      height={drawing.rect.h}
      rx={drawing.rect.rx}
    />
  {/if}
  {#each drawing.seats as seat, index (`${place.id}-${index}`)}
    <circle class="seat" cx={seat.x} cy={seat.y} r={seat.r} />
  {/each}
  {#each plate.lines as line, index (`plate-${index}`)}
    <text
      class="plate-label"
      x={drawing.cx}
      y={drawing.cy + (index - (plate.lines.length - 1) / 2) * plate.lineHeight}
      text-anchor="middle"
      dominant-baseline="central"
      font-size={plate.fontSize}
    >
      {line}
    </text>
  {/each}
  <text
    class="plan-seats"
    x={drawing.cx}
    y={drawing.size + 6}
    text-anchor="middle"
    dominant-baseline="hanging"
    font-size="16"
  >
    {seatsLabel}
  </text>
</g>
