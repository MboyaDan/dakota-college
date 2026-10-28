// Package httpapi wires HTTP routes and middleware for the Go API.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/access"
	"github.com/MboyaDan/dakota-college/backend/internal/admissions"
	"github.com/MboyaDan/dakota-college/backend/internal/applicant"
	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/enquiry"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
	"github.com/MboyaDan/dakota-college/backend/internal/kcb"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
	"github.com/MboyaDan/dakota-college/backend/internal/mpesa"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
	"github.com/MboyaDan/dakota-college/backend/internal/portal"
	"github.com/MboyaDan/dakota-college/backend/internal/results"
)

// Pinger checks that a dependency is reachable. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps are the services the router needs. Auth and Access may be nil in unit
// tests that only exercise the probes; their routes are then not mounted.
type Deps struct {
	Logger         *slog.Logger
	DB             Pinger
	TrustedProxies []string
	AllowedOrigins []string
	CookieSecure   bool
	Auth           *auth.Service
	Access         *access.Service
	Content        *content.Service
	// Images may be nil when object storage is not configured; uploads then
	// answer 503.
	Images     *media.Images
	Enquiries  *enquiry.Service
	Applicants *applicant.Service
	// Applications is the applicant's own application form.
	Applications *application.Service
	// Admissions is the staff workflow over submitted applications.
	Admissions *admissions.Service
	// Academics is student records, units, offerings and teaching.
	Academics *academics.Service
	// Portal is the student portal and the staff side of what students ask for.
	Portal *portal.Service
	// Results is marks, approval, release and transcripts.
	Results *results.Service
	// Finance is fee structures, invoices, payments and statements.
	Finance *finance.Service
	// Mpesa is the college paybill (C2B); nil when M-Pesa is not enabled.
	Mpesa *mpesa.Service
	// Kcb is KCB Buni payment notifications; nil when not enabled.
	Kcb    *kcb.Service
	Notify *notify.Service
}

const defaultBodyLimit = 1 << 20 // 1 MB for JSON endpoints

// Route describes one endpoint and how it is protected. The route table is
// what AC-07 tests walk: every route is public or names a permission.
type Route struct {
	Method string
	Path   string
	// Public routes need no session. Otherwise a session is required, and
	// Permission (if set) must be held, and Kind (if set) is the only kind
	// of account allowed (applicant routes).
	Public     bool
	Permission string
	Kind       string
}

type Router struct {
	Engine *gin.Engine
	Routes []Route
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) { r.Engine.ServeHTTP(w, req) }

