package service

import (
	"encoding/json"
	"time"

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
	if err := validateState(&st); err != nil {
		return validationFailed(err.Error())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
	normalizeState(&s.state)
	return noContent()
}

// validateState rejects states that could not have come from this service.
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
		if len(id) > 64 || len(u.ID) > 64 {
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
	for _, r := range st.Restaurants {
		if r.ID == "" || len(r.ID) > 64 {
			return errInvalid("invalid restaurant")
		}
		restIDs[r.ID] = true
		if _, err := time.LoadLocation(r.Timezone); err != nil {
			return errInvalid("invalid restaurant timezone")
		}
	}
	for ref, res := range st.Reservations {
		if ref == "" || res.Reference == "" || res.ReservationID == "" {
			return errInvalid("invalid reservation")
		}
		if _, ok := st.Users[res.UserID]; !ok {
			return errInvalid("invalid reservation owner")
		}
		if !restIDs[res.RestaurantID] {
			return errInvalid("invalid reservation restaurant")
		}
	}
	for _, rc := range st.Receipts {
		if rc.UserID == "" || rc.Method == "" || rc.Path == "" || rc.Key == "" {
			return errInvalid("invalid receipt")
		}
	}
	return nil
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
