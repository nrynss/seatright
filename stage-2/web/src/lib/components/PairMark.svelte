<svelte:options namespace="svg" />

<script lang="ts">
  import { Spring } from 'svelte/motion';
  import { PAIR_BADGE_SCALE } from '../floor';
  import { motionDuration, prefersReducedMotion, springOptions } from '../motion';

  let {
    x1,
    y1,
    x2,
    y2,
    badge,
    caption,
    available,
    selected,
    testId,
    label,
    onSelect,
    layer,
  }: {
    x1: number;
    y1: number;
    x2: number;
    y2: number;
    badge: { x: number; y: number; w: number; h: number };
    caption: string;
    available: boolean;
    selected: boolean;
    testId: string;
    label: string;
    onSelect: () => void;
    /** Connector paint sits behind the tables. The badge stays in front. */
    layer: 'link' | 'badge';
  } = $props();

  const lift = new Spring(1, springOptions());

  $effect(() => {
    const next = selected ? PAIR_BADGE_SCALE : 1;
    void lift.set(next, { instant: prefersReducedMotion() || motionDuration(1) === 0 });
  });

  const transform = $derived.by(() => {
    const cx = badge.x + badge.w / 2;
    const cy = badge.y + badge.h / 2;
    return `translate(${cx} ${cy}) scale(${lift.current}) translate(${-cx} ${-cy})`;
  });

  function onKey(event: KeyboardEvent): void {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    onSelect();
  }
</script>

{#if layer === 'link'}
  <g class="pair-mark" data-available={available ? 'true' : 'false'} data-selected={selected ? 'true' : 'false'} aria-hidden="true">
    <path class="pair-link" d={`M ${x1} ${y1} L ${x2} ${y2}`} />
  </g>
{:else}
  <g
    class="pair-badge"
    data-testid={testId}
    data-available={available ? 'true' : 'false'}
    data-selected={selected ? 'true' : 'false'}
    role="button"
    tabindex="0"
    aria-label={label}
    aria-pressed={selected}
    {transform}
    onclick={onSelect}
    onkeydown={onKey}
  >
    <rect x={badge.x} y={badge.y} width={badge.w} height={badge.h} rx="14" />
    <text x={badge.x + badge.w / 2} y={badge.y + badge.h / 2} text-anchor="middle" dominant-baseline="central">
      {caption}
    </text>
  </g>
{/if}
