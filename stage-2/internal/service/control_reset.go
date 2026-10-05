package service

import (
	"time"

	"tablekeeper/internal/clock"
)

// Reset atomically replaces all service state with the fixture in the request
// body. The replacement is built off to the side (including password hashing)
// and published under the state lock, so readers never observe a half-reset.
func (s *Service) Reset(raw []byte) Result {
	obj, _, err := ParseBody(raw)
	if err != nil {
		return malformed()
	}
	next := emptyState()
	if err := applyFixtureUsers(obj, &next); err != nil {
		return err.Result()
	}
	restaurants, rerr := parseFixtureRestaurants(obj)
	if rerr != nil {
		return rerr.Result()
	}
	next.Restaurants = restaurants
	if err := applyFixtureReservations(obj, &next); err != nil {
		return err.Result()
	}
	// Hash fixture passwords before taking the lock.
	hashes := make(map[string]string, len(next.Users))
	for id, u := range next.Users {
		sum, herr := hashPassword(u.PasswordHash)
		if herr != nil {
			return internalErr()
		}
		hashes[id] = sum
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, h := range hashes {
		u := next.Users[id]
		u.PasswordHash = h
		next.Users[id] = u
	}
	s.state = next
	return noContent()
}

// codedError is a validation failure that already knows its HTTP mapping.
type codedError struct {
	status int
	code   string
	msg    string
}

func (e *codedError) Result() Result {
	return Result{Status: e.status, Body: errorBody(e.code, e.msg)}
}

func malformedErr() *codedError {
	return &codedError{status: 400, code: "malformed_request", msg: "unparseable body or wrong field type"}
}

func invalidErr(msg string) *codedError {
	return &codedError{status: 422, code: "validation_failed", msg: msg}
}

// fixtureArray fetches an object field that must hold an array of objects.
// A missing field defaults to empty; a wrong type is malformed.
func fixtureArray(obj map[string]any, name string) ([]map[string]any, *codedError) {
	raw, ok := obj[name]
	if !ok {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, malformedErr()
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, malformedErr()
		}
		out = append(out, entry)
	}
	return out, nil
}

