<script lang="ts">
  import { untrack } from 'svelte';
  import { fade } from 'svelte/transition';
  import AvailabilityGrid from './AvailabilityGrid.svelte';
  import BookingForm from './BookingForm.svelte';
  import Confirmation from './Confirmation.svelte';
  import FloorPlan from './FloorPlan.svelte';
  import {
    attemptFor,
    newIdempotencyKey,
    sameBooking,
    submitBooking,
    UNCERTAIN_MESSAGE,
    type BookingAttempt,
    type BookingBody,
    type ReservationRecord,
  } from '../booking';
  import { rememberHold, rememberSearch, type HeldSelection } from '../hold';
  import { bookingSummary, formatLongDate, timezoneLabel } from '../format';
  import { motionDuration } from '../motion';
  import { go } from '../nav';
  import type { PreviewSlot } from '../preview';
  import {
    availabilityPath,
    DEFAULT_SEARCH_DATE,
    errorText,
    loadSearch,
    parseAvailability,
    parsePartySize,
    parseRestaurants,
    parseSearchDate,
    partyText,
    slotClock,
    type CommittedSearch,
    type RestaurantSummary,
    type SearchQuery,
  } from '../search';
  import { liveTransport, type Transport } from '../transport';

  let {
    transport = liveTransport,
    signedIn = false,
    token = null,
  }: {
    transport?: Transport;
    signedIn?: boolean;
    token?: string | null;
  } = $props();

  let restaurants = $state<RestaurantSummary[]>([]);
  let catalogPhase = $state<'loading' | 'ready' | 'error'>('loading');
  let catalogError = $state<string | null>(null);
  let catalogTick = $state(0);
  let restaurantId = $state('');
  let dateValue = $state(DEFAULT_SEARCH_DATE);
  let partyValue = $state<number | null>(2);
  let partyDraft = $state(2);

  let phase = $state<'idle' | 'loading' | 'ready' | 'error' | 'empty'>('idle');
  let result = $state<CommittedSearch | null>(null);
  let searchError = $state<string | null>(null);
  let authError = $state<string | null>(null);
  let held = $state<HeldSelection | null>(null);
  let pending = $state<BookingAttempt | null>(null);
  let receipt = $state<ReservationRecord | null>(null);
  let bookingError = $state<string | null>(null);
  let uncertain = $state(false);
  let sending = $state(false);
  let generation = 0;
  let attemptSeq = 0;
  let refreshSeq = 0;

  $effect(() => {
    const attempt = catalogTick;
    let alive = true;
    catalogPhase = 'loading';
    catalogError = null;
    void transport('/restaurants')
      .then((body) => {
        if (!alive || attempt !== catalogTick) return;
        const list = parseRestaurants(body);
        restaurants = list;
        if (!untrack(() => restaurantId) && list[0]) restaurantId = list[0].id;
        catalogPhase = 'ready';
      })
      .catch((error: unknown) => {
        if (!alive || attempt !== catalogTick) return;
        catalogError = errorText(error);
        catalogPhase = 'error';
      });
    return () => {
      alive = false;
    };
  });

  const shownSlots = $derived.by((): PreviewSlot[] => {
    if (!result) return [];
    return result.availability.slots.map((slot) => ({
      time: slotClock(slot.starts_at_local),
      availableTableIds: slot.availableTableIds,
    }));
  });

  const zone = $derived.by(() => {
    if (!result) return '';
    const clock = shownSlots[0]?.time ?? '12:00';
    return timezoneLabel(result.query.date, clock, result.detail.timezone);
  });

  const shownTime = $derived(held ? slotClock(held.starts_at_local) : (shownSlots[0]?.time ?? ''));
  const activeSlot = $derived(shownSlots.find((slot) => slot.time === shownTime));
  const summary = $derived(
    held
      ? bookingSummary(
          held.display.restaurantName,
          held.display.tableLabel,
          held.starts_at_local.slice(0, 10),
          slotClock(held.starts_at_local),
        )
      : '',
  );
  const anyAvailable = $derived(shownSlots.some((slot) => slot.availableTableIds.length > 0));

  function retryCatalog(): void {
    catalogTick += 1;
  }

  function signIn(event: MouseEvent): void {
    event.preventDefault();
    go('/login');
  }

  async function onSearch(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    authError = null;
    let query: SearchQuery;
    try {
      query = {
        restaurantId,
        date: parseSearchDate(dateValue),
        partySize: parsePartySize(partyText(partyValue)),
      };
    } catch (error) {
      searchError = errorText(error);
      phase = 'error';
      return;
    }
    if (!query.restaurantId) {
      searchError = 'Choose a restaurant.';
      phase = 'error';
      return;
    }
    generation += 1;
    const mine = generation;
    held = null;
    rememberHold(null);
    retireAttempt();
    result = null;
    rememberSearch(null);
    searchError = null;
    phase = 'loading';
    const loaded = await loadSearch(transport, query, () => mine === generation);
    if (mine !== generation || loaded.status === 'stale') return;
    if (loaded.status === 'error') {
      searchError = errorText(loaded.error);
      phase = 'error';
      return;
    }
    result = loaded.search;
    rememberSearch(loaded.search);
    phase = loaded.search.availability.slots.length === 0 ? 'empty' : 'ready';
  }

  function selectCell(tableId: string, time: string, available: boolean): void {
    if (!available || !result) return;
    const slot = result.availability.slots.find((item) => slotClock(item.starts_at_local) === time);
    if (!slot || !slot.availableTableIds.includes(tableId)) return;
    if (!signedIn) {
      authError = 'Sign in to hold a table.';
      return;
    }
    const table = result.detail.tables.find((item) => item.id === tableId);
    if (!table) return;
    if (held && held.table_id === tableId && held.starts_at_local === slot.starts_at_local) return;
    retireAttempt();
    const next: HeldSelection = {
      restaurant_id: result.query.restaurantId,
      table_id: tableId,
      starts_at_local: slot.starts_at_local,
      party_size: result.query.partySize,
      display: {
        restaurantName: result.detail.name,
        tableLabel: table.label,
        timezone: result.detail.timezone,
      },
    };
    held = next;
    rememberHold(next);
    partyDraft = result.query.partySize;
    authError = null;
  }

  function onPlanSelect(tableId: string): void {
    if (!activeSlot || !shownTime) return;
    selectCell(tableId, shownTime, activeSlot.availableTableIds.includes(tableId));
  }

  function draftBody(size: number): BookingBody | null {
    if (!held) return null;
    try {
      return {
        restaurant_id: held.restaurant_id,
        table_id: held.table_id,
        starts_at_local: held.starts_at_local,
        party_size: parsePartySize(partyText(size)),
      };
    } catch {
      return null;
    }
  }

  const typedBody = $derived(draftBody(partyDraft));
  const attemptMatchesForm = $derived(Boolean(pending && typedBody && sameBooking(pending.body, typedBody)));
  const confirmedAttempt = $derived(
    Boolean(
      receipt &&
        pending &&
        held &&
        sameBooking(pending.body, {
          restaurant_id: receipt.restaurant_id,
          table_id: receipt.table_id,
          starts_at_local: receipt.starts_at_local,
          party_size: receipt.party_size,
        }) &&
        held.table_id === receipt.table_id &&
        held.starts_at_local === receipt.starts_at_local,
    ),
  );

  function retireAttempt(): void {
    attemptSeq += 1;
    pending = null;
    receipt = null;
    bookingError = null;
    uncertain = false;
    sending = false;
  }

  async function refreshAvailability(searchGeneration: number, query: SearchQuery): Promise<void> {
    refreshSeq += 1;
    const mine = refreshSeq;
    try {
      const body = await transport(availabilityPath(query));
      if (mine !== refreshSeq || searchGeneration !== generation || !result) return;
      if (
        result.query.restaurantId !== query.restaurantId ||
        result.query.date !== query.date ||
        result.query.partySize !== query.partySize
      ) {
        return;
      }
      const availability = parseAvailability(body);
      if (mine !== refreshSeq || searchGeneration !== generation || !result) return;
      result = { ...result, availability, availabilityBody: body };
      rememberSearch(result);
    } catch {
      // The refusal is already on screen. A failed refresh leaves the previous grid in place.
    }
  }

  async function onBook(size: number): Promise<void> {
    partyDraft = size;
    if (!held || !signedIn || !token) {
      bookingError = 'Sign in to request this table.';
      uncertain = false;
      return;
    }
    const body = draftBody(size);
    if (!body) {
      bookingError = 'Party size must be a whole number.';
      return;
    }
    const next = attemptFor(pending, body, newIdempotencyKey());
    const replaced = !pending || next.key !== pending.key;
    pending = next;
    if (replaced) {
      receipt = null;
      uncertain = false;
    }
    bookingError = null;
    attemptSeq += 1;
    const mine = attemptSeq;
    const searchGeneration = generation;
    const query = result?.query;
    sending = true;
    const outcome = await submitBooking(transport, token, next);
    if (mine !== attemptSeq) return;
    sending = false;
    if (outcome.kind === 'confirmed') {
      receipt = outcome.reservation;
      bookingError = null;
      uncertain = false;
      return;
    }
    if (outcome.kind === 'uncertain') {
      bookingError = null;
      if (!receipt) uncertain = true;
      return;
    }
    uncertain = false;
    bookingError = outcome.message;
    receipt = null;
    if (outcome.code === 'table_unavailable' && query) {
      void refreshAvailability(searchGeneration, query);
    }
  }
