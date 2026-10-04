<script lang="ts">
  import { fly } from 'svelte/transition';
  import { motionDuration } from '../motion';

  let {
    summary,
    partySize = $bindable(),
    busy = false,
    heading = 'Hold this table',
    action = 'Request this table',
    onRequest = null,
  }: {
    summary: string;
    partySize: number;
    busy?: boolean;
    heading?: string;
    action?: string;
    onRequest?: ((partySize: number) => void) | null;
  } = $props();

  function onSubmit(event: SubmitEvent): void {
    event.preventDefault();
    onRequest?.(partySize);
  }
</script>

<form class="panel booking" data-testid="booking-form" onsubmit={onSubmit} in:fly={{ y: 10, opacity: 1, duration: motionDuration(220) }}>
  <h2>{heading}</h2>
  <p class="summary" data-testid="booking-summary">{summary}</p>
  <div class="field">
    <label for="booking-party-size">Party size</label>
    <input id="booking-party-size" data-testid="booking-party-size" type="number" min="1" step="1" bind:value={partySize} />
  </div>
  <button class="btn btn-primary" type="submit" data-testid="booking-submit" aria-busy={busy ? 'true' : 'false'}>
    {action}
  </button>
</form>
