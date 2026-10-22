package seed

import "time"

// Sample data. Every value is fake and labelled "(Sample)" where it could be
// mistaken for a fact about the college. School names are the confirmed ones
// (brief section 2); their descriptions are placeholders. Pool Management has
// no programmes: the college will add what it teaches (DQ-02).

type schoolSeed struct {
	slug, name, shortName, accent, summary string
	programmes                             []programmeSeed
}

type programmeSeed struct {
	slug, title, level, minGrade, summary string
	months                                int32
	published                             bool
}

const sampleSummary = "(Sample) The college will add a description of this school."

var schoolSeeds = []schoolSeed{
	{
		slug: "medical-and-health-sciences", name: "School of Medical and Health Sciences",
		shortName: "Medical and Health Sciences", accent: "medical", summary: sampleSummary,
		programmes: []programmeSeed{
			{"example-health-studies-diploma", "Diploma in Example Health Studies (Sample)", "diploma", "C", "(Sample) Placeholder programme for development.", 36, true},
			{"example-care-skills-certificate", "Certificate in Example Care Skills (Sample)", "certificate", "D+", "(Sample) Placeholder programme for development.", 24, true},
		},
	},
	{
		slug: "hospitality-and-tourism", name: "School of Hospitality and Tourism",
		shortName: "Hospitality and Tourism", accent: "hospitality", summary: sampleSummary,
		programmes: []programmeSeed{
			{"example-hospitality-operations-diploma", "Diploma in Example Hospitality Operations (Sample)", "diploma", "C-", "(Sample) Placeholder programme for development.", 30, true},
			{"example-food-safety-short-course", "Short Course in Example Food Safety (Sample)", "short_course", "", "(Sample) Placeholder programme for development.", 3, true},
		},
	},
	{
		slug: "ict", name: "School of ICT", shortName: "ICT", accent: "ict", summary: sampleSummary,
		programmes: []programmeSeed{
			{"example-information-technology-diploma", "Diploma in Example Information Technology (Sample)", "diploma", "C-", "(Sample) Placeholder programme for development.", 24, true},
			{"example-computer-applications-certificate", "Certificate in Example Computer Applications (Sample)", "certificate", "D", "(Sample) Placeholder programme for development.", 12, true},
		},
	},
	{
		slug: "business", name: "School of Business", shortName: "Business", accent: "business", summary: sampleSummary,
		programmes: []programmeSeed{
			{"example-business-management-diploma", "Diploma in Example Business Management (Sample)", "diploma", "C-", "(Sample) Placeholder programme for development.", 24, true},
			// Left as a draft so the draft and publish flow has something to work on.
			{"example-accounting-skills-certificate", "Certificate in Example Accounting Skills (Sample)", "certificate", "D", "(Sample) Draft placeholder programme.", 12, false},
		},
	},
	{
		slug: "pool-management", name: "School of Pool Management", shortName: "Pool Management", accent: "pool",
		summary: "", // left empty for the college to fill in (DQ-02)
	},
}

type faqSeed struct {
	categorySlug, categoryName string
	items                      [][2]string
}

var faqSeeds = []faqSeed{
	{"admissions", "Admissions (Sample)", [][2]string{
		{"(Sample) How do I apply?", "(Sample) Placeholder answer. Real answers are written by college staff."},
		{"(Sample) Can I save my application and finish later?", "(Sample) Placeholder answer."},
	}},
	{"fees", "Fees (Sample)", [][2]string{
		{"(Sample) How can I pay fees?", "(Sample) Placeholder answer. Payment methods are confirmed by the college."},
		{"(Sample) Can I pay in instalments?", "(Sample) Placeholder answer."},
	}},
	{"student-life", "Student life (Sample)", [][2]string{
		{"(Sample) Is accommodation available?", "(Sample) Placeholder answer."},
	}},
}

