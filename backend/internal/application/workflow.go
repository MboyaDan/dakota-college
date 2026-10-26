package application

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// The application state machine (brief 8). Transitions lists every allowed
// move; anything else is refused here and, as a second line, by a database
// trigger reading application_transitions (migration 00013). A test keeps
// the two lists identical.
//
//	draft          -> submitted | expired | withdrawn
//	submitted      -> under_review | withdrawn
//	under_review   -> info_requested | offered | rejected | withdrawn
//	info_requested -> under_review | rejected | withdrawn
//	offered        -> offer_accepted | withdrawn (declined) | expired
//	offer_accepted -> enrolled | withdrawn
//
// enrolled, rejected, withdrawn and expired are final.
var Transitions = map[string][]string{
	"draft":          {"submitted", "expired", "withdrawn"},
	"submitted":      {"under_review", "withdrawn"},
	"under_review":   {"info_requested", "offered", "rejected", "withdrawn"},
	"info_requested": {"under_review", "rejected", "withdrawn"},
	"offered":        {"offer_accepted", "withdrawn", "expired"},
	"offer_accepted": {"enrolled", "withdrawn"},
}

// Statuses are every application status, in workflow order.
var Statuses = []string{
	"draft", "submitted", "under_review", "info_requested", "offered",
	"offer_accepted", "enrolled", "rejected", "withdrawn", "expired",
}

// ErrIllegalTransition means the application is not in a status from which
// the requested change is allowed (often: someone else changed it first).
var ErrIllegalTransition = errors.New("that change is not allowed from the application's current status")

// CanMove reports whether an application may go from one status to another.
func CanMove(from, to string) bool {
	return slices.Contains(Transitions[from], to)
}

// Move changes the status inside the caller's transaction and records who
// did it and why. by is nil when the applicant (or the system) acts. The
// caller must hold the row lock and pass the status it read under it.
func Move(ctx context.Context, q *dbq.Queries, id uuid.UUID, from, to string, by *uuid.UUID, reason string) error {
	if !CanMove(from, to) {
		return ErrIllegalTransition
	}
	n, err := q.SetApplicationStatus(ctx, dbq.SetApplicationStatusParams{ID: id, FromStatus: from, ToStatus: to})
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrIllegalTransition
	}
	return q.InsertStatusHistory(ctx, dbq.InsertStatusHistoryParams{
		ApplicationID: id, FromStatus: &from, ToStatus: to, ChangedBy: by, Reason: reason,
	})
}
