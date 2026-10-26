package content

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Site settings are facts shown across the site (contact details, mission),
// so changing them needs content.publish. An empty value hides the field.

type Setting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *Service) ListSettings(ctx context.Context) ([]Setting, error) {
	rows, err := dbq.New(s.pool).StaffListSiteSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Setting, 0, len(rows))
	for _, r := range rows {
		out = append(out, Setting{Key: r.Key, Value: r.Value, Description: r.Description, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

func validateSetting(p *Problems, key, v string) {
	switch {
	case strings.HasSuffix(key, "email"):
		p.email(key, v)
	case key == "contact.phone", key == "contact.whatsapp":
		p.phone(key, v)
	case strings.HasPrefix(key, "social."), key == "campus.map_url":
		if v != "" && !strings.HasPrefix(v, "https://") {
			p.add(key, "Use a full https:// link.")
		} else {
			p.link(key, v)
		}
	case key == "applications.draft_expiry_days", key == "admissions.offer_response_days":
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 365 {
			p.add(key, "Enter a number of days from 1 to 365.")
		}
	case key == "student_number.format":
		if !strings.Contains(v, "{NNNN}") {
			p.add(key, "Keep {NNNN} in the format, where the running number goes.")
		}
	}
	p.optional(key, v, 2000)
}

// UpdateSettings changes several settings at once; all or none are saved.
func (s *Service) UpdateSettings(ctx context.Context, actor *auth.Principal, values map[string]string, req audit.Request) error {
	current, err := s.ListSettings(ctx)
	if err != nil {
		return err
	}
	var keys []string
	for _, c := range current {
		keys = append(keys, c.Key)
	}
	var p Problems
	for k, v := range values {
		if !slices.Contains(keys, k) {
			p.add(k, "This setting does not exist.")
			continue
		}
		validateSetting(&p, k, strings.TrimSpace(v))
	}
	if err := p.Err(); err != nil {
		return err
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		changed := []string{}
		for k, v := range values {
			if _, err := q.StaffUpdateSiteSetting(ctx, dbq.StaffUpdateSiteSettingParams{Key: k, Value: strings.TrimSpace(v), Actor: &actor.UserID}); err != nil {
				return err
			}
			changed = append(changed, k)
		}
		slices.Sort(changed)
		return audit.Write(ctx, q, req, audit.Entry{Action: "settings.updated", EntityType: "site_settings", Details: map[string]any{"keys": changed}})
	})
}
