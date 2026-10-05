package service

import (
	"encoding/json"
	"unicode/utf8"
)

// missingKey reports an absent or empty Idempotency-Key header.
func missingKey() Result {
	return Result{Status: 400, Body: errorBody("missing_idempotency_key", "idempotency key is required")}
}

// reuseConflict reports a key already used with a different parsed body.
func reuseConflict() Result {
	return conflict("idempotency_key_reuse", "idempotency key already used with a different body")
}

// Idempotent is the generic atomic idempotent write wrapper for the
// idempotency-keyed paths (POST /reservations, POST /reservation-moves,
// wired by S1-D2).
//
// Ordering per §7: the body is parsed as a JSON object first, then the caller
// is authenticated, then the key is validated and the completed receipt is
// resolved before the callback (endpoint validation, resource checks) runs.
// A completed receipt with a different canonical body is 409
// idempotency_key_reuse even when the new body would otherwise be invalid;
// a matching body replays the stored original response with 200 and never
// invokes the callback.
//
// Concurrency and atomicity: auth, receipt lookup, the callback and the
// state+receipt publication all happen under a single hold of the state lock,
// so concurrent identical calls serialize with exactly one 201 and the rest
// 200. The callback runs against a deep-cloned working State; only a 201
// outcome commits the working state together with the immutable serialized
// original response. Any other outcome discards the tentative state and
// stores no receipt, leaving the key reusable. The callback MUST NOT call
// back into Service methods that take the state lock; it operates on the
// given *State only.
func (s *Service) Idempotent(token, method, path, key string, raw []byte, fn func(*State, string, map[string]any) Result) Result {
	obj, canonical, err := ParseBody(raw)
	if err != nil {
		return malformed()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	uid, ok := s.state.Tokens[token]
	if !ok {
		return unauthenticated()
	}
	if key == "" {
		return missingKey()
	}
	if utf8.RuneCountInString(key) > 255 {
		return validationFailed("idempotency key must be 1 to 255 characters")
	}
	rk := ReceiptKey(uid, method, path, key)
	if rc, ok := s.state.Receipts[rk]; ok {
		if rc.Body != canonical {
			return reuseConflict()
		}
		var body any
		if err := json.Unmarshal([]byte(rc.Response), &body); err != nil {
			return internalErr()
		}
		return Result{Status: 200, Body: body}
	}
	work := cloneState(&s.state)
	res := fn(&work, uid, obj)
	if res.Status != 201 {
		return res
	}
	encoded, err := json.Marshal(res.Body)
	if err != nil {
		return internalErr()
	}
	s.state = work
	if s.state.Receipts == nil {
		s.state.Receipts = map[string]Receipt{}
	}
	s.state.Receipts[rk] = Receipt{
		UserID:   uid,
		Method:   method,
		Path:     path,
		Key:      key,
		Body:     canonical,
		Response: string(encoded),
		Status:   201,
	}
	return res
}