// Sample site settings: fake contact details that can never reach a real
// person. "+254 000 …" is not a routable Kenyan number; .invalid is a
// reserved top-level domain.
var settingSeeds = map[string]string{
	"contact.phone":            "+254 000 000 001",
	"contact.email":            "info@example.invalid",
	"contact.admissions_email": "admissions@example.invalid",
	"contact.finance_email":    "finance@example.invalid",
	"contact.whatsapp":         "+254000000002",
	"contact.address":          "(Sample) P.O. Box 0000, Makueni",
	"contact.office_hours":     "(Sample) Monday to Friday, 8 am to 5 pm",
	"campus.directions":        "(Sample) Directions to the campus will be added by the college.",
	"campus.visiting_hours":    "(Sample) Visiting hours will be added by the college.",
	"about.motto":              "(Sample) The college's motto will appear here.",
	"about.mission":            "(Sample) The college's mission will appear here.",
	"about.vision":             "(Sample) The college's vision will appear here.",
}

type userSeed struct {
	email, name, kind, role string
}

// One test account per starter role. Names make the fake obvious.
var userSeeds = []userSeed{
	{"admin@example.invalid", "Test Super Admin", "staff", "super_admin"},
	{"ict.admin@example.invalid", "Test ICT Admin 001", "staff", "ict_admin"},
	{"principal@example.invalid", "Test Principal 001", "staff", "principal"},
	{"registrar@example.invalid", "Test Registrar 001", "staff", "registrar"},
	{"admissions@example.invalid", "Test Admissions Officer 001", "staff", "admissions_officer"},
	{"finance@example.invalid", "Test Finance Officer 001", "staff", "finance_officer"},
	{"head.medical@example.invalid", "Test Head of School 001", "staff", "head_of_school"},
	{"trainer@example.invalid", "Test Trainer 001", "staff", "trainer"},
	{"editor@example.invalid", "Test Content Editor 001", "staff", "content_editor"},
	{"student001@example.invalid", "Test Student 001", "student", "student"},
}

type pageSeed struct {
	slug, title, summary, body string
}

// Sample text pages. The legal ones carry the review drafts from docs/legal
// (see pages.go); the others are placeholders for the college's own words.
var pageSeeds = []pageSeed{
	{"admissions", "How to apply (Sample)", "(Sample) How applications work.",
		"(Sample) The college will explain its application steps here.\n\n## Before you apply\n\n- (Sample) Check the entry requirements on the programme page.\n- (Sample) Have copies of your documents ready."},
	{"student-life", "Student life (Sample)", "(Sample) Life on campus.",
		"(Sample) The college will describe clubs, sports, accommodation guidance, health and safety, and student support here."},
	{"alumni", "Alumni (Sample)", "(Sample) For former students.",
		"(Sample) The college will describe its alumni community here.\n\n## Transcripts and certificates\n\n(Sample) Former students can request a transcript or a certificate copy through the contact form, choosing Alumni."},
}

type newsSeed struct{ slug, title, summary, body string }

var newsSeeds = []newsSeed{
	{"sample-news-applications", "(Sample) Applications news", "(Sample) A placeholder news summary.",
		"(Sample) Placeholder article text. Real news is written by college staff.\n\n## A sample heading\n\n- (Sample) A list item\n- (Sample) Another item"},
	{"sample-news-campus", "(Sample) Campus news", "(Sample) Another placeholder summary.", "(Sample) Placeholder article text."},
}

type eventSeed struct {
	slug, title, summary, body, location string
	startsIn                             time.Duration
}

var eventSeeds = []eventSeed{
	{"sample-open-day", "(Sample) Open day", "(Sample) A placeholder event.", "(Sample) Placeholder event details.", "(Sample) Main campus", 21 * 24 * time.Hour},
	{"sample-past-event", "(Sample) Past event", "(Sample) An event that has happened.", "(Sample) Placeholder event details.", "(Sample) Main campus", -14 * 24 * time.Hour},
}
