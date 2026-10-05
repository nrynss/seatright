package service

import (
	"tablekeeper/internal/history"
	"tablekeeper/internal/policy"
)

// This file owns the owner-only reservation history and decision views. Both
// resolve the general 401 rule to 404: unknown references, other owners'
// bookings, manager non-owners and missing-or-invalid tokens are all
// indistinguishable 404 not_found, including after cancellation.

// ReservationHistory renders the reservation's own record oldest-first. Only
// its owner may read it. Every entry carries its resulting revision and
// complete frozen accepted terms; old entries never acquire newer terms.
func (s *Service) ReservationHistory(token, reference string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	uid, ok := s.state.Tokens[token]
	if !ok {
		return notFound("unknown reservation")
	}
	if _, cerr := ownedReservation(&s.state, uid, reference); cerr != nil {
		return cerr.Result()
	}
	entries := history.CloneEntries(s.state.Histories[reference])
	if entries == nil {
		entries = []history.Entry{}
	}
	return okResult(map[string]any{"reference": reference, "entries": entries})
}

// ReservationDecision renders the current booking's reference, revision and
// complete accepted terms, including after cancellation. Visibility follows
// the history rule exactly.
func (s *Service) ReservationDecision(token, reference string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	uid, ok := s.state.Tokens[token]
	if !ok {
		return notFound("unknown reservation")
	}
	r, cerr := ownedReservation(&s.state, uid, reference)
	if cerr != nil {
		return cerr.Result()
	}
	return okResult(map[string]any{
		"reference":      reference,
		"revision":       r.Revision,
		"accepted_terms": termsToPublic(policy.CloneTerms(r.AcceptedTerms)),
	})
}
