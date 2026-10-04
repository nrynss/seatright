package service

import (
	"testing"
)

func TestCanonicalBodyIgnoresOrderAndWhitespace(t *testing.T) {
	_, canonA, err := ParseBody([]byte("{\n  \"b\":1, \"a\":[1,2]  }\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, canonB, err := ParseBody([]byte(`{"a":[1,2],"b":1}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if canonA != canonB {
		t.Fatalf("canonical differs: %q vs %q", canonA, canonB)
	}
	if _, _, err := ParseBody([]byte(`{"a":1} {"b":2}`)); err == nil {
		t.Fatal("trailing data should not parse")
	}
	if _, _, err := ParseBody([]byte(`[1,2]`)); err == nil {
		t.Fatal("non-object body should not parse as object")
	}
	if _, _, err := ParseBody([]byte(``)); err == nil {
		t.Fatal("empty body should not parse")
	}
}

func TestReceiptKeyScoping(t *testing.T) {
	if ReceiptKey("u1", "POST", "/reservations", "k") == ReceiptKey("u2", "POST", "/reservations", "k") {
		t.Fatal("key must be scoped by user")
	}
	if ReceiptKey("u1", "POST", "/reservations", "k") == ReceiptKey("u1", "POST", "/reservation-moves", "k") {
		t.Fatal("key must be scoped by path")
	}
	if ReceiptKey("u1", "POST", "/reservations", "k") == ReceiptKey("u1", "POST", "/reservations", "k2") {
		t.Fatal("key must include the key string")
	}
}

func TestStateSnapshotIsolation(t *testing.T) {
	s := newFoundation(t)
	first := s.snapshot()
	s.withLock(func(st *State) {
		st.Users["ghost"] = User{ID: "ghost"}
	})
	if _, found := first.Users["ghost"]; found {
		t.Fatal("snapshot changed after later writes")
	}
}

func TestValidEmail(t *testing.T) {
	valid := []string{"a@b", "ada@example.com", "x.y+z@sub.domain"}
	for _, e := range valid {
		if !validEmail(e) {
			t.Errorf("%q should be valid", e)
		}
	}
	invalid := []string{"", "no-at", "@b", "a@", "a@b@c", "a @b", "a@ b"}
	for _, e := range invalid {
		if validEmail(e) {
			t.Errorf("%q should be invalid", e)
		}
	}
}

func TestPublicExcludesOwner(t *testing.T) {
	r := Reservation{ReservationID: "s1", Reference: "R1", UserID: "u1", RestaurantID: "r",
		TableID: "t", PartySize: 2, Status: StatusConfirmed}
	pub := r.Public()
	if _, ok := pub["user_id"]; ok {
		t.Fatal("public reservation must exclude user_id")
	}
	for _, k := range []string{"reservation_id", "reference", "restaurant_id", "table_id", "party_size", "status", "starts_at_local", "starts_at", "ends_at", "created_at"} {
		if _, ok := pub[k]; !ok {
			t.Errorf("public reservation missing %q", k)
		}
	}
}
