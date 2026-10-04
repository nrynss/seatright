<script lang="ts">
  import { Spring } from 'svelte/motion';
  import { fly } from 'svelte/transition';
  import { motionDuration, prefersReducedMotion, springOptions } from '../motion';

  let {
    details,
    tables,
    reference = null,
  }: {
    details: string;
    tables: string;
    reference?: string | null;
  } = $props();

  const reduced = prefersReducedMotion();
  const seal = new Spring(reduced ? 1 : 0.86, springOptions());

  $effect(() => {
    void seal.set(1, { instant: reduced || motionDuration(1) === 0 });
  });
</script>

<section class="state state-confirmed" data-testid="confirmation" in:fly={{ y: 12, opacity: 1, duration: motionDuration(260) }}>
  <div class="confirmed-layout">
    <svg class="seal" viewBox="0 0 72 72" aria-hidden="true" style:transform={`scale(${seal.current})`}>
      <circle cx="36" cy="36" r="30" />
      <path d="M22 37 l10 10 l20 -22" />
    </svg>
    <div>
      <h2>Your table is confirmed</h2>
      <p data-testid="confirmation-details">{details}</p>
      <p data-testid="confirmation-tables">{tables}</p>
      {#if reference}
        <p class="reference" data-testid="confirmation-reference">{reference}</p>
      {/if}
    </div>
  </div>
</section>