// NewRouter builds the Gin engine with all /api/v1 routes.
func NewRouter(d Deps) (*Router, error) {
	e := gin.New()
	if err := e.SetTrustedProxies(d.TrustedProxies); err != nil {
		return nil, err
	}
	e.HandleMethodNotAllowed = true
	e.Use(requestID(), accessLog(d.Logger), recoverJSON(d.Logger), checkOrigin(d.AllowedOrigins))

	e.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, "not_found", "No such endpoint.")
	})
	e.NoMethod(func(c *gin.Context) {
		writeError(c, http.StatusMethodNotAllowed, "method_not_allowed", "This endpoint does not accept that method.")
	})

	rt := &Router{Engine: e}
	v1 := e.Group("/api/v1")
	cookie := sessionCookie{secure: d.CookieSecure}

	// addWithLimit mounts a handler with its protection and body size limit.
	// Authenticated routes get the session check, CSRF check (for unsafe
	// methods) and permission check.
	mount := func(r Route, bodyLimit int64, h gin.HandlerFunc) {
		path := r.Path
		r.Path = "/api/v1" + path
		rt.Routes = append(rt.Routes, r)
		chain := []gin.HandlerFunc{limitBody(bodyLimit)}
		if !r.Public {
			chain = append(chain, requireSession(d.Auth, cookie), requireCSRF(cookie))
			if r.Permission != "" {
				chain = append(chain, requirePermission(r.Permission, d.Logger))
			}
			if r.Kind != "" {
				chain = append(chain, requireKind(r.Kind))
			}
		}
		v1.Handle(r.Method, path, append(chain, h)...)
	}
	addWithLimit := func(bodyLimit int64, method, path string, public bool, permission string, h gin.HandlerFunc) {
		mount(Route{Method: method, Path: path, Public: public, Permission: permission}, bodyLimit, h)
	}
	add := func(method, path string, public bool, permission string, h gin.HandlerFunc) {
		addWithLimit(defaultBodyLimit, method, path, public, permission, h)
	}

	h := healthHandlers{db: d.DB, logger: d.Logger}
	add(http.MethodGet, "/health", true, "", h.health)
	add(http.MethodGet, "/ready", true, "", h.ready)

	if d.Auth != nil {
		a := authHandlers{svc: d.Auth, cookie: cookie, logger: d.Logger}
		add(http.MethodPost, "/auth/login", true, "", a.login)
		add(http.MethodPost, "/auth/password-reset", true, "", a.requestReset)
		add(http.MethodPost, "/auth/password-reset/confirm", true, "", a.confirmReset)
		add(http.MethodGet, "/auth/session", false, "", a.session)
		add(http.MethodPost, "/auth/logout", false, "", a.logout)
	}

	if d.Access != nil && d.Auth != nil {
		ad := adminHandlers{svc: d.Access, auth: d.Auth, logger: d.Logger}
		add(http.MethodGet, "/admin/users", false, "users.manage", ad.listUsers)
		add(http.MethodPost, "/admin/users", false, "users.manage", ad.createUser)
		add(http.MethodPut, "/admin/users/:id/roles", false, "users.manage", ad.setUserRoles)
		add(http.MethodPost, "/admin/users/:id/disable", false, "users.manage", ad.disableUser)
		add(http.MethodPost, "/admin/users/:id/enable", false, "users.manage", ad.enableUser)
		add(http.MethodGet, "/admin/roles", false, "roles.manage", ad.listRoles)
		add(http.MethodPost, "/admin/roles", false, "roles.manage", ad.createRole)
		add(http.MethodPut, "/admin/roles/:id/permissions", false, "roles.manage", ad.setRolePermissions)
		add(http.MethodDelete, "/admin/roles/:id", false, "roles.manage", ad.deleteRole)
		add(http.MethodGet, "/admin/permissions", false, "roles.manage", ad.listPermissions)
	}

	if d.Content != nil && d.Auth != nil {
		ch := contentHandlers{svc: d.Content}
		add(http.MethodGet, "/staff/content/:kind", false, content.PermRead, ch.list)
		add(http.MethodPost, "/staff/content/:kind", false, content.PermWrite, ch.create)
		add(http.MethodGet, "/staff/content/:kind/:id", false, content.PermRead, ch.get)
		add(http.MethodPut, "/staff/content/:kind/:id", false, content.PermWrite, ch.update)
		add(http.MethodDelete, "/staff/content/:kind/:id", false, content.PermWrite, ch.remove)
		add(http.MethodDelete, "/staff/content/:kind/:id/pending", false, content.PermWrite, ch.discardPending)
		add(http.MethodPost, "/staff/content/:kind/:id/publish", false, content.PermPublish, ch.publish)
		add(http.MethodPost, "/staff/content/:kind/:id/unpublish", false, content.PermPublish, ch.unpublish)
		add(http.MethodPost, "/staff/content/:kind/:id/archive", false, content.PermPublish, ch.archive)
		add(http.MethodGet, "/staff/settings", false, content.PermRead, ch.listSettings)
		add(http.MethodPut, "/staff/settings", false, content.PermPublish, ch.updateSettings)

		ih := imageHandlers{images: d.Images}
		addWithLimit(media.MaxUploadBytes+(1<<20), http.MethodPost, "/staff/images", false, content.PermWrite, ih.upload)
		add(http.MethodGet, "/staff/images/:id", false, content.PermRead, ih.get)
	}

	if d.Enquiries != nil && d.Auth != nil {
		eh := enquiryHandlers{svc: d.Enquiries}
		add(http.MethodGet, "/staff/enquiries", false, enquiry.PermView, eh.list)
		add(http.MethodGet, "/staff/enquiries/counts", false, enquiry.PermView, eh.counts)
		add(http.MethodGet, "/staff/enquiries/assignees", false, enquiry.PermManage, eh.assignees)
		add(http.MethodGet, "/staff/enquiries/:id", false, enquiry.PermView, eh.get)
		add(http.MethodPost, "/staff/enquiries/:id/assign", false, enquiry.PermManage, eh.assign)
		add(http.MethodPost, "/staff/enquiries/:id/status", false, enquiry.PermManage, eh.setStatus)
		add(http.MethodPost, "/staff/enquiries/:id/notes", false, enquiry.PermManage, eh.addNote)
	}

	if d.Notify != nil && d.Auth != nil {
		nh := notificationHandlers{svc: d.Notify}
		add(http.MethodGet, "/staff/notification-templates", false, notify.PermManage, nh.list)
		add(http.MethodPut, "/staff/notification-templates/:key/:channel", false, notify.PermManage, nh.update)
		add(http.MethodGet, "/staff/notifications", false, notify.PermManage, nh.sent)
		add(http.MethodGet, "/staff/notifications/counts", false, notify.PermManage, nh.counts)
		add(http.MethodPost, "/staff/notifications/:id/resend", false, notify.PermManage, nh.resend)
	}

	if d.Applicants != nil && d.Auth != nil {
		ah := applicantHandlers{svc: d.Applicants, cookie: cookie}
		add(http.MethodPost, "/applicant/sign-in-link", true, "", ah.requestLink)
		add(http.MethodPost, "/applicant/resume", true, "", ah.resume)
		add(http.MethodPost, "/applicant/sign-in", true, "", ah.signIn)
		mount(Route{Method: http.MethodGet, Path: "/applicant/applications", Kind: "applicant"}, defaultBodyLimit, ah.applications)
	}

	if d.Applications != nil && d.Auth != nil {
		fh := applicationHandlers{svc: d.Applications}
		own := func(method, path string, h gin.HandlerFunc) {
			mount(Route{Method: method, Path: path, Kind: "applicant"}, defaultBodyLimit, h)
		}
		own(http.MethodPost, "/applicant/applications", fh.start)
		own(http.MethodGet, "/applicant/applications/:id", fh.get)
		own(http.MethodPut, "/applicant/applications/:id/intake", fh.saveIntake)
		own(http.MethodPut, "/applicant/applications/:id/personal", fh.savePersonal)
		own(http.MethodPut, "/applicant/applications/:id/guardian", fh.saveGuardian)
		own(http.MethodPut, "/applicant/applications/:id/education", fh.saveEducation)
		own(http.MethodPut, "/applicant/applications/:id/sponsor", fh.saveSponsor)
		mount(Route{Method: http.MethodPost, Path: "/applicant/applications/:id/documents", Kind: "applicant"},
			application.DocumentUploadsLimit, fh.uploadDocument)
		own(http.MethodDelete, "/applicant/applications/:id/documents/:doc", fh.removeDocument)
		own(http.MethodPost, "/applicant/applications/:id/documents/:doc/link", fh.documentLink)
		own(http.MethodPost, "/applicant/applications/:id/submit", fh.submit)
		own(http.MethodPost, "/applicant/applications/:id/reply", fh.reply)
		own(http.MethodPost, "/applicant/applications/:id/accept-offer", fh.acceptOffer)
		own(http.MethodPost, "/applicant/applications/:id/withdraw", fh.withdraw)
		own(http.MethodPost, "/applicant/applications/:id/offer-letter/link", fh.offerLetterLink)
		add(http.MethodPost, "/staff/applications/:id/documents/:doc/link", false, application.PermView, fh.staffDocumentLink)
		// Any signed-in account; the link itself says whose it is.
		add(http.MethodGet, "/files/:token", false, "", fh.openFile)
	}

	if d.Admissions != nil && d.Auth != nil {
		xh := admissionsHandlers{svc: d.Admissions}
		add(http.MethodGet, "/staff/applications", false, admissions.PermView, xh.list)
		add(http.MethodGet, "/staff/applications/counts", false, admissions.PermView, xh.counts)
		add(http.MethodGet, "/staff/applications/assignees", false, admissions.PermReview, xh.assignees)
		add(http.MethodGet, "/staff/applications/report", false, admissions.PermView, xh.report)
		add(http.MethodGet, "/staff/applications/:id", false, admissions.PermView, xh.get)
		add(http.MethodPost, "/staff/applications/:id/assign", false, admissions.PermReview, xh.assign)
		add(http.MethodPost, "/staff/applications/:id/notes", false, admissions.PermReview, xh.addNote)
		add(http.MethodPost, "/staff/applications/:id/request-info", false, admissions.PermReview, xh.requestInfo)
		add(http.MethodPost, "/staff/applications/:id/resume-review", false, admissions.PermReview, xh.resumeReview)
		add(http.MethodPost, "/staff/applications/:id/reject", false, admissions.PermOffer, xh.reject)
		add(http.MethodPost, "/staff/applications/:id/offer", false, admissions.PermOffer, xh.offer)
		add(http.MethodPost, "/staff/applications/:id/offer-letter/link", false, admissions.PermView, xh.offerLetterLink)
		add(http.MethodPost, "/staff/applications/:id/enrol", false, admissions.PermEnrol, xh.enrol)
	}

	if d.Academics != nil && d.Auth != nil {
		ac := academicsHandlers{svc: d.Academics}
		staff := func(method, path string, h gin.HandlerFunc) {
			mount(Route{Method: method, Path: path, Kind: "staff"}, defaultBodyLimit, h)
		}
		add(http.MethodGet, "/staff/students", false, academics.PermStudentsView, ac.listStudents)
		add(http.MethodGet, "/staff/students/:id", false, academics.PermStudentsView, ac.getStudent)
		add(http.MethodPost, "/staff/students/:id/status", false, academics.PermStudentsUpdate, ac.setStudentStatus)
		add(http.MethodPost, "/staff/students/:id/cohort", false, academics.PermStudentsUpdate, ac.setStudentCohort)
		add(http.MethodPost, "/staff/students/:id/guardians", false, academics.PermStudentsUpdate, ac.addGuardian)
		add(http.MethodPut, "/staff/students/:id/guardians/:item", false, academics.PermStudentsUpdate, ac.updateGuardian)
		add(http.MethodDelete, "/staff/students/:id/guardians/:item", false, academics.PermStudentsUpdate, ac.removeGuardian)
		add(http.MethodGet, "/staff/cohorts", false, academics.PermStudentsView, ac.listCohorts)
		add(http.MethodPost, "/staff/cohorts", false, academics.PermManage, ac.createCohort)

		// Reference lists any member of staff may read.
		staff(http.MethodGet, "/staff/terms", ac.listTerms)
		staff(http.MethodGet, "/staff/units", ac.listUnits)
		staff(http.MethodGet, "/staff/units/:id", ac.getUnit)
		staff(http.MethodGet, "/staff/programmes/:id/units", ac.programmeUnits)

		add(http.MethodPost, "/staff/terms", false, academics.PermManage, ac.createTerm)
		add(http.MethodPut, "/staff/terms/:id", false, academics.PermManage, ac.updateTerm)
		add(http.MethodPost, "/staff/units", false, academics.PermManage, ac.createUnit)
		add(http.MethodPut, "/staff/units/:id", false, academics.PermManage, ac.updateUnit)
		add(http.MethodPut, "/staff/programmes/:id/units", false, academics.PermManage, ac.setProgrammeUnits)
		add(http.MethodGet, "/staff/trainers", false, academics.PermManage, ac.trainers)
		add(http.MethodGet, "/staff/offerings", false, academics.PermManage, ac.listOfferings)
		add(http.MethodPost, "/staff/offerings", false, academics.PermManage, ac.createOffering)
		add(http.MethodGet, "/staff/offerings/:id", false, academics.PermManage, ac.getOffering)
		add(http.MethodPut, "/staff/offerings/:id/trainers", false, academics.PermManage, ac.setTrainers)
		add(http.MethodPost, "/staff/offerings/:id/timetable", false, academics.PermManage, ac.addTimetableEntry)
		add(http.MethodDelete, "/staff/offerings/:id/timetable/:item", false, academics.PermManage, ac.removeTimetableEntry)
		add(http.MethodPost, "/staff/offerings/:id/registrations", false, academics.PermManage, ac.register)
		add(http.MethodPost, "/staff/offerings/:id/registrations/:item/drop", false, academics.PermManage, ac.drop)

		add(http.MethodGet, "/staff/teaching", false, academics.PermTeach, ac.myTeaching)
		add(http.MethodGet, "/staff/teaching/:id", false, academics.PermTeach, ac.myOffering)

		add(http.MethodGet, "/admin/users/:id/schools", false, "users.manage", ac.userSchools)
		add(http.MethodPut, "/admin/users/:id/schools", false, "users.manage", ac.setUserSchools)
	}

	if d.Portal != nil && d.Academics != nil && d.Auth != nil {
		ph := portalHandlers{svc: d.Portal, academics: d.Academics, auth: d.Auth, cookie: cookie}
		add(http.MethodPost, "/student/sign-in", true, "", ph.signIn)
		add(http.MethodPost, "/student/password-reset", true, "", ph.passwordReset)
		student := func(method, path string, h gin.HandlerFunc) {
			mount(Route{Method: method, Path: path, Kind: "student"}, defaultBodyLimit, h)
		}
		student(http.MethodGet, "/student/dashboard", ph.dashboard)
		student(http.MethodGet, "/student/profile", ph.profile)
		student(http.MethodPut, "/student/profile/phone", ph.setPhone)
		student(http.MethodPost, "/student/profile/change-request", ph.requestChange)
		student(http.MethodPost, "/student/guardians", ph.addGuardian)
		student(http.MethodPut, "/student/guardians/:item", ph.updateGuardian)
		student(http.MethodDelete, "/student/guardians/:item", ph.removeGuardian)
		student(http.MethodGet, "/student/registration", ph.registration)
		student(http.MethodPost, "/student/registration/:id", ph.register)
		student(http.MethodPost, "/student/registration/:id/drop", ph.drop)
		student(http.MethodGet, "/student/timetable", ph.timetable)
		student(http.MethodGet, "/student/notices", ph.notices)
		student(http.MethodGet, "/student/requests", ph.requests)
		student(http.MethodPost, "/student/requests", ph.createRequest)

		add(http.MethodGet, "/staff/notices", false, academics.PermManage, ph.listNotices)
		add(http.MethodPost, "/staff/notices", false, academics.PermManage, ph.postNotice)
		add(http.MethodPost, "/staff/notices/:id/withdraw", false, academics.PermManage, ph.withdrawNotice)
		add(http.MethodGet, "/staff/teaching/:id/notices", false, academics.PermTeach, ph.classNotices)
		add(http.MethodPost, "/staff/teaching/:id/notices", false, academics.PermTeach, ph.postClassNotice)
		add(http.MethodPost, "/staff/teaching/:id/notices/:item/withdraw", false, academics.PermTeach, ph.withdrawClassNotice)
		add(http.MethodGet, "/staff/service-requests", false, academics.PermStudentsView, ph.listServiceRequests)
		add(http.MethodPost, "/staff/service-requests/:id", false, academics.PermStudentsUpdate, ph.updateServiceRequest)
		add(http.MethodGet, "/staff/profile-requests", false, academics.PermStudentsView, ph.listChangeRequests)
		add(http.MethodPost, "/staff/profile-requests/:id/approve", false, academics.PermStudentsUpdate, ph.decideChange(true))
		add(http.MethodPost, "/staff/profile-requests/:id/decline", false, academics.PermStudentsUpdate, ph.decideChange(false))
	}

	if d.Results != nil && d.Academics != nil && d.Auth != nil {
		rh := resultsHandlers{svc: d.Results, academics: d.Academics}
		mount(Route{Method: http.MethodGet, Path: "/staff/programmes/:id/assessment", Kind: "staff"}, defaultBodyLimit, rh.scheme)
		add(http.MethodPut, "/staff/programmes/:id/assessment", false, academics.PermManage, rh.setScheme)

		add(http.MethodGet, "/staff/teaching/:id/results", false, results.PermEnter, rh.trainerSheet)
		addWithLimit(4<<20, http.MethodPut, "/staff/teaching/:id/results/marks", false, results.PermEnter, rh.enterMarks)
		add(http.MethodPost, "/staff/teaching/:id/results/submit", false, results.PermEnter, rh.submit)

		add(http.MethodGet, "/staff/result-approvals", false, results.PermApprove, rh.list("submitted", "submitted", "approved", "released", "draft"))
		add(http.MethodGet, "/staff/result-approvals/:id", false, results.PermApprove, rh.get)
		add(http.MethodPost, "/staff/result-approvals/:id/approve", false, results.PermApprove, rh.decide(rh.approve))
		add(http.MethodPost, "/staff/result-approvals/:id/return", false, results.PermApprove, rh.decide(rh.ret))
		add(http.MethodGet, "/staff/result-releases", false, results.PermRelease, rh.list("approved", "approved", "released"))
		add(http.MethodGet, "/staff/result-releases/:id", false, results.PermRelease, rh.get)
		add(http.MethodPost, "/staff/result-releases/:id/release", false, results.PermRelease, rh.decide(rh.release))

		add(http.MethodGet, "/staff/students/:id/results", false, academics.PermStudentsView, rh.staffStudentResults)
		add(http.MethodPost, "/staff/students/:id/transcripts", false, results.PermRelease, rh.issueTranscript)
		add(http.MethodPost, "/staff/students/:id/transcripts/:item/link", false, academics.PermStudentsView, rh.staffTranscriptLink)

		studentOnly := func(method, path string, h gin.HandlerFunc) {
			mount(Route{Method: method, Path: path, Kind: "student"}, defaultBodyLimit, h)
		}
		studentOnly(http.MethodGet, "/student/results", rh.studentResults)
		studentOnly(http.MethodGet, "/student/results/slip", rh.slip)
		studentOnly(http.MethodPost, "/student/transcripts/:id/link", rh.studentTranscriptLink)
	}

	if d.Finance != nil && d.Auth != nil {
		fh := financeHandlers{svc: d.Finance}
		add(http.MethodGet, "/staff/finance/summary", false, finance.PermView, fh.summary)
		add(http.MethodGet, "/staff/finance/programmes", false, finance.PermView, fh.programmes)
		add(http.MethodGet, "/staff/finance/fee-structures", false, finance.PermView, fh.listStructures)
		add(http.MethodPost, "/staff/finance/fee-structures", false, finance.PermInvoice, fh.createStructure)
		add(http.MethodGet, "/staff/finance/fee-structures/:id", false, finance.PermView, fh.getStructure)
		add(http.MethodPut, "/staff/finance/fee-structures/:id", false, finance.PermInvoice, fh.updateStructure)
		add(http.MethodDelete, "/staff/finance/fee-structures/:id", false, finance.PermInvoice, fh.deleteStructure)

		add(http.MethodGet, "/staff/finance/invoices", false, finance.PermView, fh.listInvoices)
		add(http.MethodPost, "/staff/finance/invoices/issue", false, finance.PermInvoice, fh.issueInvoices)
		add(http.MethodGet, "/staff/finance/invoices/:id", false, finance.PermView, fh.getInvoice)
		add(http.MethodPut, "/staff/finance/invoices/:id", false, finance.PermInvoice, fh.updateInvoice)
		add(http.MethodPost, "/staff/finance/invoices/:id/fill", false, finance.PermInvoice, fh.fillInvoice)
		add(http.MethodPost, "/staff/finance/invoices/:id/issue", false, finance.PermInvoice, fh.issueInvoice)
		add(http.MethodPost, "/staff/finance/invoices/:id/void", false, finance.PermInvoice, fh.voidInvoice)
		add(http.MethodGet, "/staff/finance/invoices/:id/pdf", false, finance.PermView, fh.invoicePDF)
		add(http.MethodPost, "/staff/finance/billing", false, finance.PermInvoice, fh.bill)

		add(http.MethodGet, "/staff/finance/payments", false, finance.PermView, fh.listPayments)
		add(http.MethodPost, "/staff/finance/payments", false, finance.PermRecord, fh.recordPayment)
		add(http.MethodGet, "/staff/finance/payments/:id", false, finance.PermView, fh.getPayment)
		add(http.MethodPost, "/staff/finance/payments/:id/confirm", false, finance.PermConfirm, fh.confirmPayment)
		add(http.MethodPost, "/staff/finance/payments/:id/reject", false, finance.PermConfirm, fh.rejectPayment)
		add(http.MethodGet, "/staff/finance/payments/:id/receipt", false, finance.PermView, fh.receipt)

		add(http.MethodGet, "/staff/finance/arrears", false, finance.PermView, fh.arrears)
		add(http.MethodGet, "/staff/finance/students/:id", false, finance.PermView, fh.account)
		add(http.MethodGet, "/staff/finance/students/:id/statement", false, finance.PermView, fh.statement)
		add(http.MethodPost, "/staff/finance/students/:id/invoices", false, finance.PermInvoice, fh.createInvoice)
		add(http.MethodPost, "/staff/finance/students/:id/adjustments", false, finance.PermAdjust, fh.adjust)

		own := func(method, path string, h gin.HandlerFunc) {
			mount(Route{Method: method, Path: path, Kind: "student"}, defaultBodyLimit, h)
		}
		own(http.MethodGet, "/student/fees", fh.myFees)
		own(http.MethodGet, "/student/fees/statement", fh.myStatement)
		own(http.MethodGet, "/student/fees/invoices/:id", fh.myInvoice)
		own(http.MethodGet, "/student/fees/payments/:id/receipt", fh.myReceipt)
	}

	if d.Mpesa != nil && d.Auth != nil {
		mh := mpesaHandlers{svc: d.Mpesa}
		// Safaricom's callbacks: public, but answered only on the secret path.
		addWithLimit(callbackBodyLimit, http.MethodPost, "/payments/c2b/:secret/validation", true, "", mh.callback("validation"))
		addWithLimit(callbackBodyLimit, http.MethodPost, "/payments/c2b/:secret/confirmation", true, "", mh.callback("confirmation"))

		add(http.MethodGet, "/staff/finance/mpesa", false, finance.PermView, mh.status)
		add(http.MethodGet, "/staff/finance/mpesa/transactions", false, finance.PermView, mh.list)
		add(http.MethodPost, "/staff/finance/mpesa/transactions/:id/assign", false, finance.PermConfirm, mh.assign)
		add(http.MethodPost, "/staff/finance/mpesa/transactions/:id/refund", false, finance.PermConfirm, mh.refund)
		add(http.MethodGet, "/staff/finance/mpesa/reconciliation", false, finance.PermView, mh.reconcile)
		add(http.MethodPost, "/staff/finance/mpesa/register-urls", false, mpesa.PermSetup, mh.register)
		add(http.MethodPost, "/staff/finance/mpesa/simulate", false, mpesa.PermSetup, mh.simulate)
		add(http.MethodPost, "/staff/finance/mpesa/reprocess", false, mpesa.PermSetup, mh.reprocess)
	}

	if d.Kcb != nil && d.Auth != nil {
		kh := kcbHandlers{svc: d.Kcb}
		// KCB's notifications: public, but answered only on the secret path.
		addWithLimit(callbackBodyLimit, http.MethodPost, "/payments/kcb/:secret/notification", true, "", kh.notification)

		add(http.MethodGet, "/staff/finance/kcb", false, finance.PermView, kh.status)
		add(http.MethodGet, "/staff/finance/kcb/transactions", false, finance.PermView, kh.list)
		add(http.MethodPost, "/staff/finance/kcb/transactions/:id/assign", false, finance.PermConfirm, kh.assign)
		add(http.MethodPost, "/staff/finance/kcb/transactions/:id/refund", false, finance.PermConfirm, kh.refund)
		add(http.MethodGet, "/staff/finance/kcb/reconciliation", false, finance.PermView, kh.reconcile)
		add(http.MethodPost, "/staff/finance/kcb/reprocess", false, mpesa.PermSetup, kh.reprocess)
	}

	if d.Content != nil && d.Enquiries != nil {
		ph := publicHandlers{content: d.Content, images: d.Images, enquiries: d.Enquiries}
		add(http.MethodGet, "/public/site", true, "", ph.site)
		add(http.MethodPost, "/public/enquiries", true, "", ph.createEnquiry)
		add(http.MethodGet, "/public/news", true, "", ph.newsList)
		add(http.MethodGet, "/public/news/:slug", true, "", ph.newsArticle)
		add(http.MethodGet, "/public/events", true, "", ph.events)
		add(http.MethodGet, "/public/events/:slug", true, "", ph.event)
	}

	return rt, nil
}

// errorBody is the error shape for every endpoint (see openapi.yaml Error).
// Fields lists per-field problems for validation errors.
type errorBody struct {
	Error struct {
		Code      string                 `json:"code"`
		Message   string                 `json:"message"`
		RequestID string                 `json:"request_id,omitempty"`
		Fields    []content.FieldProblem `json:"fields,omitempty"`
	} `json:"error"`
}

func writeError(c *gin.Context, status int, code, message string) {
	var b errorBody
	b.Error.Code = code
	b.Error.Message = message
	b.Error.RequestID = c.GetString(requestIDKey)
	c.AbortWithStatusJSON(status, b)
}