// validReference enforces the confirmation-reference shape from §8: 6 to 12
// characters of A-Z0-9. Seeded references must already have this shape.
func validReference(ref string) bool {
	if len(ref) < 6 || len(ref) > 12 {
		return false
	}
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// checkID enforces the opaque-ID contract on fixture ids: nonempty strings
// of at most 64 characters.
func checkID(v any) (string, *codedError) {
	s, ok := v.(string)
	if !ok {
		return "", malformedErr()
	}
	if s == "" || len(s) > 64 {
		return "", invalidErr("id must be 1..64 characters")
	}
	return s, nil
}

func applyFixtureUsers(obj map[string]any, next *State) *codedError {
	entries, cerr := fixtureArray(obj, "users")
	if cerr != nil {
		return cerr
	}
	for _, e := range entries {
		idRaw, ok := e["id"]
		if !ok {
			return invalidErr("user id is required")
		}
		id, cerr := checkID(idRaw)
		if cerr != nil {
			return cerr
		}
		email, present, wrongType := fieldString(e, "email")
		password, presentPw, wrongTypePw := fieldString(e, "password")
		display, presentDisplay, wrongTypeDisplay := fieldString(e, "display_name")
		if wrongType || wrongTypePw || wrongTypeDisplay {
			return malformedErr()
		}
		if !present || !presentPw || !presentDisplay {
			return invalidErr("user email, password and display_name are required")
		}
		// PasswordHash temporarily carries the plaintext until Reset hashes
		// it before publishing; it never leaves this function unhashed.
		next.Users[id] = User{ID: id, Email: email, DisplayName: display, PasswordHash: password}
	}
	return nil
}

func parseFixtureRestaurants(obj map[string]any) ([]Restaurant, *codedError) {
	entries, cerr := fixtureArray(obj, "restaurants")
	if cerr != nil {
		return nil, cerr
	}
	out := make([]Restaurant, 0, len(entries))
	for _, e := range entries {
		r, cerr := parseFixtureRestaurant(e)
		if cerr != nil {
			return nil, cerr
		}
		out = append(out, r)
	}
	return out, nil
}

func parseFixtureRestaurant(e map[string]any) (Restaurant, *codedError) {
	var r Restaurant
	idRaw, ok := e["id"]
	if !ok {
		return r, invalidErr("restaurant id is required")
	}
	id, cerr := checkID(idRaw)
	if cerr != nil {
		return r, cerr
	}
	name, present, wrongType := fieldString(e, "name")
	tz, presentTz, wrongTypeTz := fieldString(e, "timezone")
	if wrongType || wrongTypeTz {
		return r, malformedErr()
	}
	if !present || !presentTz {
		return r, invalidErr("restaurant name and timezone are required")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return r, invalidErr("unknown timezone")
	}
	slot, cerr := fixturePositiveInt(e, "slot_minutes", 1)
	if cerr != nil {
		return r, cerr
	}
	duration, cerr := fixturePositiveInt(e, "reservation_duration_minutes", 1)
	if cerr != nil {
		return r, cerr
	}
	cutoff, cerr := fixtureCutoff(e)
	if cerr != nil {
		return r, cerr
	}
	hours, cerr := parseFixtureHours(e)
	if cerr != nil {
		return r, cerr
	}
	tables, cerr := parseFixtureTables(e)
	if cerr != nil {
		return r, cerr
	}
	combinable, cerr := parseFixtureCombinable(e, tables)
	if cerr != nil {
		return r, cerr
	}
	r = Restaurant{
		ID:                         id,
		Name:                       name,
		Timezone:                   tz,
		SlotMinutes:                slot,
		ReservationDurationMinutes: duration,
		CancellationCutoffMinutes:  cutoff,
		OpeningHours:               hours,
		Tables:                     tables,
		Combinable:                 combinable,
	}
	return r, nil
}

// parseFixtureCombinable parses the optional combinable fixture field:
// absent means no pairs. Each entry must be an array of exactly two known
// distinct table ids of this restaurant; declared order is retained.
func parseFixtureCombinable(e map[string]any, tables []Table) ([][]string, *codedError) {
	raw, ok := e["combinable"]
	if !ok {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, malformedErr()
	}
	known := map[string]bool{}
	for _, t := range tables {
		known[t.ID] = true
	}
	out := make([][]string, 0, len(list))
	for _, item := range list {
		pair, ok := item.([]any)
		if !ok || len(pair) != 2 {
			return nil, invalidErr("combinable entries must be pairs of table ids")
		}
		a, okA := pair[0].(string)
		b, okB := pair[1].(string)
		if !okA || !okB {
			return nil, malformedErr()
		}
		if a == "" || b == "" || a == b || !known[a] || !known[b] {
			return nil, invalidErr("combinable entries must be two known distinct tables")
		}
		out = append(out, []string{a, b})
	}
	if out == nil {
		out = [][]string{}
	}
	return out, nil
}

// fixturePositiveInt reads a required integer field with a minimum value.
func fixturePositiveInt(e map[string]any, name string, min int) (int, *codedError) {
	raw, ok := e[name]
	if !ok {
		return 0, invalidErr(name + " is required")
	}
	n, isNumber, isInteger := fixtureInt(raw)
	if !isNumber {
		return 0, malformedErr()
	}
	if !isInteger || n < min {
		return 0, invalidErr(name + " is out of range")
	}
	return n, nil
}

func fixtureCutoff(e map[string]any) (int, *codedError) {
	return fixturePositiveInt(e, "cancellation_cutoff_minutes", 0)
}

func parseFixtureHours(e map[string]any) ([]OpeningHour, *codedError) {
	entries, cerr := fixtureArray(e, "opening_hours")
	if cerr != nil {
		return nil, cerr
	}
	out := make([]OpeningHour, 0, len(entries))
	for _, h := range entries {
		weekday, present, wrongType := fieldString(h, "weekday")
		opens, presentOpens, wrongTypeOpens := fieldString(h, "opens")
		closes, presentCloses, wrongTypeCloses := fieldString(h, "closes")
		if wrongType || wrongTypeOpens || wrongTypeCloses {
			return nil, malformedErr()
		}
		if !present || !presentOpens || !presentCloses {
			return nil, invalidErr("opening hours need weekday, opens and closes")
		}
		if !weekdays[weekday] {
			return nil, invalidErr("unknown weekday")
		}
		oh, omin, err := parseHHMM(opens)
		if err != nil {
			return nil, invalidErr("invalid opens time")
		}
		ch, cmin, err := parseHHMM(closes)
		if err != nil {
			return nil, invalidErr("invalid closes time")
		}
		if ch*60+cmin <= oh*60+omin {
			return nil, invalidErr("closes must be later than opens on the same day")
		}
		out = append(out, OpeningHour{Weekday: weekday, Opens: opens, Closes: closes})
	}
	if out == nil {
		out = []OpeningHour{}
	}
	return out, nil
}

func parseFixtureTables(e map[string]any) ([]Table, *codedError) {
	entries, cerr := fixtureArray(e, "tables")
	if cerr != nil {
		return nil, cerr
	}
	out := make([]Table, 0, len(entries))
	for _, t := range entries {
		idRaw, ok := t["id"]
		if !ok {
			return nil, invalidErr("table id is required")
		}
		id, cerr := checkID(idRaw)
		if cerr != nil {
			return nil, cerr
		}
		label, present, wrongType := fieldString(t, "label")
		if wrongType {
			return nil, malformedErr()
		}
		if !present {
			return nil, invalidErr("table label is required")
		}
		capacity, cerr := fixturePositiveInt(t, "capacity", 1)
		if cerr != nil {
			return nil, cerr
		}
		out = append(out, Table{ID: id, Label: label, Capacity: capacity})
	}
	if out == nil {
		out = []Table{}
	}
	return out, nil
}

func applyFixtureReservations(obj map[string]any, next *State) *codedError {
	entries, cerr := fixtureArray(obj, "reservations")
	if cerr != nil {
		return cerr
	}
	for _, e := range entries {
		idRaw, ok := e["id"]
		if !ok {
			idRaw, ok = e["reservation_id"]
		}
		if !ok {
			return invalidErr("seeded reservation id is required")
		}
		id, cerr := checkID(idRaw)
		if cerr != nil {
			return cerr
		}
		reference, present, wrongType := fieldString(e, "reference")
		if wrongType {
			return malformedErr()
		}
		if !present {
			return invalidErr("seeded reservation reference is required")
		}
		if !validReference(reference) {
			return invalidErr("seeded reservation reference must be 6..12 characters of A-Z0-9")
		}
		owner, present, wrongType := fieldString(e, "user_id")
		if wrongType {
			return malformedErr()
		}
		if !present {
			return invalidErr("seeded reservation user_id is required")
		}
		if _, ok := next.Users[owner]; !ok {
			return invalidErr("seeded reservation owner is unknown")
		}
		restaurantID, present, wrongType := fieldString(e, "restaurant_id")
		if wrongType {
			return malformedErr()
		}
		if !present {
			return invalidErr("seeded reservation restaurant_id is required")
		}
		var restaurant *Restaurant
		for i := range next.Restaurants {
			if next.Restaurants[i].ID == restaurantID {
				restaurant = &next.Restaurants[i]
				break
			}
		}
		if restaurant == nil {
			return invalidErr("seeded reservation restaurant is unknown")
		}
		ids, cerr := parseSeedTables(e, restaurant)
		if cerr != nil {
			return cerr
		}
		startsLocal, present, wrongType := fieldString(e, "starts_at_local")
		if wrongType {
			return malformedErr()
		}
		if !present {
			return invalidErr("seeded reservation starts_at_local is required")
		}
		partyRaw, ok := e["party_size"]
		if !ok {
			return invalidErr("seeded reservation party_size is required")
		}
		// The endpoint-specific party_size rule takes precedence over the
		// general wrong-type rule: every invalid value, including strings
		// and booleans, is 422 validation_failed.
		party, isNumber, isInteger := fixtureInt(partyRaw)
		if !isNumber || !isInteger || party < 1 {
			return invalidErr("seeded reservation party_size must be an integer of at least 1")
		}
		status := StatusConfirmed
		if statusRaw, ok := e["status"]; ok {
			statusStr, ok := statusRaw.(string)
			if !ok {
				return malformedErr()
			}
			if statusStr != StatusConfirmed && statusStr != StatusCancelled {
				return invalidErr("seeded reservation status is invalid")
			}
			status = statusStr
		}
		start, err := clock.ResolveLocal(startsLocal, restaurant.Timezone)
		if err != nil {
			if ce, ok := err.(*clock.Error); ok {
				return &codedError{status: 422, code: ce.Code, msg: ce.Code}
			}
			return invalidErr("seeded reservation starts_at_local is invalid")
		}
		end := start.Add(time.Duration(restaurant.ReservationDurationMinutes) * time.Minute)
		if _, exists := next.Reservations[reference]; exists {
			return invalidErr("duplicate seeded reservation reference")
		}
		rec := Reservation{
			ReservationID: id,
			Reference:     reference,
			UserID:        owner,
			RestaurantID:  restaurantID,
			PartySize:     party,
			Status:        status,
			StartsAtLocal: startsLocal,
			StartsAt:      formatTimestamp(start),
			EndsAt:        formatTimestamp(end),
			CreatedAt:     formatTimestamp(time.Now().UTC()),
		}
		setReservationTables(&rec, ids)
		next.Reservations[reference] = rec
	}
	return nil
}

// parseSeedTables resolves a seeded table_id or table_ids to a canonical
// stored set: singletons accept table_id; pairs require declared combinable
// membership in declared order. Unknown fixture tables stay
// validation_failed here (the 404 resource mapping belongs to endpoints).
func parseSeedTables(e map[string]any, restaurant *Restaurant) ([]string, *codedError) {
	_, hasSingle := e["table_id"]
	_, hasMulti := e["table_ids"]
	if hasSingle && hasMulti {
		return nil, invalidErr("seeded table_id and table_ids are mutually exclusive")
	}
	if hasMulti {
		raw, ok := e["table_ids"].([]any)
		if !ok {
			return nil, malformedErr()
		}
		if len(raw) == 0 || len(raw) > 2 {
			return nil, invalidErr("seeded table_ids must hold one or two tables")
		}
		ids := make([]string, 0, len(raw))
		for _, item := range raw {
			id, ok := item.(string)
			if !ok {
				return nil, malformedErr()
			}
			ids = append(ids, id)
		}
		if len(ids) == 2 && ids[0] == ids[1] {
			return nil, invalidErr("duplicate seeded table id")
		}
		if len(ids) == 1 {
			if !tableKnown(restaurant, ids[0]) {
				return nil, invalidErr("seeded reservation table is unknown")
			}
			return ids, nil
		}
		if ordered := canonicalPairOrder(restaurant, ids[0], ids[1]); ordered != nil {
			return ordered, nil
		}
		return nil, invalidErr("seeded tables cannot be combined")
	}
	tableID, present, wrongType := fieldString(e, "table_id")
	if wrongType {
		return nil, malformedErr()
	}
	if !present {
		return nil, invalidErr("seeded reservation table_id is required")
	}
	if !tableKnown(restaurant, tableID) {
		return nil, invalidErr("seeded reservation table is unknown")
	}
	return []string{tableID}, nil
}
