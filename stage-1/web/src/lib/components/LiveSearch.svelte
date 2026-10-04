<script lang="ts">
  import { untrack } from 'svelte';
  import { fade } from 'svelte/transition';
  import AvailabilityGrid from './AvailabilityGrid.svelte';
  import BookingForm from './BookingForm.svelte';
  import FloorPlan from './FloorPlan.svelte';
  import { rememberHold, rememberSearch, type HeldSelection } from '../hold';
  import { bookingSummary, formatLongDate, timezoneLabel } from '../format';
  import { motionDuration } from '../motion';
  import { go } from '../nav';
  import type { PreviewSlot } from '../preview';
  import {
    DEFAULT_SEARCH_DATE,
    errorText,
    loadSearch,
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
  }: {
    transport?: Transport;
    signedIn?: boolean;
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
  let generation = 0;

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
        <BookingForm {summary} bind:partySize={partyDraft} />
      {/if}
    {/if}
  {/if}
</section>
