<script lang="ts">
  import { cancelReservation, failureMessage, loadReservation, type ReservationRecord } from '../booking';
  import { bookingSummary } from '../format';
  import { detailPath, parseDetail, slotClock } from '../search';
  import type { Transport } from '../transport';

  interface ShownReservation {
    reference: string;
    status: 'confirmed' | 'cancelled';
    summary: string;
    tables: string;
  }

  let {
    error = null,
    reservation = null,
    transport = null,
    token = null,
  }: {
    error?: string | null;
    reservation?: ShownReservation | null;
    transport?: Transport | null;
    token?: string | null;
  } = $props();

  let referenceDraft = $state('');
  let localError = $state<string | null>(null);
  let localReservation = $state<ShownReservation | null>(null);
  let lookupSeq = 0;

  const live = $derived(transport !== null && transport !== undefined);
  const shownError = $derived(live ? localError : error);
  const shownReservation = $derived(live ? localReservation : reservation);

  $effect(() => {
    if (live && !token) localReservation = null;
  });

  function present(record: ReservationRecord, label: string, restaurantName: string): ShownReservation {
    const date = record.starts_at_local.slice(0, 10);
    const clock = slotClock(record.starts_at_local);
    return {
      reference: record.reference,
      status: record.status,
      summary: bookingSummary(restaurantName, label, date, clock),
      tables: `Table ${label}`,
    };
  }

  async function onSubmit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (!live || !transport) return;
    const reference = referenceDraft.trim();
    if (!token) {
      localError = 'Sign in to look up a reservation.';
      localReservation = null;
      return;
    }
    if (!reference) {
      localError = 'Enter a reservation reference.';
      localReservation = null;
      return;
    }
    lookupSeq += 1;
    const mine = lookupSeq;
    localError = null;
    try {
      const record = await loadReservation(transport, token, reference);
      if (mine !== lookupSeq) return;
      const detail = parseDetail(await transport(detailPath(record.restaurant_id)));
      if (mine !== lookupSeq) return;
      const table = detail.tables.find((item) => item.id === record.table_id);
      if (!table) throw new Error('That table is no longer listed for this restaurant.');
      localReservation = present(record, table.label, detail.name);
      localError = null;
    } catch (failure) {
      if (mine !== lookupSeq) return;
      localReservation = null;
      localError = failureMessage(failure, 'That reservation could not be looked up.');
    }
  }

  async function onCancel(): Promise<void> {
    if (!live || !transport || !token || !localReservation || localReservation.status !== 'confirmed') return;
    const reference = localReservation.reference;
    lookupSeq += 1;
    const mine = lookupSeq;
    try {
      const record = await cancelReservation(transport, token, reference);
      if (mine !== lookupSeq || !localReservation || localReservation.reference !== reference) return;
      localReservation = { ...localReservation, status: record.status, reference: record.reference };
      localError = null;
    } catch (failure) {
      if (mine !== lookupSeq) return;
      localError = failureMessage(failure, 'That cancellation could not be confirmed.');
    }
  }
</script>

<section class="route lookup-card">
  <p class="kicker">Already booked</p>
  <h1 class="display">Find a reservation</h1>
  <p class="lede">Enter the reference from your confirmation.</p>
  <form class="stack-form panel" onsubmit={onSubmit}>
    <div class="field">
      <label for="lookup-reference-input">Reservation reference</label>
      <input
        id="lookup-reference-input"
        data-testid="lookup-reference-input"
        name="reference"
        type="text"
        autocomplete="off"
        bind:value={referenceDraft}
      />
    </div>
    <button class="btn btn-primary" type="submit" data-testid="lookup-submit">Look up</button>
  </form>
  {#if shownError}
    <p class="state state-refused" data-testid="reservation-error" role="alert">{shownError}</p>
  {/if}
  {#if shownReservation}
    <article class="panel" data-testid="reservation-detail">
      <p data-testid="reservation-status">{shownReservation.status}</p>
      <p>{shownReservation.summary}</p>
      <p data-testid="reservation-tables">{shownReservation.tables}</p>
      {#if shownReservation.status === 'confirmed'}
        <button type="button" class="btn" data-testid="reservation-cancel-button" onclick={onCancel}>Cancel reservation</button>
      {/if}
    </article>
  {/if}
</section>
