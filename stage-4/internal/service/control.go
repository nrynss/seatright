package service

import (
	"encoding/json"
	"time"
	"unicode/utf8"

	"tablekeeper/internal/clock"

	"golang.org/x/crypto/bcrypt"
)

// Export returns the portable snapshot: track, format_version 1 and the
// opaque state object. The snapshot is atomic and read-only; later writes
// cannot change the returned value.
func (s *Service) Export() Result {
	st := s.snapshot()
	return okResult(map[string]any{
		"track":          "tablekeeper",
		"format_version": 1,
		"state":          st,
	})
}

// Import atomically replaces all state with a previously exported object.
// Invalid input returns an error without changing the destination.
func (s *Service) Import(raw []byte) Result {
	obj, _, err := ParseBody(raw)
	if err != nil {
		return malformed()
	}
	track, _, _ := fieldString(obj, "track")
	if track != "tablekeeper" {
		return validationFailed("wrong track")
	}
	version, ok := obj["format_version"]
	if !ok {
		return validationFailed("missing format_version")
	}
	if num, ok := version.(float64); !ok || num != 1 {
		return validationFailed("wrong format_version")
	}
	stateRaw, ok := obj["state"]
	if !ok {
		return validationFailed("missing state")
	}
	stateObj, ok := stateRaw.(map[string]any)
	if !ok {
		return validationFailed("invalid state")
	}
	encoded, err := json.Marshal(stateObj)
	if err != nil {
		return validationFailed("invalid state")
	}
	var st State
	if err := json.Unmarshal(encoded, &st); err != nil {
		return validationFailed("invalid state")
	}
	// Legacy normalization: stage-1 records carry only TableID. After the
	// offside parse and before validation, give each such singleton its
	// canonical ids=[TableID]; pairs already carry canonical TableIDs.
	// Receipt bodies/responses, ids, refs, times, hashes and tokens are
	// never rewritten.
	for ref, res := range st.Reservations {
		if len(res.TableIDs) == 0 && res.TableID != "" {
			res.TableIDs = []string{res.TableID}
			st.Reservations[ref] = res
		}
	}
	if err := normalizeVersionState(&st); err != nil {
		return validationFailed(err.Error())
	}
	if err := validateState(&st); err != nil {
		return validationFailed(err.Error())
	}
	if err := validateVersionState(&st); err != nil {
		return validationFailed(err.Error())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
	normalizeState(&s.state)
	return noContent()
}

// validateState rejects states that could not have come from this service.
// It checks stored-record consistency (identities, owners, configuration
// membership, resolvable local times with absolute durations, receipt
// scope/canonical bodies), not ordinary booking business rules: reset seeds
// enforce only positive party size and known tables, so import must not add
// grid, hours, end-by-closes or capacity upper bounds. No stage-3 numeric
// maxima apply: any large but valid stage-1 cutoff passes.
func validateState(st *State) error {
	if st.Users == nil || st.Tokens == nil || st.Reservations == nil || st.Receipts == nil {
		return errInvalid("invalid state")
	}
	if st.Restaurants == nil {
		return errInvalid("invalid state")
	}
	for id, u := range st.Users {
		// DisplayName may be empty: signup accepts an empty display_name, so
		// a valid account's unchanged export must import.
		if id == "" || u.ID == "" || u.Email == "" || u.PasswordHash == "" {
			return errInvalid("invalid user")
		}
		if id != u.ID {
			return errInvalid("invalid user")
		}
		if len(id) > 64 || len(u.ID) > 64 {
			return errInvalid("invalid user")
		}
		if !validEmail(u.Email) {
			return errInvalid("invalid user")
		}
		if _, err := bcrypt.Cost([]byte(u.PasswordHash)); err != nil {
			return errInvalid("invalid user credentials")
		}
	}
	for tok, uid := range st.Tokens {
		if tok == "" || uid == "" {
			return errInvalid("invalid token")
		}
		if _, ok := st.Users[uid]; !ok {
			return errInvalid("invalid token owner")
		}
	}
	restIDs := map[string]bool{}
	restTables := map[string]map[string]Table{}
	for _, r := range st.Restaurants {
		if r.ID == "" || len(r.ID) > 64 {
			return errInvalid("invalid restaurant")
		}
		if restIDs[r.ID] {
			return errInvalid("invalid restaurant")
		}
		restIDs[r.ID] = true
		if _, err := time.LoadLocation(r.Timezone); err != nil {
			return errInvalid("invalid restaurant timezone")
		}
		if r.SlotMinutes < 1 || r.ReservationDurationMinutes < 1 {
			return errInvalid("invalid restaurant")
		}
		if r.CancellationCutoffMinutes < 0 {
			return errInvalid("invalid restaurant")
		}
		// Nil and empty collections are equivalent: the snapshot clone
		// renders empty slices as null, so an unchanged export of a
		// closed (opening_hours:[]) or table-less restaurant must import.
		if err := validateHours(r.OpeningHours); err != nil {
			return err
		}
		tables, err := validateTables(r.Tables)
		if err != nil {
			return err
		}
		restTables[r.ID] = tables
		if err := validateCombinable(r.Combinable, tables); err != nil {
			return err
		}
	}
	if err := validateReservations(st, restIDs, restTables); err != nil {
		return err
	}
	if err := validateReceipts(st); err != nil {
		return err
	}
	return nil
}

// validateHours mirrors reset fixture hours: known weekdays without
// duplicates, strict HH:MM bounds, closes later than opens on the same day.
func validateHours(hs []OpeningHour) error {
	seen := map[string]bool{}
	for _, h := range hs {
		if !weekdays[h.Weekday] || seen[h.Weekday] {
			return errInvalid("invalid opening hours")
		}
		seen[h.Weekday] = true
		o, ok1 := splitValidationHM(h.Opens)
		c, ok2 := splitValidationHM(h.Closes)
		if !ok1 || !ok2 || c <= o {
			return errInvalid("invalid opening hours")
		}
	}
	return nil
}

// validateTables mirrors reset fixture tables: nonempty ids within the
// opaque limit, no duplicates, and positive capacities. Labels need only be
// present strings (the fixture and spec impose no nonempty rule), so blank
// labels import and are preserved verbatim.
func validateTables(ts []Table) (map[string]Table, error) {
	out := map[string]Table{}
	for _, t := range ts {
		if t.ID == "" || len(t.ID) > 64 {
			return nil, errInvalid("invalid table")
		}
		if _, dup := out[t.ID]; dup {
			return nil, errInvalid("invalid table")
		}
		if t.Capacity < 1 {
			return nil, errInvalid("invalid table")
		}
		out[t.ID] = t
	}
	return out, nil
}

// validateCombinable checks declared pairs: each holds exactly two known
// distinct tables. Declared order is retained; duplicates of the same
// unordered pair are preserved (spec is silent, producer order wins).
func validateCombinable(pairs [][]string, tables map[string]Table) error {
	for _, p := range pairs {
		if len(p) != 2 {
			return errInvalid("invalid combinable pair")
		}
		if p[0] == "" || p[1] == "" || p[0] == p[1] {
			return errInvalid("invalid combinable pair")
		}
		if _, ok := tables[p[0]]; !ok {
			return errInvalid("invalid combinable pair")
		}
		if _, ok := tables[p[1]]; !ok {
			return errInvalid("invalid combinable pair")
		}
	}
	return nil
}

// validateReservations checks reservation identity, ownership, configuration
// reservations remain valid; times are checked for the consistency the seed
// producer guarantees (resolvable local, absolute duration, RFC 3339
// instants). Reset enforces only positive party size and a known table, so
// import keeps that contract: ordinary booking capacity upper bounds live
// in the reservation core, not in import validation.
func validateReservations(st *State, restIDs map[string]bool, restTables map[string]map[string]Table) error {
	seenIDs := map[string]bool{}
	for ref, res := range st.Reservations {
		if ref == "" || ref != res.Reference || !validReference(res.Reference) {
			return errInvalid("invalid reservation")
		}
		if res.ReservationID == "" || len(res.ReservationID) > 64 {
			return errInvalid("invalid reservation")
		}
		if seenIDs[res.ReservationID] {
			return errInvalid("invalid reservation")
		}
		seenIDs[res.ReservationID] = true
		if _, ok := st.Users[res.UserID]; !ok {
			return errInvalid("invalid reservation owner")
		}
		if !restIDs[res.RestaurantID] {
			return errInvalid("invalid reservation restaurant")
		}
		tables := restTables[res.RestaurantID]
		ids := reservationTableIDs(res)
		// Stored singleton fields must agree: a record carrying both a
		// legacy TableID and TableIDs must name the same table, so corrupt
		// mixed fields cannot silently refer to different tables.
		if res.TableID != "" && len(res.TableIDs) > 0 {
			if len(res.TableIDs) != 1 || res.TableIDs[0] != res.TableID {
				return errInvalid("invalid reservation table")
			}
		}
		if len(ids) == 0 || len(ids) > 2 {
			return errInvalid("invalid reservation table")
		}
		if len(ids) == 2 && ids[0] == ids[1] {
			return errInvalid("invalid reservation table")
		}
		for _, id := range ids {
			if _, ok := tables[id]; !ok {
				return errInvalid("invalid reservation table")
			}
		}
		if len(ids) == 2 && !pairDeclared(st, res.RestaurantID, ids) {
			return errInvalid("invalid reservation table")
		}
		if len(ids) == 2 && !isCanonicalPairOrder(st, res.RestaurantID, ids) {
			return errInvalid("invalid reservation table")
		}
		if res.PartySize < 1 {
			return errInvalid("invalid reservation party size")
		}
		if res.Status != StatusConfirmed && res.Status != StatusCancelled {
			return errInvalid("invalid reservation status")
		}
		if err := validateReservationTimes(st, res); err != nil {
			return err
		}
	}
	return nil
}

// pairDeclared reports whether ids names a declared combinable pair of the
// restaurant, in either order. Singletons never reach here.
func isCanonicalPairOrder(st *State, restaurantID string, ids []string) bool {
	if len(ids) != 2 {
		return false
	}
	for _, r := range st.Restaurants {
		if r.ID != restaurantID {
			continue
		}
		ordered := canonicalPairOrder(&r, ids[0], ids[1])
		return ordered != nil && ordered[0] == ids[0] && ordered[1] == ids[1]
	}
	return false
}

func pairDeclared(st *State, restaurantID string, ids []string) bool {
	if len(ids) != 2 {
		return false
	}
	for _, r := range st.Restaurants {
		if r.ID != restaurantID {
			continue
		}
		return canonicalPairOrder(&r, ids[0], ids[1]) != nil
	}
	return false
}

// validateReservationTimes checks the stored-record consistency the producer
// reject), ends_at is exactly duration absolute minutes after starts_at,
// and all three timestamps parse as RFC 3339 with explicit offsets
// (numeric +00:00 accepted, never rewritten). Grid, hours and
// end-by-closes are booking business rules the seed path never enforced,
// so import must not invent them here.
func validateReservationTimes(st *State, res Reservation) error {
	var restaurant *Restaurant
	for i := range st.Restaurants {
		if st.Restaurants[i].ID == res.RestaurantID {
			restaurant = &st.Restaurants[i]
			break
		}
	}
	if restaurant == nil {
		return errInvalid("invalid reservation restaurant")
	}
	startLocal, skipped, err := resolveValidationInstant(res.StartsAtLocal, restaurant.Timezone)
	if err != nil {
		return errInvalid("invalid reservation time")
	}
	start, err := parseImportedInstant(res.StartsAt)
	if err != nil {
		return errInvalid("invalid reservation time")
	}
	end, err := parseImportedInstant(res.EndsAt)
	if err != nil {
		return errInvalid("invalid reservation time")
	}
	if skipped {
		// The seed producer resolves skipped spring-forward walls forward
		// with the post-transition offset rather than rejecting them;
		// import mirrors that stored-record contract (see F4 audit).
		if !sameWallMinute(start, restaurant.Timezone, res.StartsAtLocal) {
			return errInvalid("invalid reservation time")
		}
	} else if !start.Equal(startLocal) {
		return errInvalid("invalid reservation time")
	}
	duration := restaurant.ReservationDurationMinutes
	if res.Revision != 0 {
		if res.AcceptedTerms.ReservationDurationMinutes <= 0 {
			return errInvalid("invalid reservation terms")
		}
		duration = res.AcceptedTerms.ReservationDurationMinutes
	}
	if !end.Equal(start.Add(time.Duration(duration) * time.Minute)) {
		return errInvalid("invalid reservation time")
	}
	if _, err := parseImportedInstant(res.CreatedAt); err != nil {
		return errInvalid("invalid reservation time")
	}
	return nil
}

// parseImportedInstant parses an RFC 3339 timestamp. Both the legacy Zulu
// spelling (current base seed formatter) and explicit numeric offsets
// (integrated producer spelling, including +00:00) compare by instant and
// are retained verbatim on import.
func parseImportedInstant(s string) (time.Time, error) {
	if len(s) == 0 {
		return time.Time{}, errInvalid("invalid reservation time")
	}
	return time.Parse(time.RFC3339, s)
}

// resolveValidationInstant mirrors the seed producer: first occurrence for
// existing walls (reports skipped=false), forward post-transition instant
// for spring-forward skips (reports skipped=true), error for malformed
// input or unknown zones.
func resolveValidationInstant(local, timezone string) (time.Time, bool, error) {
	if t, err := clock.ResolveLocal(local, timezone); err == nil {
		return t, false, nil
	} else if ce, ok := err.(*clock.Error); !ok || ce.Code != "invalid_local_time" {
		return time.Time{}, false, err
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, false, err
	}
	y, mo, d, hh, mm, ok := splitValidationLocal(local)
	if !ok {
		return time.Time{}, false, errInvalid("invalid reservation time")
	}
	return time.Date(y, time.Month(mo), d, hh, mm, 0, 0, loc), true, nil
}

// sameWallMinute reports whether t displays as the bare local wall minute.
func sameWallMinute(t time.Time, timezone, local string) bool {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return false
	}
	return t.In(loc).Format("2006-01-02T15:04") == local
}

