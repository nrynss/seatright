<script lang="ts">
  import { fly } from 'svelte/transition';
  import { motionDuration } from '../motion';

  let {
    summary,
    partySize = $bindable(),
  }: {
    summary: string;
    partySize: number;
  } = $props();

  function onSubmit(event: SubmitEvent): void {
    event.preventDefault();
  }
</script>

<form class="panel booking" data-testid="booking-form" onsubmit={onSubmit} in:fly={{ y: 10, opacity: 1, duration: motionDuration(220) }}>
  <h2>Hold this table</h2>
  <p class="summary" data-testid="booking-summary">{summary}</p>
  <p class="booking-note">Booking submission is pending.</p>
  <div class="field">
    <label for="booking-party-size">Party size</label>
    <input id="booking-party-size" data-testid="booking-party-size" type="number" min="1" step="1" bind:value={partySize} />
  </div>
  <button class="btn btn-primary" type="submit" data-testid="booking-submit">Request this table</button>
</form>
