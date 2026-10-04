package service

import (
	"sync"
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
		Users:        map[string]User{},
		Tokens:       map[string]string{},
		Restaurants:  []Restaurant{},
		Reservations: map[string]Reservation{},
		Receipts:     map[string]Receipt{},
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
		Users:        make(map[string]User, len(st.Users)),
		Tokens:       make(map[string]string, len(st.Tokens)),
		Restaurants:  make([]Restaurant, len(st.Restaurants)),
		Reservations: make(map[string]Reservation, len(st.Reservations)),
		Receipts:     make(map[string]Receipt, len(st.Receipts)),
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
		v.TableIDs = append([]string(nil), v.TableIDs...)
		out.Reservations[k] = v
	}
	for k, v := range st.Receipts {
		out.Receipts[k] = v
	}
	return out
}

func cloneRestaurant(r Restaurant) Restaurant {
	out := r
	out.OpeningHours = append([]OpeningHour(nil), r.OpeningHours...)
	out.Tables = append([]Table(nil), r.Tables...)
	out.Combinable = append([][]string(nil), r.Combinable...)
	for i, p := range r.Combinable {
		out.Combinable[i] = append([]string(nil), p...)
	}
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
