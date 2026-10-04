package service

// This file binds the idempotent write endpoints to their callbacks through
// the integrated generic wrapper (idempotency.go): POST /reservations runs
// the integrated creation callback, and POST /reservation-moves runs the
// atomic batch callback in moves.go.

// CreateReservation binds POST /reservations to the integrated creation
// callback. token is the raw bearer token ("" when missing or malformed) and
// key is the raw Idempotency-Key header value.
func (s *Service) CreateReservation(token, key string, raw []byte) Result {
	return s.Idempotent(token, "POST", "/reservations", key, raw, s.createReservationLocked)
}

// MoveReservations binds POST /reservation-moves to the atomic batch
// callback.
func (s *Service) MoveReservations(token, key string, raw []byte) Result {
	return s.Idempotent(token, "POST", "/reservation-moves", key, raw, s.moveReservationsLocked)
}
