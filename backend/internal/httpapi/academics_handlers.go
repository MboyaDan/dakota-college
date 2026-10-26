package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
)

// Student records and academics (Stage 11a). The router checks the
// permission; the service applies the school scope (D-133), and records
// outside it answer 404.

type academicsHandlers struct{ svc *academics.Service }

func academicsError(c *gin.Context, err error) {
	var ve *content.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(c, ve.Problems)
	case errors.Is(err, academics.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "That record does not exist, or it is in a school you do not work in.")
	case errors.Is(err, academics.ErrOutsideScope):
		writeError(c, http.StatusForbidden, "outside_your_schools", "You can only change records of the schools you work in.")
	case errors.Is(err, academics.ErrCollegeWide):
		writeError(c, http.StatusForbidden, "college_wide", "Terms apply to every school, so only the registrar's office can change them.")
	case errors.Is(err, academics.ErrDuplicate):
		writeError(c, http.StatusConflict, "duplicate", "This unit is already scheduled in that term.")
	case errors.Is(err, academics.ErrUnchanged):
		writeError(c, http.StatusConflict, "unchanged", "Nothing changed. Reload to see the latest.")
	default:
		internalError(c, err)
	}
}

func (h academicsHandlers) scope(c *gin.Context) (academics.Scope, bool) {
	sc, err := h.svc.ScopeFor(c.Request.Context(), principal(c))
	if err != nil {
		internalError(c, err)
		return sc, false
	}
	return sc, true
}

func recordID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "That record does not exist, or it is in a school you do not work in.")
		return uuid.Nil, false
	}
	return id, true
}

