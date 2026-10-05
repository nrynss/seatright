// Package service implements the Tablekeeper stage-1 foundation: shared state
// contract, authentication, test control endpoints and HTTP routing.
//
// State contract (frozen for later work items S1-C/S1-D and stages 2-4):
//
//	State uses exported serializable fields:
//	  Users        map[userID]User           (keyed by user id)
//	  Tokens       map[token]userID          (bearer token -> owner)
//	  Restaurants  []Restaurant              (fixture order)
//	  Reservations map[reference]Reservation (keyed by booking reference)
//	  Receipts     map[receiptKey]Receipt    (idempotency receipts)
//
//	User carries ID, Email, DisplayName and PasswordHash (bcrypt). Plaintext
//	passwords are never stored.
//
//	Restaurant retains the fixture field names and order, including Tables.
//	Stage 1 stores only stage-1 configuration.
//
//	Reservation stores ReservationID, Reference, UserID (excluded from
//	ordinary JSON responses, present in state/export), RestaurantID, TableID,
//	PartySize, Status, StartsAtLocal, StartsAt, EndsAt and CreatedAt.
//
//	Receipt stores the caller (UserID), Method, Path and Key, the canonical
//	parsed JSON body (Body) and the immutable original response JSON
//	(Response, with its original Status). Receipt identity is
//	user + method + path + key + canonical body; see ReceiptKey and
//	CanonicalBody.
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"tablekeeper/internal/history"
	"tablekeeper/internal/policy"
)

// Confirmation statuses for a reservation.
const (
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
)

// OpeningHour is one weekday entry of a restaurant's opening hours.
type OpeningHour struct {
	Weekday string `json:"weekday"`
	Opens   string `json:"opens"`
	Closes  string `json:"closes"`
}

// Table is a bookable table of a restaurant.
type Table struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Capacity int    `json:"capacity"`
}

// Restaurant is the stage-2 restaurant configuration. Field names and order
// mirror the reset fixture. Combinable holds the declared unordered table
// pairs; absent means no pairs.
type Restaurant struct {
	ID                         string        `json:"id"`
	Name                       string        `json:"name"`
	Timezone                   string        `json:"timezone"`
	SlotMinutes                int           `json:"slot_minutes"`
	ReservationDurationMinutes int           `json:"reservation_duration_minutes"`
	CancellationCutoffMinutes  int           `json:"cancellation_cutoff_minutes"`
	OpeningHours               []OpeningHour `json:"opening_hours"`
	Tables                     []Table       `json:"tables"`
	Combinable                 [][]string    `json:"combinable"`
	ManagerUserIDs             []string      `json:"manager_user_ids"`
}

// User is an account. PasswordHash is a bcrypt hash; plaintext passwords are
// never stored. PasswordHash is part of state/export (a private test
// artifact) and never appears in API responses.
type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	DisplayName  string `json:"display_name"`
	PasswordHash string `json:"password_hash"`
}

// Reservation is a booking. UserID identifies the owner in state and export
// but is excluded from ordinary JSON responses; use Public to render those.
// TableIDs holds the canonical stored set; legacy TableID is retained for
// stage-1 records and equals the single member iff the set has one member.
type Reservation struct {
	ReservationID string       `json:"reservation_id"`
	Reference     string       `json:"reference"`
	UserID        string       `json:"user_id"`
	RestaurantID  string       `json:"restaurant_id"`
	TableID       string       `json:"table_id"`
	TableIDs      []string     `json:"table_ids"`
	PartySize     int          `json:"party_size"`
	Status        string       `json:"status"`
	StartsAtLocal string       `json:"starts_at_local"`
	StartsAt      string       `json:"starts_at"`
	EndsAt        string       `json:"ends_at"`
	CreatedAt     string       `json:"created_at"`
	Revision      int          `json:"revision"`
	AcceptedTerms policy.Terms `json:"accepted_terms"`
}

// Public renders the reservation exactly as ordinary API responses carry it:
// the same fields minus the owner UserID. Responses always carry table_ids,
// and carry table_id only when the set has exactly one member.
func (r Reservation) Public() map[string]any {
	ids := append([]string(nil), reservationTableIDs(r)...)
	terms := policy.CloneTerms(r.AcceptedTerms)
	out := map[string]any{
		"reservation_id":  r.ReservationID,
		"reference":       r.Reference,
		"restaurant_id":   r.RestaurantID,
		"table_ids":       ids,
		"party_size":      r.PartySize,
		"status":          r.Status,
		"starts_at_local": r.StartsAtLocal,
		"starts_at":       r.StartsAt,
		"ends_at":         r.EndsAt,
		"created_at":      r.CreatedAt,
		"revision":        r.Revision,
		"accepted_terms":  termsToPublic(terms),
	}
	if len(ids) == 1 {
		out["table_id"] = ids[0]
	}
	return out
}

