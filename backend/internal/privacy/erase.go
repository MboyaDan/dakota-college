package privacy

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Erased placeholders. The email keeps the unique index happy and can never
// receive mail (.invalid, RFC 2606).
func erasedEmail(applicantID uuid.UUID) string {
	return "erased-" + applicantID.String() + "@erased.invalid"
}

// eraseApplicant removes an applicant's personal details from every
// application, document, consent, message and sign-in record, inside the
// caller's transaction. What stays: the applicant and application rows with
// reference, programme, intake, status and dates, so counts and references
// still work, and audit rows, which never name the person (D-211). Files are
// marked deleted here; their objects go after commit (removeFiles).
//
// It refuses students and applications still being decided. Drafts may be
// erased: the person is asking us to forget an unfinished form.
func eraseApplicant(ctx context.Context, q *dbq.Queries, id uuid.UUID, req audit.Request, reason string) error {
	a, err := q.LockApplicantForErasure(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	switch {
	case a.ErasedAt != nil:
		return ErrAlreadyErased
	case a.IsStudent:
		return ErrIsStudent
	case a.InProgress > 0:
		return ErrInProgress
	}
	if err := q.AllowErasure(ctx); err != nil {
		return err
	}
	// Consents and files first: both find their rows through guardians and
	// documents, which go next.
	if _, err := q.EraseApplicantConsents(ctx, id); err != nil {
		return err
	}
	files, err := q.MarkApplicantFilesDeleted(ctx, dbq.MarkApplicantFilesDeletedParams{ApplicantID: id, UserID: a.UserID})
	if err != nil {
		return err
	}
	steps := []func(context.Context, uuid.UUID) error{
		q.DeleteApplicantGuardians, q.DeleteApplicantEducation, q.DeleteApplicantDocuments,
		q.EraseApplicantOffers, q.EraseApplicantInfoRequests, q.EraseApplicantNotes, q.EraseApplicantStatusReasons,
	}
	for _, step := range steps {
		if err := step(ctx, id); err != nil {
			return err
		}
	}
	apps, err := q.EraseApplicantApplications(ctx, id)
	if err != nil {
		return err
	}
	if err := q.DeleteApplicantSignInLinks(ctx, dbq.DeleteApplicantSignInLinksParams{Email: a.Email, ApplicantID: id}); err != nil {
		return err
	}
	messages, err := q.DeleteMessagesTo(ctx, dbq.DeleteMessagesToParams{Email: a.Email, Phone: a.Phone})
	if err != nil {
		return err
	}
	if err := q.EraseApplicantRow(ctx, dbq.EraseApplicantRowParams{ID: id, ErasedEmail: erasedEmail(id)}); err != nil {
		return err
	}
	if a.UserID != nil {
		if err := q.EraseApplicantUser(ctx, dbq.EraseApplicantUserParams{ID: *a.UserID, ErasedEmail: erasedEmail(id)}); err != nil {
			return err
		}
		if err := q.DeleteUserSessions(ctx, *a.UserID); err != nil {
			return err
		}
		if err := q.DeleteUserPasswordResets(ctx, *a.UserID); err != nil {
			return err
		}
	}
	return audit.Write(ctx, q, req, audit.Entry{
		Action: "privacy.applicant_erased", EntityType: "applicant", EntityID: id.String(),
		Details: map[string]any{"reason": reason, "applications": apps, "files": files, "messages": messages},
	})
}