func pageParam(c *gin.Context) (int, bool) {
	v := c.Query("page")
	if v == "" {
		return 1, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 10000 {
		writeError(c, http.StatusBadRequest, "invalid_request", "The page number must be a whole number from 1.")
		return 0, false
	}
	return n, true
}

// ---------- students ----------

func (h academicsHandlers) listStudents(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	f := academics.StudentFilter{Status: c.Query("status"), Search: c.Query("q")}
	if f.Status != "" && !academics.IsStudentStatus(f.Status) {
		writeError(c, http.StatusBadRequest, "invalid_request", "Choose a status from the list.")
		return
	}
	if f.ProgrammeID, ok = optionalUUID(c, "programme", "Choose a programme from the list."); !ok {
		return
	}
	if f.CohortID, ok = optionalUUID(c, "cohort", "Choose a cohort from the list."); !ok {
		return
	}
	if f.Page, ok = pageParam(c); !ok {
		return
	}
	page, err := h.svc.ListStudents(c.Request.Context(), sc, f)
	if err != nil {
		academicsError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// student answers with the student record as it is now.
func (h academicsHandlers) student(c *gin.Context, sc academics.Scope, id uuid.UUID, err error) {
	if err != nil {
		academicsError(c, err)
		return
	}
	st, err := h.svc.GetStudent(c.Request.Context(), sc, id)
	if err != nil {
		academicsError(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// studentAction binds the body into T and runs do on a student in scope.
func studentAction[T any](h academicsHandlers, c *gin.Context, do func(sc academics.Scope, id uuid.UUID, body T) error) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	var body T
	if !bindJSON(c, &body) {
		return
	}
	h.student(c, sc, id, do(sc, id, body))
}

func (h academicsHandlers) getStudent(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	if id, ok := recordID(c, "id"); ok {
		h.student(c, sc, id, nil)
	}
}

func (h academicsHandlers) setStudentStatus(c *gin.Context) {
	studentAction(h, c, func(sc academics.Scope, id uuid.UUID, b struct {
		Status string `json:"status" binding:"max=20"`
		Reason string `json:"reason" binding:"max=20000"`
	}) error {
		return h.svc.SetStatus(c.Request.Context(), principal(c), sc, id, b.Status, b.Reason, auditRequest(c))
	})
}

func (h academicsHandlers) setStudentCohort(c *gin.Context) {
	studentAction(h, c, func(sc academics.Scope, id uuid.UUID, b struct {
		CohortID uuid.UUID `json:"cohort_id" binding:"required"`
	}) error {
		return h.svc.SetCohort(c.Request.Context(), sc, id, b.CohortID, auditRequest(c))
	})
}

func (h academicsHandlers) addGuardian(c *gin.Context) {
	studentAction(h, c, func(sc academics.Scope, id uuid.UUID, b academics.GuardianInput) error {
		return h.svc.AddGuardian(c.Request.Context(), sc, id, b, auditRequest(c))
	})
}

func (h academicsHandlers) updateGuardian(c *gin.Context) {
	gid, ok := recordID(c, "item")
	if !ok {
		return
	}
	studentAction(h, c, func(sc academics.Scope, id uuid.UUID, b academics.GuardianInput) error {
		return h.svc.UpdateGuardian(c.Request.Context(), sc, id, gid, b, auditRequest(c))
	})
}

func (h academicsHandlers) removeGuardian(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	gid, ok := recordID(c, "item")
	if !ok {
		return
	}
	h.student(c, sc, id, h.svc.RemoveGuardian(c.Request.Context(), sc, id, gid, auditRequest(c)))
}

// ---------- reference lists (any staff) ----------

func (h academicsHandlers) listTerms(c *gin.Context) {
	terms, err := h.svc.ListTerms(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": terms})
}

func (h academicsHandlers) listUnits(c *gin.Context) {
	school, ok := optionalUUID(c, "school", "Choose a school from the list.")
	if !ok {
		return
	}
	units, err := h.svc.ListUnits(c.Request.Context(), school, c.Query("q"))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": units})
}

func (h academicsHandlers) getUnit(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	u, err := h.svc.GetUnit(c.Request.Context(), id)
	if err != nil {
		academicsError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h academicsHandlers) programmeUnits(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	units, err := h.svc.ProgrammeUnits(c.Request.Context(), id)
	if err != nil {
		academicsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": units})
}

// ---------- catalogue changes (academics.manage) ----------

func created(c *gin.Context, id uuid.UUID, err error) {
	if err != nil {
		academicsError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h academicsHandlers) createTerm(c *gin.Context) {
	sc, ok := h.scope(c)
	var body academics.TermInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	id, err := h.svc.CreateTerm(c.Request.Context(), sc, body, auditRequest(c))
	created(c, id, err)
}

func (h academicsHandlers) updateTerm(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	var body academics.TermInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.UpdateTerm(c.Request.Context(), sc, id, body, auditRequest(c)); err != nil {
		academicsError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h academicsHandlers) createUnit(c *gin.Context) {
	sc, ok := h.scope(c)
	var body academics.UnitInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	id, err := h.svc.CreateUnit(c.Request.Context(), sc, body, auditRequest(c))
	created(c, id, err)
}

func (h academicsHandlers) updateUnit(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	var body academics.UnitInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.UpdateUnit(c.Request.Context(), sc, id, body, auditRequest(c)); err != nil {
		academicsError(c, err)
		return
	}
	h.getUnit(c)
}

func (h academicsHandlers) setProgrammeUnits(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	var body struct {
		Items []academics.ProgrammeUnitInput `json:"items" binding:"max=200"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.SetProgrammeUnits(c.Request.Context(), sc, id, body.Items, auditRequest(c)); err != nil {
		academicsError(c, err)
		return
	}
	h.programmeUnits(c)
}

func (h academicsHandlers) listCohorts(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	programme, ok := optionalUUID(c, "programme", "Choose a programme from the list.")
	if !ok {
		return
	}
	cohorts, err := h.svc.ListCohorts(c.Request.Context(), sc, programme)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": cohorts})
}

func (h academicsHandlers) createCohort(c *gin.Context) {
	sc, ok := h.scope(c)
	var body academics.CohortInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	id, err := h.svc.CreateCohort(c.Request.Context(), sc, body, auditRequest(c))
	created(c, id, err)
}

// ---------- offerings (academics.manage) ----------

func (h academicsHandlers) listOfferings(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	term, ok := optionalUUID(c, "term", "Choose a term from the list.")
	if !ok {
		return
	}
	unit, ok := optionalUUID(c, "unit", "Choose a unit from the list.")
	if !ok {
		return
	}
	items, err := h.svc.ListOfferings(c.Request.Context(), sc, term, unit)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h academicsHandlers) createOffering(c *gin.Context) {
	sc, ok := h.scope(c)
	var body academics.OfferingInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	id, err := h.svc.CreateOffering(c.Request.Context(), sc, body, auditRequest(c))
	created(c, id, err)
}

// offering answers with the offering as it is now.
func (h academicsHandlers) offering(c *gin.Context, sc academics.Scope, id uuid.UUID, err error) {
	if err != nil {
		academicsError(c, err)
		return
	}
	o, err := h.svc.GetOffering(c.Request.Context(), sc, id)
	if err != nil {
		academicsError(c, err)
		return
	}
	c.JSON(http.StatusOK, o)
}

func offeringAction[T any](h academicsHandlers, c *gin.Context, do func(sc academics.Scope, id uuid.UUID, body T) error) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	var body T
	if !bindJSON(c, &body) {
		return
	}
	h.offering(c, sc, id, do(sc, id, body))
}

func (h academicsHandlers) getOffering(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	if id, ok := recordID(c, "id"); ok {
		h.offering(c, sc, id, nil)
	}
}

func (h academicsHandlers) setTrainers(c *gin.Context) {
	offeringAction(h, c, func(sc academics.Scope, id uuid.UUID, b struct {
		TrainerIDs []uuid.UUID `json:"trainer_ids" binding:"max=50"`
	}) error {
		return h.svc.SetTrainers(c.Request.Context(), principal(c), sc, id, b.TrainerIDs, auditRequest(c))
	})
}

func (h academicsHandlers) addTimetableEntry(c *gin.Context) {
	offeringAction(h, c, func(sc academics.Scope, id uuid.UUID, b academics.TimetableInput) error {
		return h.svc.AddTimetableEntry(c.Request.Context(), sc, id, b, auditRequest(c))
	})
}

func (h academicsHandlers) removeTimetableEntry(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	entryID, ok := recordID(c, "item")
	if !ok {
		return
	}
	h.offering(c, sc, id, h.svc.RemoveTimetableEntry(c.Request.Context(), sc, id, entryID, auditRequest(c)))
}

func (h academicsHandlers) register(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	var body academics.RegisterInput
	if !ok || !bindJSON(c, &body) {
		return
	}
	res, err := h.svc.Register(c.Request.Context(), principal(c), sc, id, body, auditRequest(c))
	if err != nil {
		academicsError(c, err)
		return
	}
	o, err := h.svc.GetOffering(c.Request.Context(), sc, id)
	if err != nil {
		academicsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": res, "offering": o})
}

func (h academicsHandlers) drop(c *gin.Context) {
	sc, ok := h.scope(c)
	if !ok {
		return
	}
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	reg, ok := recordID(c, "item")
	if !ok {
		return
	}
	h.offering(c, sc, id, h.svc.Drop(c.Request.Context(), principal(c), sc, id, reg, auditRequest(c)))
}

func (h academicsHandlers) trainers(c *gin.Context) {
	people, err := h.svc.Trainers(c.Request.Context())
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": people})
}

// ---------- teaching (academics.teach) ----------

func (h academicsHandlers) myTeaching(c *gin.Context) {
	t, err := h.svc.MyTeaching(c.Request.Context(), principal(c).UserID)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h academicsHandlers) myOffering(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	o, err := h.svc.MyOffering(c.Request.Context(), principal(c).UserID, id)
	if err != nil {
		academicsError(c, err)
		return
	}
	c.JSON(http.StatusOK, o)
}

// ---------- school assignments (users.manage) ----------

func (h academicsHandlers) userSchools(c *gin.Context) {
	id, ok := recordID(c, "id")
	if !ok {
		return
	}
	schools, err := h.svc.UserSchools(c.Request.Context(), id)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": schools})
}

func (h academicsHandlers) setUserSchools(c *gin.Context) {
	id, ok := recordID(c, "id")
	var body struct {
		SchoolIDs []uuid.UUID `json:"school_ids" binding:"max=50"`
	}
	if !ok || !bindJSON(c, &body) {
		return
	}
	if err := h.svc.SetUserSchools(c.Request.Context(), id, body.SchoolIDs, auditRequest(c)); err != nil {
		academicsError(c, err)
		return
	}
	h.userSchools(c)
}
