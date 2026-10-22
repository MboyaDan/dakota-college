// Package content is the staff content manager (Stage 5): draft, publish and
// archive for public website content.
//
// Workflow (DQ-14: Content Editor writes, Registrar or Principal publishes):
//   - content.write creates drafts and edits them in place;
//   - editing a published item saves a pending version; the live site keeps
//     showing the published one until someone with content.publish publishes;
//   - publish, unpublish and archive need content.publish;
//   - only never-published drafts can be deleted; published items are archived.
package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

const PermRead, PermWrite, PermPublish = "content.read", "content.write", "content.publish"

var (
	ErrNotFound      = errors.New("not found")
	ErrUnknownKind   = errors.New("unknown content type")
	ErrConflict      = errors.New("changed by someone else since you opened it")
	ErrNeedsPublish  = errors.New("needs content.publish")
	ErrNotDeletable  = errors.New("only drafts that were never published can be deleted")
	ErrInUse         = errors.New("other content refers to this")
	ErrDuplicateSlug = errors.New("slug already used")
	ErrNoWorkflow    = errors.New("this content type has no draft and publish")
	ErrWrongStatus   = errors.New("not allowed in the current status")
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func lookup(name string) (kind, error) {
	k, ok := Kinds[name]
	if !ok {
		return nil, ErrUnknownKind
	}
	return k, nil
}

// Item is what the editor sees: the live document, any pending edit, and a
// version used to detect two people editing at once.
type Item struct {
	Kind             string     `json:"kind"`
	ID               uuid.UUID  `json:"id"`
	State                       // status, published_at, is_sample, updated_at
	Workflow         bool       `json:"workflow"`
	Live             Document   `json:"live"`
	Pending          Document   `json:"pending"`
	PendingUpdatedAt *time.Time `json:"pending_updated_at"`
	Version          string     `json:"version"`
}

func version(state State, pendingAt *time.Time) string {
	if pendingAt != nil {
		return "p-" + pendingAt.UTC().Format(time.RFC3339Nano)
	}
	return "l-" + state.UpdatedAt.UTC().Format(time.RFC3339Nano)
}

func (s *Service) List(ctx context.Context, kindName string) ([]Summary, error) {
	k, err := lookup(kindName)
	if err != nil {
		return nil, err
	}
	q := dbq.New(s.pool)
	items, err := k.List(ctx, q)
	if err != nil {
		return nil, err
	}
	pending, err := q.ListPendingIDs(ctx, k.Name())
	if err != nil {
		return nil, err
	}
	for i := range items {
		for _, id := range pending {
			if items[i].ID == id {
				items[i].HasPending = true
			}
		}
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, kindName string, id uuid.UUID) (*Item, error) {
	k, err := lookup(kindName)
	if err != nil {
		return nil, err
	}
	return s.get(ctx, dbq.New(s.pool), k, id)
}

func (s *Service) get(ctx context.Context, q *dbq.Queries, k kind, id uuid.UUID) (*Item, error) {
	live, state, err := k.Load(ctx, q, id)
	if err != nil {
		return nil, err
	}
	item := &Item{Kind: k.Name(), ID: id, State: state, Workflow: k.Workflow(), Live: live}
	p, err := q.GetPending(ctx, dbq.GetPendingParams{EntityType: k.Name(), EntityID: id})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		if item.Pending, err = k.Decode(p.Document); err != nil {
			return nil, err
		}
		at := p.UpdatedAt
		item.PendingUpdatedAt = &at
	}
	item.Version = version(state, item.PendingUpdatedAt)
	return item, nil
}

// dbError turns constraint violations into errors the editor can act on.
func dbError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrDuplicateSlug
		case "23503":
			return ErrInUse
		case "23514":
			return &ValidationError{Problems: Problems{{Field: "document", Message: "This breaks a content rule: " + pgErr.ConstraintName}}}
		}
	}
	return err
}

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
	return dbError(err)
}

func entry(action string, k kind, id uuid.UUID, details map[string]any) audit.Entry {
	return audit.Entry{Action: action, EntityType: "content." + k.Name(), EntityID: id.String(), Details: details}
}

func (s *Service) Create(ctx context.Context, actor *auth.Principal, kindName string, raw json.RawMessage, req audit.Request) (uuid.UUID, error) {
	k, err := lookup(kindName)
	if err != nil {
		return uuid.Nil, err
	}
	if !k.Workflow() && !actor.Has(PermPublish) {
		return uuid.Nil, ErrNeedsPublish
	}
	doc, err := k.Decode(raw)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = s.tx(ctx, func(q *dbq.Queries) error {
		if id, err = k.Create(ctx, q, doc, actor.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("content.created", k, id, nil))
	})
	return id, err
}

// Update saves an edit. baseVersion is the version the editor started from;
// if someone saved in between, ErrConflict stops the second save from
// silently overwriting the first.
func (s *Service) Update(ctx context.Context, actor *auth.Principal, kindName string, id uuid.UUID, raw json.RawMessage, baseVersion string, req audit.Request) (*Item, error) {
	k, err := lookup(kindName)
	if err != nil {
		return nil, err
	}
	if !k.Workflow() && !actor.Has(PermPublish) {
		return nil, ErrNeedsPublish
	}
	doc, err := k.Decode(raw)
	if err != nil {
		return nil, err
	}
	var out *Item
	err = s.tx(ctx, func(q *dbq.Queries) error {
		cur, err := s.get(ctx, q, k, id)
		if err != nil {
			return err
		}
		if baseVersion != cur.Version {
			return ErrConflict
		}
		pending := k.Workflow() && cur.Status == "published"
		if pending {
			b, err := json.Marshal(doc)
			if err != nil {
				return err
			}
			if _, err := q.UpsertPending(ctx, dbq.UpsertPendingParams{EntityType: k.Name(), EntityID: id, Document: b, UpdatedBy: actor.UserID}); err != nil {
				return err
			}
		} else if err := k.Save(ctx, q, id, doc, actor.UserID); err != nil {
			return err
		}
		if err := audit.Write(ctx, q, req, entry("content.updated", k, id, map[string]any{"pending": pending})); err != nil {
			return err
		}
		out, err = s.get(ctx, q, k, id)
		return err
	})
	return out, err
}

