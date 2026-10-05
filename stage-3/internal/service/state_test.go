package service

import (
	"testing"
)

func TestEmptyStateShape(t *testing.T) {
	s := New()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Users == nil || s.state.Tokens == nil || s.state.Reservations == nil || s.state.Receipts == nil {
		t.Fatal("state maps must be non-nil")
	}
	if s.state.Restaurants == nil {
		t.Fatal("restaurants must be non-nil")
	}
}
