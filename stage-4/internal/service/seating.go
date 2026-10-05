package service

// This file holds the exact stage-2 seating helpers. S2-B consumes them for
// pair CRUD, availability options and atomic moves; S2-M owns them here.
// Helpers are pure: no locking, no state mutation except through the
// explicitly passed reservation pointer in setReservationTables.

// SeatingOption is one bookable choice: a single table or a declared pair,
// with its summed fixture capacity.
type SeatingOption struct {
	TableIDs []string `json:"table_ids"`
	Capacity int      `json:"capacity"`
}

// reservationTableIDs returns the stored set: a fresh copy of TableIDs when
// present, else the legacy TableID singleton, else an empty set.
func reservationTableIDs(r Reservation) []string {
	if len(r.TableIDs) > 0 {
		return append([]string(nil), r.TableIDs...)
	}
	if r.TableID != "" {
		return []string{r.TableID}
	}
	return []string{}
}

// setReservationTables stores ids as a copied set; TableID mirrors the
// singleton member iff the set has exactly one member, else empty.
func setReservationTables(r *Reservation, ids []string) {
	r.TableIDs = append([]string(nil), ids...)
	if len(ids) == 1 {
		r.TableID = ids[0]
	} else {
		r.TableID = ""
	}
}

// canonicalPairOrder returns the declared order of the pair containing
// exactly a and b, or nil when no declared pair matches. Comparison is
// unordered: a reversed input names the same set.
func canonicalPairOrder(r *Restaurant, a, b string) []string {
	for _, p := range r.Combinable {
		if len(p) != 2 {
			continue
		}
		if (p[0] == a && p[1] == b) || (p[0] == b && p[1] == a) {
			return []string{p[0], p[1]}
		}
	}
	return nil
}

// parseTableSelection resolves table_id/table_ids request fields to a
// canonical stored set in declared combinable order. current is nil for
// creation (a selection is required) and non-nil for PATCH (absent fields
// retain current). It performs selection validation only: no time, party,
// occupancy or state mutation.
func parseTableSelection(r *Restaurant, obj map[string]any, current []string) ([]string, *codedError) {
	rawSingle, hasSingle := obj["table_id"]
	rawMulti, hasMulti := obj["table_ids"]
	if hasSingle && hasMulti {
		return nil, invalidErr("table_id and table_ids are mutually exclusive")
	}
	if !hasSingle && !hasMulti {
		if current == nil {
			return nil, invalidErr("table_id or table_ids is required")
		}
		return append([]string(nil), current...), nil
	}
	if hasSingle {
		id, ok := rawSingle.(string)
		if !ok {
			return nil, malformedErr()
		}
		if id == "" || len(id) > 64 {
			return nil, invalidErr("table_id must be 1..64 characters")
		}
		if !tableKnown(r, id) {
			return nil, &codedError{status: 404, code: "not_found", msg: "unknown table"}
		}
		return []string{id}, nil
	}
	list, ok := rawMulti.([]any)
	if !ok {
		return nil, malformedErr()
	}
	if len(list) == 0 {
		return nil, invalidErr("table_ids must not be empty")
	}
	if len(list) > 2 {
		return nil, &codedError{status: 422, code: "combination_not_allowed", msg: "at most two tables can be combined"}
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		id, ok := item.(string)
		if !ok {
			return nil, malformedErr()
		}
		if id == "" || len(id) > 64 {
			return nil, invalidErr("table_ids must be 1..64 characters")
		}
		ids = append(ids, id)
	}
	if len(ids) == 2 && ids[0] == ids[1] {
		return nil, invalidErr("duplicate table id")
	}
	for _, id := range ids {
		if !tableKnown(r, id) {
			return nil, &codedError{status: 404, code: "not_found", msg: "unknown table"}
		}
	}
	// Cross-restaurant membership is impossible here: ids come from one
	// restaurant's tables. Foreign ids are unknown ids (404 above).
	if len(ids) == 1 {
		return ids, nil
	}
	if ordered := canonicalPairOrder(r, ids[0], ids[1]); ordered != nil {
		return ordered, nil
	}
	return nil, &codedError{status: 422, code: "combination_not_allowed", msg: "tables cannot be combined"}
}

// seatingOptions returns every fixture single in order, then every declared
// pair in combinable order with summed fixture capacities. No party or
// occupancy filtering: S2-B applies those per slot.
func seatingOptions(r *Restaurant) []SeatingOption {
	out := make([]SeatingOption, 0, len(r.Tables)+len(r.Combinable))
	caps := map[string]int{}
	for _, t := range r.Tables {
		caps[t.ID] = t.Capacity
		out = append(out, SeatingOption{TableIDs: []string{t.ID}, Capacity: t.Capacity})
	}
	for _, p := range r.Combinable {
		if len(p) != 2 {
			continue
		}
		c1, ok1 := caps[p[0]]
		c2, ok2 := caps[p[1]]
		if !ok1 || !ok2 {
			continue
		}
		out = append(out, SeatingOption{TableIDs: []string{p[0], p[1]}, Capacity: c1 + c2})
	}
	return out
}

// tableSetsIntersect reports whether two table sets share any member.
func tableSetsIntersect(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

func tableKnown(r *Restaurant, id string) bool {
	for _, t := range r.Tables {
		if t.ID == id {
			return true
		}
	}
	return false
}