// applyPending writes a pending edit to the live tables and clears it.
func (s *Service) applyPending(ctx context.Context, q *dbq.Queries, k kind, item *Item, actor uuid.UUID) (Document, error) {
	if item.Pending == nil {
		return item.Live, nil
	}
	if err := k.Save(ctx, q, item.ID, item.Pending, actor); err != nil {
		return nil, err
	}
	if err := q.DeletePending(ctx, dbq.DeletePendingParams{EntityType: k.Name(), EntityID: item.ID}); err != nil {
		return nil, err
	}
	return item.Pending, nil
}

// Publish makes the item (with any pending edit) live, after checking it is
// complete. Only content.publish holders reach this (route permission).
func (s *Service) Publish(ctx context.Context, actor *auth.Principal, kindName string, id uuid.UUID, baseVersion string, req audit.Request) (*Item, error) {
	k, err := lookup(kindName)
	if err != nil {
		return nil, err
	}
	if !k.Workflow() {
		return nil, ErrNoWorkflow
	}
	var out *Item
	err = s.tx(ctx, func(q *dbq.Queries) error {
		cur, err := s.get(ctx, q, k, id)
		if err != nil {
			return err
		}
		if baseVersion != cur.Version {
			return ErrConflict
		}
		doc := cur.Live
		if cur.Pending != nil {
			doc = cur.Pending
		}
		if err := k.ReadyToPublish(ctx, q, doc); err != nil {
			return err
		}
		if _, err := s.applyPending(ctx, q, k, cur, actor.UserID); err != nil {
			return err
		}
		publishedAt := cur.PublishedAt
		if publishedAt == nil {
			now := time.Now()
			publishedAt = &now
		}
		if err := k.SetStatus(ctx, q, id, "published", publishedAt, actor.UserID); err != nil {
			return err
		}
		if err := audit.Write(ctx, q, req, entry("content.published", k, id, map[string]any{"had_pending": cur.Pending != nil, "from": cur.Status})); err != nil {
			return err
		}
		out, err = s.get(ctx, q, k, id)
		return err
	})
	return out, err
}

// Unpublish and Archive take an item off the site. A pending edit is kept by
// applying it, so no work is lost.
func (s *Service) Unpublish(ctx context.Context, actor *auth.Principal, kindName string, id uuid.UUID, req audit.Request) (*Item, error) {
	return s.takeDown(ctx, actor, kindName, id, "draft", "content.unpublished", req)
}

func (s *Service) Archive(ctx context.Context, actor *auth.Principal, kindName string, id uuid.UUID, req audit.Request) (*Item, error) {
	return s.takeDown(ctx, actor, kindName, id, "archived", "content.archived", req)
}

func (s *Service) takeDown(ctx context.Context, actor *auth.Principal, kindName string, id uuid.UUID, status, action string, req audit.Request) (*Item, error) {
	k, err := lookup(kindName)
	if err != nil {
		return nil, err
	}
	if !k.Workflow() {
		return nil, ErrNoWorkflow
	}
	var out *Item
	err = s.tx(ctx, func(q *dbq.Queries) error {
		cur, err := s.get(ctx, q, k, id)
		if err != nil {
			return err
		}
		if cur.Status == status {
			return ErrWrongStatus
		}
		if _, err := s.applyPending(ctx, q, k, cur, actor.UserID); err != nil {
			return err
		}
		if err := k.SetStatus(ctx, q, id, status, cur.PublishedAt, actor.UserID); err != nil {
			return err
		}
		if err := audit.Write(ctx, q, req, entry(action, k, id, map[string]any{"from": cur.Status})); err != nil {
			return err
		}
		out, err = s.get(ctx, q, k, id)
		return err
	})
	return out, err
}

func (s *Service) DiscardPending(ctx context.Context, kindName string, id uuid.UUID, req audit.Request) error {
	k, err := lookup(kindName)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		if _, _, err := k.Load(ctx, q, id); err != nil {
			return err
		}
		if err := q.DeletePending(ctx, dbq.DeletePendingParams{EntityType: k.Name(), EntityID: id}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("content.pending_discarded", k, id, nil))
	})
}

func (s *Service) Delete(ctx context.Context, actor *auth.Principal, kindName string, id uuid.UUID, req audit.Request) error {
	k, err := lookup(kindName)
	if err != nil {
		return err
	}
	if !k.Workflow() && !actor.Has(PermPublish) {
		return ErrNeedsPublish
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		_, state, err := k.Load(ctx, q, id)
		if err != nil {
			return err
		}
		if k.Workflow() && (state.Status != "draft" || state.PublishedAt != nil) {
			return ErrNotDeletable
		}
		if err := q.DeletePending(ctx, dbq.DeletePendingParams{EntityType: k.Name(), EntityID: id}); err != nil {
			return err
		}
		if err := k.Remove(ctx, q, id); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("content.deleted", k, id, nil))
	})
}
