// Package audit records who did what, when (brief 13). Entries are
// append-only in the database. Never put secrets (passwords, tokens) in them.
package audit

import (
	"context"
	"encoding/json"
	"net/netip"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Request describes the HTTP request an action came from.
type Request struct {
	ActorID   *uuid.UUID
	RequestID string
	IP        *netip.Addr
	UserAgent string
}

type Entry struct {
	Action     string
	EntityType string
	EntityID   string
	Details    map[string]any
}

// Write inserts one audit row using q, so it joins the caller's transaction:
// if the action rolls back, so does its audit entry.
func Write(ctx context.Context, q *dbq.Queries, r Request, e Entry) error {
	details := []byte("{}")
	if len(e.Details) > 0 {
		var err error
		if details, err = json.Marshal(e.Details); err != nil {
			return err
		}
	}
	return q.InsertAuditLog(ctx, dbq.InsertAuditLogParams{
		ActorUserID: r.ActorID,
		Action:      e.Action,
		EntityType:  e.EntityType,
		EntityID:    nonEmpty(e.EntityID),
		RequestID:   nonEmpty(r.RequestID),
		Ip:          r.IP,
		UserAgent:   nonEmpty(r.UserAgent),
		Details:     details,
	})
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
