package contracts

import "fmt"

// errors.go — shared error / rejection types (SPEC §14.2). Change only via the
// §0.5 coordinated step.

// RejectionReason enumerates why the §14.2 gatekeeper rejected an event. Every
// rejection is logged to the event log as an EventRejected entry so the
// pipeline — and the validator's own coverage — is debuggable.
type RejectionReason string

const (
	RejectSchema               RejectionReason = "schema"                // missing/wrong fields, confidence out of range, unknown type
	RejectReferentialIntegrity RejectionReason = "referential_integrity" // referenced entity does not exist
	RejectTemporalMonotonicity RejectionReason = "temporal_monotonicity" // timestamp before last applied event
	RejectIllegalTransition    RejectionReason = "illegal_transition"    // not a legal state transition (§8.4)
	RejectRangeSanity          RejectionReason = "range_sanity"          // capacity/extent/population out of bounds
	RejectDuplicate            RejectionReason = "duplicate"             // duplicate event ID (idempotency)
)

// RejectionError is returned by StateStore.Apply when an event fails the §14.2
// validation contract. Callers can errors.As to inspect the Reason.
type RejectionError struct {
	EventID EventID
	Reason  RejectionReason
	Detail  string
}

func (e *RejectionError) Error() string {
	return fmt.Sprintf("event %q rejected (%s): %s", e.EventID, e.Reason, e.Detail)
}
