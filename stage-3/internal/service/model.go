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
	ReservationID string   `json:"reservation_id"`
	Reference     string   `json:"reference"`
	UserID        string   `json:"user_id"`
	RestaurantID  string   `json:"restaurant_id"`
	TableID       string   `json:"table_id"`
	TableIDs      []string `json:"table_ids"`
	PartySize     int      `json:"party_size"`
	Status        string   `json:"status"`
	StartsAtLocal string   `json:"starts_at_local"`
	StartsAt      string   `json:"starts_at"`
	EndsAt        string   `json:"ends_at"`
	CreatedAt     string   `json:"created_at"`
}

// Public renders the reservation exactly as ordinary API responses carry it:
// the same fields minus the owner UserID. Responses always carry table_ids,
// and carry table_id only when the set has exactly one member.
func (r Reservation) Public() map[string]any {
	ids := append([]string(nil), reservationTableIDs(r)...)
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
	}
	if len(ids) == 1 {
		out["table_id"] = ids[0]
	}
	return out
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
	Users        map[string]User        `json:"users"`
	Tokens       map[string]string      `json:"tokens"`
	Restaurants  []Restaurant           `json:"restaurants"`
	Reservations map[string]Reservation `json:"reservations"`
	Receipts     map[string]Receipt     `json:"receipts"`
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
