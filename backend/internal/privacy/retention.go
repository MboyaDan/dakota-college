package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
)

// Retention modes (RETENTION_MODE).
const (
	ModeOff    = "off"
	ModeReport = "report"
	ModeApply  = "apply"
)

const (
	day   = 24 * time.Hour
	month = 30 * day
	// erasureBatch caps how many applicants one run erases, so a first run
	// over years of data stays short. The rest wait for the next day.
	erasureBatch = 200
	fileBatch    = 500
	// RunEvery is how often the API runs the schedule.
	RunEvery = 24 * time.Hour
	// firstRunAfter lets the API settle before its first run.
	firstRunAfter = 2 * time.Minute
)

// Rule is one line of the retention schedule. Housekeeping rules remove
// records that have no use once spent (sign-in links, ended sessions) and
// run in report mode too. The others remove personal data the college
// still could use, so they wait for apply mode and the college's approval
// of the periods (DG-06). docs/retention.md is the written schedule.
type Rule struct {
	Key          string        `json:"key"`
	Title        string        `json:"title"`
	Keep         string        `json:"keep"`
	After        time.Duration `json:"-"`
	Housekeeping bool          `json:"housekeeping"`
	run          func(ctx context.Context, s *Service, q *dbq.Queries, before time.Time) (int64, error)
}

// Schedule is the retention schedule, in the order it runs. Periods are
// the defaults in DQ-18 and A-48 until the college confirms them.
var Schedule = []Rule{
	{Key: "sign_in_links", Title: "Spent sign-in, reset and download links", Keep: "1 day after they expire",
		After: day, Housekeeping: true, run: func(ctx context.Context, _ *Service, q *dbq.Queries, before time.Time) (int64, error) {
			return sum(ctx, before, q.DeleteSpentSignInLinks, q.DeleteSpentPasswordResets, q.DeleteSpentFileLinks, q.DeleteSpentLoginChallenges)
		}},
	{Key: "sessions", Title: "Ended sign-in sessions (address and browser)", Keep: "30 days after they end",
		After: 30 * day, Housekeeping: true, run: func(ctx context.Context, _ *Service, q *dbq.Queries, before time.Time) (int64, error) {
			return q.DeleteEndedSessions(ctx, before)
		}},
	{Key: "repeat_protection", Title: "Records that stop a form being sent twice", Keep: "Until they expire",
		Housekeeping: true, run: func(ctx context.Context, _ *Service, q *dbq.Queries, before time.Time) (int64, error) {
			return q.DeleteSpentIdempotencyRecords(ctx, before)
		}},
	{Key: "expired_drafts", Title: "Unfinished applications past their expiry date (marked expired)", Keep: "Until the draft expires",
		Housekeeping: true, run: func(ctx context.Context, _ *Service, q *dbq.Queries, before time.Time) (int64, error) {
			return q.ExpireOverdueDrafts(ctx, &before)
		}},
	{Key: "retention_log", Title: "This log of retention runs", Keep: "24 months",
		After: 24 * month, Housekeeping: true, run: func(ctx context.Context, _ *Service, q *dbq.Queries, before time.Time) (int64, error) {
			return q.DeleteOldRetentionRuns(ctx, before)
		}},
	{Key: "messages", Title: "Record of emails and text messages sent", Keep: "12 months after sending",
		After: 12 * month, run: func(ctx context.Context, _ *Service, q *dbq.Queries, before time.Time) (int64, error) {
			return q.DeleteOldMessages(ctx, before)
		}},
	{Key: "enquiries", Title: "Contact form and other enquiries, with staff notes", Keep: "12 months after they are closed",
		After: 12 * month, run: func(ctx context.Context, _ *Service, q *dbq.Queries, before time.Time) (int64, error) {
			return q.DeleteClosedEnquiries(ctx, &before)
		}},
	{Key: "applicants", Title: "Applicants who did not enrol: personal details, documents and answers", Keep: "12 months after the last application ended or its intake started, whichever is later",
		After: 12 * month, run: func(ctx context.Context, s *Service, q *dbq.Queries, before time.Time) (int64, error) {
			ids, err := q.ApplicantsDueForErasure(ctx, dbq.ApplicantsDueForErasureParams{Before: before, MaxRows: erasureBatch})
			if err != nil {
				return 0, err
			}
			for _, id := range ids {
				if err := eraseApplicant(ctx, q, id, audit.Request{}, "retention"); err != nil {
					return 0, err
				}
			}
			return int64(len(ids)), nil
		}},
}

func sum(ctx context.Context, before time.Time, fns ...func(context.Context, time.Time) (int64, error)) (int64, error) {
	var n int64
	for _, fn := range fns {
		c, err := fn(ctx, before)
		if err != nil {
			return n, err
		}
		n += c
	}
	return n, nil
}

// RuleResult is what one rule found and removed. In report mode Done is 0
// for rules that are not housekeeping.
type RuleResult struct {
	Rule string `json:"rule"`
	Due  int64  `json:"due"`
	Done int64  `json:"done"`
}

// FilesRule names the step that removes the objects behind deleted files.
const FilesRule = "removed_files"

type RunResult struct {
	ID      uuid.UUID    `json:"id"`
	Mode    string       `json:"mode"`
	Results []RuleResult `json:"results"`
}

