package policy

import (
	"tablekeeper/internal/clock"
)

// clockResolve validates a single wall start against grid rules, surfacing
// clock's own error codes (invalid_local_time vs validation_failed).
func clockResolve(local string, rules clock.Rules) (clock.Slot, error) {
	return clock.ValidateSlot(local, rules)
}

// clockSlots lists a date's grid slots under the given rules.
func clockSlots(date string, rules clock.Rules) ([]clock.Slot, error) {
	return clock.Slots(date, rules)
}
