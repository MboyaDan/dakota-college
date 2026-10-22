package seed

import "embed"

// Copies of the legal page drafts in docs/legal, kept identical by
// TestLegalDraftsMatchDocs. Seeded as "(Sample) Draft" pages so the layout can
// be reviewed locally; production gets only text the college approves.
//
//go:embed legal/*.md
var legalDrafts embed.FS

var legalPages = []struct{ slug, title, file string }{
	{"privacy", "Privacy notice", "privacy-notice.md"},
	{"cookies", "Cookie notice", "cookie-notice.md"},
	{"terms", "Terms of use", "terms.md"},
	{"accessibility", "Accessibility statement", "accessibility-statement.md"},
}

const draftNote = "(Sample) Draft for review, not approved by the college.\n\n"