// Run applies the schedule once in mode (report or apply). Housekeeping is
// committed in both modes. Personal-data rules run in one transaction that
// is committed in apply mode and rolled back in report mode, so a report
// counts exactly what apply would remove. Every run is logged in
// retention_runs; an applied run is also audited.
func (s *Service) Run(ctx context.Context, mode string) (*RunResult, error) {
	if mode != ModeReport && mode != ModeApply {
		return nil, errors.New("retention mode must be report or apply")
	}
	q := dbq.New(s.pool)
	id, err := q.InsertRetentionRun(ctx, mode)
	if err != nil {
		return nil, err
	}
	res := &RunResult{ID: id, Mode: mode}
	runErr := s.runRules(ctx, mode, res)
	if runErr == nil {
		runErr = s.removeFiles(ctx, mode, res)
	}
	results, _ := json.Marshal(res.Results)
	msg := ""
	if runErr != nil {
		msg = runErr.Error()
	}
	// The log row is written even when ctx was cancelled mid-run.
	if err := q.FinishRetentionRun(context.WithoutCancel(ctx), dbq.FinishRetentionRunParams{ID: id, Results: results, Error: msg}); err != nil && runErr == nil {
		runErr = err
	}
	return res, runErr
}

var errRollback = errors.New("report only")

func (s *Service) runRules(ctx context.Context, mode string, res *RunResult) error {
	now := s.now()
	for _, housekeeping := range []bool{true, false} {
		var found []RuleResult
		err := s.tx(ctx, func(q *dbq.Queries) error {
			ok, err := q.RetentionLock(ctx)
			if err != nil {
				return err
			}
			if !ok {
				return ErrBusy
			}
			for _, r := range Schedule {
				if r.Housekeeping != housekeeping {
					continue
				}
				n, err := r.run(ctx, s, q, now.Add(-r.After))
				if err != nil {
					return errors.New(r.Key + ": " + err.Error())
				}
				done := n
				if !housekeeping && mode != ModeApply {
					done = 0
				}
				found = append(found, RuleResult{Rule: r.Key, Due: n, Done: done})
			}
			if !housekeeping {
				if mode != ModeApply {
					return errRollback
				}
				return audit.Write(ctx, q, audit.Request{}, audit.Entry{
					Action: "privacy.retention_applied", EntityType: "retention_run", EntityID: res.ID.String(),
					Details: map[string]any{"results": found},
				})
			}
			return nil
		})
		if err != nil && !errors.Is(err, errRollback) {
			return err
		}
		res.Results = append(res.Results, found...)
	}
	return nil
}

// removeFiles deletes from storage the objects behind files marked deleted,
// by erasure or earlier removals, and records each one that is gone. It
// runs in both modes: those files are already unreachable.
func (s *Service) removeFiles(ctx context.Context, _ string, res *RunResult) error {
	q := dbq.New(s.pool)
	due, err := q.CountFilesAwaitingRemoval(ctx)
	if err != nil {
		return err
	}
	r := RuleResult{Rule: FilesRule, Due: due}
	if s.storage != nil && due > 0 {
		files, err := q.FilesAwaitingRemoval(ctx, fileBatch)
		if err != nil {
			return err
		}
		for _, f := range files {
			if err := s.storage.Delete(ctx, f.Bucket, f.ObjectKey); err != nil && !errors.Is(err, media.ErrObjectNotFound) {
				s.logger.Error("retention: could not remove file", slog.String("file_id", f.ID.String()), slog.String("error", err.Error()))
				continue
			}
			if err := q.MarkFileRemoved(ctx, f.ID); err != nil {
				return err
			}
			r.Done++
		}
	}
	res.Results = append(res.Results, r)
	return nil
}

// RunDaily runs the schedule in the configured mode shortly after start and
// then every RunEvery, until ctx ends. Several API instances may run it:
// the database lock lets one through at a time.
func (s *Service) RunDaily(ctx context.Context) {
	if s.mode == ModeOff {
		s.logger.Warn("retention schedule is off (RETENTION_MODE=off)")
		return
	}
	t := time.NewTimer(firstRunAfter)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		res, err := s.Run(ctx, s.mode)
		switch {
		case errors.Is(err, ErrBusy):
			s.logger.Info("retention: another instance is running the schedule")
		case err != nil && ctx.Err() == nil:
			s.logger.Error("retention run failed", slog.String("error", err.Error()))
		case err == nil:
			s.logger.Info("retention run finished", slog.String("mode", res.Mode), slog.Any("results", res.Results))
		}
		t.Reset(RunEvery)
	}
}

// RunLog is one logged retention run, for the staff screen.
type RunLog struct {
	ID         uuid.UUID       `json:"id"`
	Mode       string          `json:"mode"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at"`
	Results    json.RawMessage `json:"results"`
	Error      string          `json:"error"`
}

// Overview is the schedule, the configured mode and the latest runs.
type Overview struct {
	Mode     string   `json:"mode"`
	Schedule []Rule   `json:"schedule"`
	Runs     []RunLog `json:"runs"`
}

func (s *Service) Overview(ctx context.Context) (*Overview, error) {
	rows, err := dbq.New(s.pool).ListRetentionRuns(ctx, 30)
	if err != nil {
		return nil, err
	}
	o := &Overview{Mode: s.mode, Schedule: Schedule, Runs: make([]RunLog, len(rows))}
	for i, r := range rows {
		o.Runs[i] = RunLog{ID: r.ID, Mode: r.Mode, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Results: r.Results, Error: r.Error}
	}
	return o, nil
}