</script>

<section class="route">
  <header>
    {#if result}
      <p class="kicker">Seating</p>
      <h1 class="display">{result.detail.name}</h1>
      <p class="lede">
        {formatLongDate(result.query.date)} · party of {result.query.partySize} · {zone}
      </p>
    {:else}
      <p class="kicker">Reservations</p>
      <h1 class="display">Find a table</h1>
      <p class="lede">Choose a restaurant, a date and a party size. The floor follows the search you submit.</p>
    {/if}
  </header>
  <div class="rule" aria-hidden="true" in:fade={{ duration: motionDuration(280) }}></div>

  {#if catalogPhase === 'loading'}
    <div class="state state-loading" data-testid="loading-state" role="status" aria-busy="true">
      <h2>Opening the book</h2>
      <p>Restaurant names are on their way.</p>
      <div class="skeletons" aria-hidden="true">
        <div class="skeleton"></div>
        <div class="skeleton"></div>
        <div class="skeleton short"></div>
      </div>
    </div>
  {:else if catalogPhase === 'error'}
    <div class="state state-refused" data-testid="search-error" role="alert">
      <h2>The restaurant list did not load</h2>
      <p>{catalogError}</p>
      <button class="btn" type="button" data-testid="catalog-retry" onclick={retryCatalog}>Try again</button>
    </div>
  {:else}
    {#if restaurants.length === 0}
      <div class="state state-empty" data-testid="no-restaurants">
        <h2>No restaurants</h2>
        <p>There are no restaurants to search yet.</p>
      </div>
    {/if}
    <form class="search-form" onsubmit={onSearch}>
      <div class="field">
        <label for="restaurant-select">Restaurant</label>
        <select id="restaurant-select" data-testid="restaurant-select" bind:value={restaurantId}>
          {#each restaurants as restaurant (restaurant.id)}
            <option value={restaurant.id}>{restaurant.name}</option>
          {/each}
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

    {#if authError}
      <p class="state state-refused" data-testid="auth-error" role="alert">
        {authError}
        <a href="/login" onclick={signIn}>Sign in</a>
      </p>
    {/if}

    {#if phase === 'loading'}
      <div class="state state-loading" data-testid="loading-state" role="status" aria-busy="true">
        <h2>Looking up the floor</h2>
        <p>Seating times for this search are being gathered.</p>
        <div class="skeletons" aria-hidden="true">
          <div class="skeleton"></div>
          <div class="skeleton"></div>
          <div class="skeleton short"></div>
        </div>
      </div>
    {:else if phase === 'error' && searchError}
      <div class="state state-refused" data-testid="search-error" role="alert">
        <h2>That search did not finish</h2>
        <p>{searchError}</p>
      </div>
    {:else if phase === 'empty'}
      <div class="state state-empty" data-testid="no-slots">
        <svg class="empty-drawing" viewBox="0 0 220 120" aria-hidden="true">
          <rect class="table-top" x="40" y="28" width="140" height="64" rx="18" />
        </svg>
        <h2>No seating times</h2>
        <p>
          {result ? `${result.detail.name} has no seating times on ${formatLongDate(result.query.date)}.` : 'This day is closed.'}
        </p>
      </div>
    {:else if phase === 'ready' && result && shownTime && activeSlot}
      <div class="floor-and-grid">
        <FloorPlan
          tables={result.detail.tables}
          availableIds={activeSlot.availableTableIds}
          selectedId={held?.table_id ?? null}
          time={shownTime}
          onSelect={onPlanSelect}
        />
        <AvailabilityGrid
          tables={result.detail.tables}
          slots={shownSlots}
          date={result.query.date}
          selectedId={held?.table_id ?? null}
          selectedTime={held ? slotClock(held.starts_at_local) : null}
          onSelect={selectCell}
        />
      </div>
      {#if !anyAvailable}
        <p class="state state-empty" data-testid="zero-available">
          No table fits this party. The times stay on the floor so you can see what is taken.
        </p>
      {/if}
      {#if held && summary}
        <BookingForm {summary} bind:partySize={partyDraft} busy={sending} onRequest={(size) => void onBook(size)} />
        {#if sending}
          <p class="booking-note" role="status">Sending your request.</p>
        {/if}
        {#if uncertain && attemptMatchesForm}
          <p class="state state-uncertain" data-testid="booking-uncertain" role="status">{UNCERTAIN_MESSAGE}</p>
        {/if}
        {#if bookingError}
          <p class="state state-refused" data-testid="booking-error" role="alert">{bookingError}</p>
        {/if}
        {#if confirmedAttempt && receipt && held}
          <Confirmation
            details={bookingSummary(
              held.display.restaurantName,
              held.display.tableLabel,
              receipt.starts_at_local.slice(0, 10),
              slotClock(receipt.starts_at_local),
            )}
            tables={`Table ${held.display.tableLabel}`}
            reference={receipt.reference}
          />
        {/if}
      {/if}
    {/if}
  {/if}
</section>
