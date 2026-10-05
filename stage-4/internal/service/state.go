package service

import (
	"sync"

	"tablekeeper/internal/history"
	"tablekeeper/internal/policy"
)

// Service owns all mutable state. mu guards every operation including reads,
// so each request observes one consistent transaction. Expensive password
// work may run before locking, but the final duplicate/identity validation
// always happens under the lock.
type Service struct {
	mu    sync.Mutex
	state State
}

// New returns a service with empty state. Test control endpoints (reset,
// import) populate it.
func New() *Service {
	return &Service{state: emptyState()}
}

func emptyState() State {
	return State{
		Users:               map[string]User{},
		Tokens:              map[string]string{},
		Restaurants:         []Restaurant{},
		Reservations:        map[string]Reservation{},
		Receipts:            map[string]Receipt{},
		Policies:            map[string][]policy.Policy{},
		Histories:           map[string][]history.Entry{},
		Series:              map[string]Series{},
		RestaurantRevisions: map[string]int{},
		Plans:               map[string]Replan{},
		Closures:            map[string][]Closure{},
	}
}

// withLock runs fn while holding mu.
func (s *Service) withLock(fn func(*State)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.state)
}

// snapshot returns a deep copy of the current state for atomic read-only
// export: later writes cannot change the exported value.
func (s *Service) snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneState(&s.state)
}

func cloneState(st *State) State {
	out := State{
		Users:               make(map[string]User, len(st.Users)),
		Tokens:              make(map[string]string, len(st.Tokens)),
		Restaurants:         make([]Restaurant, len(st.Restaurants)),
		Reservations:        make(map[string]Reservation, len(st.Reservations)),
		Receipts:            make(map[string]Receipt, len(st.Receipts)),
		Policies:            clonePolicyMap(st.Policies),
		Histories:           cloneHistoryMap(st.Histories),
		Series:              cloneSeriesMap(st.Series),
		RestaurantRevisions: cloneCounterMap(st.RestaurantRevisions),
		Plans:               cloneReplanMap(st.Plans),
		Closures:            cloneClosureMap(st.Closures),
	}
	for k, v := range st.Users {
		out.Users[k] = v
	}
	for k, v := range st.Tokens {
		out.Tokens[k] = v
	}
	for i, r := range st.Restaurants {
		out.Restaurants[i] = cloneRestaurant(r)
	}
	for k, v := range st.Reservations {
		v.TableIDs = cloneStringsPreserveNil(v.TableIDs)
		v.AcceptedTerms = policy.CloneTerms(v.AcceptedTerms)
		out.Reservations[k] = v
	}
	for k, v := range st.Receipts {
		out.Receipts[k] = v
	}
	return out
}

// clonePolicyMap deep-copies the policies map preserving nil-vs-empty at the
// map, per-key slice and nested terms levels.
func clonePolicyMap(in map[string][]policy.Policy) map[string][]policy.Policy {
	if in == nil {
		return nil
	}
	out := make(map[string][]policy.Policy, len(in))
	for k, v := range in {
		if v == nil {
			out[k] = nil
			continue
		}
		cp := make([]policy.Policy, len(v))
		for i, p := range v {
			cp[i] = policy.ClonePolicy(p)
		}
		out[k] = cp
	}
	return out
}

// cloneHistoryMap deep-copies histories preserving nil-vs-empty at the map
// and per-key entry-slice levels (CloneEntries preserves entry internals).
func cloneHistoryMap(in map[string][]history.Entry) map[string][]history.Entry {
	if in == nil {
		return nil
	}
	out := make(map[string][]history.Entry, len(in))
	for k, v := range in {
		if v == nil {
			out[k] = nil
			continue
		}
		out[k] = history.CloneEntries(v)
	}
	return out
}

// cloneSeriesMap deep-copies series preserving nil-vs-empty at the map level
// and nil-vs-empty member slices per series.
func cloneSeriesMap(in map[string]Series) map[string]Series {
	if in == nil {
		return nil
	}
	out := make(map[string]Series, len(in))
	for k, v := range in {
		out[k] = cloneSeries(v)
	}
	return out
}

// cloneCounterMap copies revision counters preserving nil-vs-empty.
func cloneCounterMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// cloneStringsPreserveNil copies a string slice while preserving nil vs
// non-nil empty shape, so producer JSON identity survives cloning.
func cloneStringsPreserveNil(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// cloneSeries deep-copies series members.
func cloneSeries(s Series) Series {
	out := s
	if s.Members != nil {
		out.Members = append([]SeriesMember{}, s.Members...)
	}
	return out
}

func cloneRestaurant(r Restaurant) Restaurant {
	out := r
	if r.OpeningHours != nil {
		out.OpeningHours = append([]OpeningHour{}, r.OpeningHours...)
	}
	if r.Tables != nil {
		out.Tables = append([]Table{}, r.Tables...)
	}
	if r.Combinable != nil {
		out.Combinable = append([][]string{}, r.Combinable...)
		for i, p := range r.Combinable {
			out.Combinable[i] = cloneStringsPreserveNil(p)
		}
	}
	out.ManagerUserIDs = cloneStringsPreserveNil(r.ManagerUserIDs)
	return out
}

// restaurantByID returns the restaurant with the given id, or nil.
func restaurantByID(st *State, id string) *Restaurant {
	for i := range st.Restaurants {
		if st.Restaurants[i].ID == id {
			return &st.Restaurants[i]
		}
	}
	return nil
}

// userByEmail returns the user with the given email address, or nil.
func userByEmail(st *State, email string) *User {
	for _, u := range st.Users {
		if u.Email == email {
			u := u
			return &u
		}
	}
	return nil
}
