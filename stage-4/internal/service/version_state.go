package service

import (
	"encoding/json"
	"errors"

	"tablekeeper/internal/history"
	"tablekeeper/internal/policy"
)

// This file owns the shared stage-3 version/state foundation: fixture policy0
// terms, reservation history snapshots, legacy metadata initialization and
// offside state normalization. It performs no booking business validation,
// no locking and no idempotent writes.

// fixtureTerms builds the fixture policy0 terms for a restaurant: version 0,
// the original configuration and every original table capacity. Publication
// maxima never apply to fixture terms.
func fixtureTerms(r Restaurant) policy.Terms {
	caps := make(map[string]int, len(r.Tables))
	for _, t := range r.Tables {
		caps[t.ID] = t.Capacity
	}
	hours := make([]policy.Hours, 0, len(r.OpeningHours))
	for _, h := range r.OpeningHours {
		hours = append(hours, policy.Hours{Weekday: h.Weekday, Opens: h.Opens, Closes: h.Closes})
	}
	return policy.Terms{
		PolicyVersion:              0,
		SlotMinutes:                r.SlotMinutes,
		ReservationDurationMinutes: r.ReservationDurationMinutes,
		CancellationCutoffMinutes:  r.CancellationCutoffMinutes,
		OpeningHours:               hours,
		Capacities:                 caps,
	}
}

// reservationSnapshot builds the history snapshot for a reservation: the
// canonical table ids, current time/party/revision and freshly marshalled
// complete terms.
func reservationSnapshot(res Reservation) history.Snapshot {
	terms := policy.CloneTerms(res.AcceptedTerms)
	raw, err := json.Marshal(termsToPublic(terms))
	if err != nil {
		raw = json.RawMessage("{}")
	}
	return history.Snapshot{
		TableIDs:      append([]string{}, reservationTableIDs(res)...),
		StartsAtLocal: res.StartsAtLocal,
		PartySize:     res.PartySize,
		Revision:      res.Revision,
		AcceptedTerms: raw,
	}
}

// initializeReservationMetadata fills missing legacy metadata on a stored
// record only: revision 1, fixture0 terms and one reconstructed created
// history entry at the retained created_at instant. Records that already
// carry revision metadata are left untouched. Counters are never incremented
// here. A cancelled legacy record keeps status cancelled with the same
// single reconstructed created entry: the historical cancellation time is
// unavailable, so no cancelled entry is manufactured.
func initializeReservationMetadata(st *State, res *Reservation) error {
	if res.Revision != 0 {
		return nil
	}
	var restaurant *Restaurant
	for i := range st.Restaurants {
		if st.Restaurants[i].ID == res.RestaurantID {
			restaurant = &st.Restaurants[i]
			break
		}
	}
	if restaurant == nil {
		return errors.New("unknown restaurant")
	}
	res.Revision = 1
	res.AcceptedTerms = policy.CloneTerms(fixtureTerms(*restaurant))
	snap := reservationSnapshot(*res)
	st.Histories[res.Reference] = []history.Entry{history.Created(snap, res.CreatedAt)}
	return nil
}

// normalizeVersionState fills offside absent legacy state before validation:
// missing maps, per-restaurant manager defaults, revision counters and
// missing booking metadata. Existing modern revisions, terms, histories,
// policies, series and counters stay verbatim. Obvious negative
// revisions/counters and incomplete non-legacy metadata are rejected rather
// than silently repaired; exhaustive modern history validation stays with
// S3-I.
func normalizeVersionState(st *State) error {
	if st.Policies == nil {
		st.Policies = map[string][]policy.Policy{}
	}
	if st.Histories == nil {
		st.Histories = map[string][]history.Entry{}
	}
	if st.Series == nil {
		st.Series = map[string]Series{}
	}
	if st.RestaurantRevisions == nil {
		st.RestaurantRevisions = map[string]int{}
	}
	normalizeReplanState(st)
	for i := range st.Restaurants {
		if st.Restaurants[i].ManagerUserIDs == nil {
			st.Restaurants[i].ManagerUserIDs = []string{}
		}
		if _, ok := st.RestaurantRevisions[st.Restaurants[i].ID]; !ok {
			st.RestaurantRevisions[st.Restaurants[i].ID] = 0
		} else if st.RestaurantRevisions[st.Restaurants[i].ID] < 0 {
			return errors.New("negative restaurant revision")
		}
	}
	for ref, res := range st.Reservations {
		if res.Revision < 0 {
			return errors.New("negative reservation revision")
		}
		if res.Revision == 0 {
			// Revision zero must be genuine absent older-stage metadata:
			// it must not erase modern terms or history already present.
			if !isZeroTerms(res.AcceptedTerms) {
				return errors.New("revision zero with modern terms")
			}
			if h, ok := st.Histories[ref]; ok && len(h) != 0 {
				return errors.New("revision zero with modern history")
			}
			if err := initializeReservationMetadata(st, &res); err != nil {
				return err
			}
			st.Reservations[ref] = res
			continue
		}
		// Positive revision requires usable complete terms (fixture or
		// published shape: positive slot/duration, nonnegative cutoff and
		// capacities for every table of the reservation's restaurant) and a
		// nonempty retained history. Missing/empty/unusable terms can no
		// longer hide behind the fixture-duration fallback.
		if !usableTerms(st, res) {
			return errors.New("incomplete reservation terms")
		}
		if len(st.Histories[ref]) == 0 {
			return errors.New("missing reservation history")
		}
	}
	return nil
}

// isZeroTerms reports whether terms carry no modern content: zero numerics
// with nil/empty hours and capacities.
func isZeroTerms(t policy.Terms) bool {
	return t.PolicyVersion == 0 && t.SlotMinutes == 0 &&
		t.ReservationDurationMinutes == 0 && t.CancellationCutoffMinutes == 0 &&
		len(t.OpeningHours) == 0 && len(t.Capacities) == 0
}

// usableTerms reports whether the record's accepted terms are complete enough
// to validate and time the booking: positive slot/duration, nonnegative
// cutoff and capacities naming every table of its restaurant.
func usableTerms(st *State, res Reservation) bool {
	t := res.AcceptedTerms
	if t.SlotMinutes <= 0 || t.ReservationDurationMinutes <= 0 || t.CancellationCutoffMinutes < 0 {
		return false
	}
	if t.Capacities == nil {
		return false
	}
	for i := range st.Restaurants {
		if st.Restaurants[i].ID != res.RestaurantID {
			continue
		}
		for _, tb := range st.Restaurants[i].Tables {
			if _, ok := t.Capacities[tb.ID]; !ok {
				return false
			}
		}
		return true
	}
	return false
}