// termsToPublic renders accepted terms with the exact specification JSON
// names: policy_version plus the full selected policy snapshot, never
// effective_from.
func termsToPublic(t policy.Terms) map[string]any {
	hours := make([]any, 0, len(t.OpeningHours))
	for _, h := range t.OpeningHours {
		hours = append(hours, map[string]any{
			"weekday": h.Weekday, "opens": h.Opens, "closes": h.Closes,
		})
	}
	caps := make(map[string]any, len(t.Capacities))
	for k, v := range t.Capacities {
		caps[k] = v
	}
	return map[string]any{
		"policy_version":               t.PolicyVersion,
		"slot_minutes":                 t.SlotMinutes,
		"reservation_duration_minutes": t.ReservationDurationMinutes,
		"cancellation_cutoff_minutes":  t.CancellationCutoffMinutes,
		"opening_hours":                hours,
		"capacities":                   caps,
	}
}

// Receipt is one successful idempotent write: who called, where, under which
// key, with which canonical body, and the immutable original response.
type Receipt struct {
	UserID   string `json:"user_id"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Key      string `json:"key"`
	Body     string `json:"body"`
	Response string `json:"response"`
	Status   int    `json:"status"`
}

// State is the whole service state. Every field is exported and serialized
// for export/import; future stages extend these types without renaming or
// dropping existing fields.
type State struct {
	Users               map[string]User            `json:"users"`
	Tokens              map[string]string          `json:"tokens"`
	Restaurants         []Restaurant               `json:"restaurants"`
	Reservations        map[string]Reservation     `json:"reservations"`
	Receipts            map[string]Receipt         `json:"receipts"`
	Policies            map[string][]policy.Policy `json:"policies"`
	Histories           map[string][]history.Entry `json:"histories"`
	Series              map[string]Series          `json:"series"`
	RestaurantRevisions map[string]int             `json:"restaurant_revisions"`
	Plans               map[string]Replan          `json:"plans"`
	Closures            map[string][]Closure       `json:"closures"`
}

// Series is one recurring agreement: ordered members with their scheduled
// dates and diner-exception flags. No series operations or public routes
// exist yet; later ownership resolves ordered Members.
type Series struct {
	ID            string         `json:"series_id"`
	UserID        string         `json:"user_id"`
	RestaurantID  string         `json:"restaurant_id"`
	Revision      int            `json:"revision"`
	IntervalWeeks int            `json:"interval_weeks"`
	Members       []SeriesMember `json:"members"`
}

// SeriesMember is one occurrence of a series: its index, booking reference,
// original scheduled local date and permanent diner-exception flag.
type SeriesMember struct {
	Index         int    `json:"index"`
	Reference     string `json:"reference"`
	ScheduledDate string `json:"scheduled_date"`
	Exception     bool   `json:"exception"`
}

// Closure is one applied table closure: the closed table plus the half-open
// interval [from,to) as RFC 3339 instants with explicit offsets.
type Closure struct {
	TableID string `json:"table_id"`
	From    string `json:"from"`
	To      string `json:"to"`
}

// ReplanAssignment is one considered booking's seating in a plan: its
// reference, canonical table set and whether the set changed.
type ReplanAssignment struct {
	Reference string   `json:"reference"`
	TableIDs  []string `json:"table_ids"`
	Changed   bool     `json:"changed"`
}

// Replan is one stored seating-repair plan: identity, restaurant, the
// restaurant revision at preview time, the proposed closure, every considered
// assignment in reference order, objective values and application state.
// RestaurantID and Applied are internal (never rendered publicly).
type Replan struct {
	ID                 string             `json:"plan_id"`
	RestaurantID       string             `json:"restaurant_id"`
	RestaurantRevision int                `json:"restaurant_revision"`
	Closure            Closure            `json:"closure"`
	Assignments        []ReplanAssignment `json:"assignments"`
	MovedCount         int                `json:"moved_count"`
	UnusedSeats        int                `json:"unused_seats"`
	Applied            bool               `json:"applied"`
}

// ReceiptKey scopes an idempotency key to the calling user, method and path,
// so another user or path may reuse the same key string independently.
func ReceiptKey(userID, method, path, key string) string {
	return userID + "\x00" + method + "\x00" + path + "\x00" + key
}

// CanonicalBody returns the canonical form of a parsed JSON body: key order
// and whitespace do not affect replay identity. v must be the value produced
// by parsing the request body as JSON.
func CanonicalBody(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return "", err
	}
	return compact.String(), nil
}

// ParseBody parses raw request bytes as a JSON object. It returns the parsed
// object, its canonical form, and an error when the bytes do not parse or do
// not hold an object.
func ParseBody(raw []byte) (map[string]any, string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, "", fmt.Errorf("empty body")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, "", err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, "", fmt.Errorf("trailing data")
		}
		return nil, "", err
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("not an object")
	}
	canonical, err := CanonicalBody(obj)
	if err != nil {
		return nil, "", err
	}
	return obj, canonical, nil
}
