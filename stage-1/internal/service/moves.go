package service

// This file implements the stage-1 atomic move batch. moveReservationsLocked
// is the idempotent-write callback for POST /reservation-moves: it validates
// the whole body shape first, then checks each booking in input order with
// the ordinary ownedReservation/prepareAmendment helpers, then validates the
// final occupancy of the whole resulting set (so legal swaps pass), and
// applies every change to the passed working state together. It locks nothing
// itself. Identity, ownership and creation time never change; no-op listed
// bookings keep their values and occupancy.

// moveReservationsLocked executes an atomic batch on the working state st.
func (s *Service) moveReservationsLocked(st *State, userID string, obj map[string]any) Result {
	items, cerr := parseMovesBody(obj)
	if cerr != nil {
		return cerr.Result()
	}
	prepared := make([]Reservation, 0, len(items))
	restaurantID := ""
	for _, item := range items {
		current, cerr := ownedReservation(st, userID, item.reference)
		if cerr != nil {
			return cerr.Result()
		}
		if restaurantID == "" {
			restaurantID = current.RestaurantID
		} else if current.RestaurantID != restaurantID {
			return invalidErr("moves must belong to one restaurant").Result()
		}
		candidate, cerr := prepareAmendment(st, current, item.changes)
		if cerr != nil {
			return cerr.Result()
		}
		prepared = append(prepared, candidate)
	}
	// Publish candidates into the working copy, then check each result
	// against everything else: unlisted bookings and the other results.
	// Old assignments are gone, so legal swaps validate cleanly.
	for _, cand := range prepared {
		st.Reservations[cand.Reference] = cand
	}
	for _, cand := range prepared {
		// Exclude only the candidate itself: every other listed result and
		// every unlisted booking still blocks. On conflict the wrapper
		// discards the whole working state, so nothing is committed.
		if conflictingReservation(st, cand, map[string]bool{cand.Reference: true}) {
			return conflict("table_unavailable", "resulting bookings overlap")
		}
	}
	itemsOut := make([]any, 0, len(prepared))
	for _, cand := range prepared {
		itemsOut = append(itemsOut, cand.Public())
	}
	return created(map[string]any{"reservations": itemsOut})
}

// moveItem is one parsed batch entry: its booking reference and the raw
// PATCH-style change fields.
type moveItem struct {
	reference string
	changes   map[string]any
}

// parseMovesBody validates the batch shape: moves is 1..8 objects with
// distinct string references. Every shape violation is 422
// validation_failed.
func parseMovesBody(obj map[string]any) ([]moveItem, *codedError) {
	raw, ok := obj["moves"]
	if !ok {
		return nil, invalidErr("moves is required")
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, invalidErr("moves must be an array")
	}
	if len(list) < 1 || len(list) > 8 {
		return nil, invalidErr("moves must contain 1..8 items")
	}
	items := make([]moveItem, 0, len(list))
	seen := map[string]bool{}
	for _, entry := range list {
		item, ok := entry.(map[string]any)
		if !ok {
			return nil, invalidErr("every move must be an object")
		}
		refRaw, ok := item["reference"]
		if !ok {
			return nil, invalidErr("every move needs a string reference")
		}
		ref, ok := refRaw.(string)
		if !ok || ref == "" {
			return nil, invalidErr("every move needs a string reference")
		}
		if seen[ref] {
			return nil, invalidErr("duplicate move reference")
		}
		seen[ref] = true
		changes := map[string]any{}
		for _, field := range []string{"table_id", "starts_at_local", "party_size"} {
			if v, ok := item[field]; ok {
				changes[field] = v
			}
		}
		items = append(items, moveItem{reference: ref, changes: changes})
	}
	return items, nil
}
