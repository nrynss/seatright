<script lang="ts">
  interface ShownReservation {
    reference: string;
    status: 'confirmed' | 'cancelled';
    summary: string;
    tables: string;
  }

  let {
    error = null,
    reservation = null,
  }: {
    error?: string | null;
    reservation?: ShownReservation | null;
  } = $props();

  function onSubmit(event: SubmitEvent): void {
    event.preventDefault();
  }
</script>

<section class="route lookup-card">
  <p class="kicker">Already booked</p>
  <h1 class="display">Find a reservation</h1>
  <p class="lede">Enter the reference from your confirmation.</p>
  <form class="stack-form panel" onsubmit={onSubmit}>
    <div class="field">
      <label for="lookup-reference-input">Reservation reference</label>
      <input id="lookup-reference-input" data-testid="lookup-reference-input" name="reference" type="text" autocomplete="off" />
    </div>
    <button class="btn btn-primary" type="submit" data-testid="lookup-submit">Look up</button>
  </form>
  {#if error}
    <p class="state state-refused" data-testid="reservation-error" role="alert">{error}</p>
  {/if}
  {#if reservation}
    <article class="panel" data-testid="reservation-detail">
      <p data-testid="reservation-status">{reservation.status}</p>
      <p>{reservation.summary}</p>
      <p data-testid="reservation-tables">{reservation.tables}</p>
      {#if reservation.status === 'confirmed'}
        <button type="button" class="btn" data-testid="reservation-cancel-button">Cancel reservation</button>
      {/if}
    </article>
  {/if}
</section>
