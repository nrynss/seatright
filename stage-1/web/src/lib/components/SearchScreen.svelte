<script lang="ts">
  import { untrack } from 'svelte';
  import { fade } from 'svelte/transition';
  import AvailabilityGrid from './AvailabilityGrid.svelte';
  import BookingForm from './BookingForm.svelte';
  import Confirmation from './Confirmation.svelte';
  import FloorPlan from './FloorPlan.svelte';
  import { initialSelection, type Presentation } from '../demo';
  import { bookingSummary, formatLongDate, timezoneLabel } from '../format';
  import { motionDuration } from '../motion';
  import {
    previewFixture,
    slotByTime,
    tableById,
    tableIsAvailable,
    type PreviewFixture,
  } from '../preview';
  import { activeTime, chooseCell, type Selection } from '../selection';

  let {
    fixture = previewFixture,
    presentation = 'results' as Presentation,
  }: {
    fixture?: PreviewFixture;
    presentation?: Presentation;
  } = $props();

  let restaurantId = $state(untrack(() => fixture.restaurant.id));
  let dateValue = $state(untrack(() => fixture.date));
  let partyValue = $state(untrack(() => fixture.partySize));
  let partyDraft = $state(untrack(() => fixture.partySize));
  let selected = $state<Selection | null>(untrack(() => initialSelection(presentation, fixture)));

  const zone = $derived(
    fixture.slots[0]
      ? timezoneLabel(fixture.date, fixture.slots[0].time, fixture.restaurant.timezone)
      : fixture.restaurant.timezone,
  );
  const shownTime = $derived(activeTime(fixture.slots, selected));
  const activeSlot = $derived(shownTime ? slotByTime(fixture, shownTime) : undefined);
  const selectedTable = $derived(selected ? tableById(fixture, selected.tableId) : undefined);
  const summary = $derived(
    selected && selectedTable
      ? bookingSummary(fixture.restaurant.name, selectedTable.label, fixture.date, selected.time)
      : '',
  );
  const showFloor = $derived(presentation !== 'loading' && presentation !== 'empty');

  function onSearch(event: SubmitEvent): void {
    event.preventDefault();
  }

  function onGridSelect(tableId: string, time: string, available: boolean): void {
    const next = chooseCell(selected, tableId, time, available);
    if (next === selected) return;
    selected = next;
    if (next) partyDraft = partyValue;
  }

  function onPlanSelect(tableId: string): void {
    if (!activeSlot || !shownTime) return;
    onGridSelect(tableId, shownTime, tableIsAvailable(activeSlot, tableId));
  }
</script>

<section
  class="route"
  data-preview="sample"
  data-preview-restaurant={fixture.restaurant.id}
  data-preview-date={fixture.date}
  data-preview-party={fixture.partySize}
>
  <header>
    <p class="kicker">Sample seating</p>
    <h1 class="display">{fixture.restaurant.name}</h1>
    <p class="lede">
      {formatLongDate(fixture.date)} · party of {fixture.partySize} · {zone}
    </p>
  </header>
  <div class="rule" aria-hidden="true" in:fade={{ duration: motionDuration(280) }}></div>
  <form class="search-form" onsubmit={onSearch}>
    <div class="field">
      <label for="restaurant-select">Restaurant</label>
      <select id="restaurant-select" data-testid="restaurant-select" bind:value={restaurantId}>
        <option value={fixture.restaurant.id}>{fixture.restaurant.name}</option>
      </select>
    </div>
    <div class="field">
      <label for="date-input">Date</label>
      <input id="date-input" data-testid="date-input" type="date" bind:value={dateValue} />
    </div>
    <div class="field">
      <label for="party-size-input">Party size</label>
      <input id="party-size-input" data-testid="party-size-input" type="number" min="1" step="1" bind:value={partyValue} />
    </div>
    <button class="btn btn-primary" type="submit" data-testid="search-button">Search times</button>
  </form>

  {#if presentation === 'loading'}
    <div class="state state-loading" data-testid="loading-state" role="status" aria-busy="true">
      <h2>Looking up the floor</h2>
      <p>{fixture.restaurant.name}'s seating times are being gathered.</p>
      <div class="skeletons" aria-hidden="true">
        <div class="skeleton"></div>
        <div class="skeleton"></div>
        <div class="skeleton short"></div>
      </div>
    </div>
  {:else if presentation === 'empty'}
    <div class="state state-empty" data-testid="no-slots">
      <svg class="empty-drawing" viewBox="0 0 220 120" aria-hidden="true">
        <rect class="table-top" x="40" y="28" width="140" height="64" rx="18" />
      </svg>
      <h2>No seating times</h2>
      <p>{fixture.restaurant.name} has no times to offer for this search.</p>
    </div>
  {:else if shownTime && activeSlot}
    <div class="floor-and-grid">
      <FloorPlan
        tables={fixture.tables}
        availableIds={activeSlot.availableTableIds}
        selectedId={selected?.tableId ?? null}
        time={shownTime}
        onSelect={onPlanSelect}
      />
      <AvailabilityGrid
        tables={fixture.tables}
        slots={fixture.slots}
        date={fixture.date}
        selectedId={selected?.tableId ?? null}
        selectedTime={selected?.time ?? null}
        onSelect={onGridSelect}
      />
    </div>
    {#if selected && summary}
      <BookingForm {summary} bind:partySize={partyDraft} />
    {/if}
    {#if presentation === 'refused'}
      <p class="state state-refused" data-testid="booking-error" role="alert">
        That table is no longer free. The rest of your choices are still here.
      </p>
    {/if}
    {#if presentation === 'uncertain'}
      <p class="state state-uncertain" data-testid="booking-uncertain" role="status">
        We could not confirm that request. It may have reached the restaurant. The table, time and party size are unchanged.
      </p>
    {/if}
    {#if presentation === 'confirmed' && selectedTable && selected}
      <Confirmation
        details={summary}
        tables={`Table ${selectedTable.label}`}
      />
    {/if}
  {/if}
</section>