// splitValidationLocal parses strict bare YYYY-MM-DDTHH:MM with a real date.
func splitValidationLocal(s string) (y, mo, d, hh, mm int, ok bool) {
	if len(s) != 16 || s[10] != 'T' {
		return 0, 0, 0, 0, 0, false
	}
	if y, mo, d, ok = splitValidationDate(s[:10]); !ok {
		return 0, 0, 0, 0, 0, false
	}
	mins, hok := splitValidationHM(s[11:16])
	if !hok {
		return 0, 0, 0, 0, 0, false
	}
	return y, mo, d, mins / 60, mins % 60, true
}

// splitValidationDate parses strict YYYY-MM-DD with a real calendar day.
func splitValidationDate(s string) (y, m, d int, ok bool) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return 0, 0, 0, false
	}
	for _, part := range []string{s[:4], s[5:7], s[8:10]} {
		if !isValidationDigits(part) {
			return 0, 0, 0, false
		}
	}
	y = validationAtoi(s[:4])
	m = validationAtoi(s[5:7])
	d = validationAtoi(s[8:10])
	if m < 1 || m > 12 || d < 1 || d > validationDaysIn(y, m) {
		return 0, 0, 0, false
	}
	return y, m, d, true
}

func isValidationDigits(s string) bool {
	for i := range s {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func validationDaysIn(y, m int) int {
	switch m {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	}
	return 0
}

func validationAtoi(s string) int {
	n := 0
	for i := range s {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// validateReceipts checks every stored receipt: owner and scoped key match,
// the key respects the 1..255 character envelope, the body is a canonical
// JSON object, and the original response parses as JSON for replays.
func validateReceipts(st *State) error {
	for k, rc := range st.Receipts {
		if rc.UserID == "" || rc.Method == "" || rc.Path == "" || rc.Key == "" {
			return errInvalid("invalid receipt")
		}
		if utf8.RuneCountInString(rc.Key) > 255 {
			return errInvalid("invalid receipt")
		}
		if _, ok := st.Users[rc.UserID]; !ok {
			return errInvalid("invalid receipt owner")
		}
		if k != ReceiptKey(rc.UserID, rc.Method, rc.Path, rc.Key) {
			return errInvalid("invalid receipt")
		}
		obj, canonical, err := ParseBody([]byte(rc.Body))
		if err != nil {
			return errInvalid("invalid receipt")
		}
		_ = obj
		if canonical != rc.Body {
			return errInvalid("invalid receipt")
		}
		var v any
		if err := json.Unmarshal([]byte(rc.Response), &v); err != nil {
			return errInvalid("invalid receipt")
		}
		if _, ok := v.(map[string]any); !ok {
			return errInvalid("invalid receipt")
		}
		if rc.Status != 201 {
			return errInvalid("invalid receipt")
		}
	}
	return nil
}

// toClockHours adapts stored opening hours to the clock package shape.
func toClockHours(hs []OpeningHour) []clock.Hours {
	out := make([]clock.Hours, 0, len(hs))
	for _, h := range hs {
		out = append(out, clock.Hours{Weekday: h.Weekday, Opens: h.Opens, Closes: h.Closes})
	}
	return out
}

// splitValidationHM parses strict HH:MM in 00:00..23:59 to minutes.
func splitValidationHM(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	for i := range s {
		if i == 2 {
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

type invalidError struct{ msg string }

func (e *invalidError) Error() string { return e.msg }

func errInvalid(msg string) error { return &invalidError{msg: msg} }

// normalizeState fills nil collection fields after import so later code can
// rely on non-nil maps and slices.
func normalizeState(st *State) {
	if st.Users == nil {
		st.Users = map[string]User{}
	}
	if st.Tokens == nil {
		st.Tokens = map[string]string{}
	}
	if st.Restaurants == nil {
		st.Restaurants = []Restaurant{}
	}
	if st.Reservations == nil {
		st.Reservations = map[string]Reservation{}
	}
	if st.Receipts == nil {
		st.Receipts = map[string]Receipt{}
	}
	for i := range st.Restaurants {
		if st.Restaurants[i].OpeningHours == nil {
			st.Restaurants[i].OpeningHours = []OpeningHour{}
		}
		if st.Restaurants[i].Tables == nil {
			st.Restaurants[i].Tables = []Table{}
		}
	}
}
