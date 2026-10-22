package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/access"
	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

func concretePath(path string) string {
	return strings.NewReplacer(":id", uuid.NewString(), ":kind", "schools").Replace(path)
}

// AC-07: the backend denies unauthorised calls even if the frontend is
// bypassed. Walks the whole route table, so a new route cannot be forgotten.
func TestEveryRouteIsProtected(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	public := map[string]bool{
		"GET /api/v1/health": true, "GET /api/v1/ready": true, "POST /api/v1/auth/login": true,
		"POST /api/v1/auth/password-reset": true, "POST /api/v1/auth/password-reset/confirm": true,
	}
	permissions := map[string]bool{}
	for _, r := range h.router.Routes {
		key := r.Method + " " + r.Path
		if r.Public != public[key] {
			t.Errorf("%s: public=%v, but the reviewed public list says %v", key, r.Public, public[key])
		}
		if strings.HasPrefix(r.Path, "/api/v1/admin/") && r.Permission == "" {
			t.Errorf("%s has no permission", key)
		}
		if r.Permission != "" {
			permissions[r.Permission] = true
		}
	}

	// 1. No session: every protected route says 401.
	for _, r := range h.router.Routes {
		if r.Public {
			continue
		}
		w := h.do(req{method: r.Method, path: concretePath(r.Path), body: map[string]any{}})
		expectStatus(t, w, http.StatusUnauthorized, "unauthenticated")
	}

	// 2. For each permission, a user holding every other grantable permission
	//    is refused on the routes that need it.
	all, err := dbq.New(h.pool).ListPermissions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for perm := range permissions {
		var others []string
		for _, p := range all {
			if p.Key != perm && p.Key != access.SystemAdmin {
				others = append(others, p.Key)
			}
		}
		roleKey := "all_but_" + strings.ReplaceAll(perm, ".", "_")
		if _, err := h.access.CreateRole(ctx, roleKey, roleKey, "", others, audit.Request{}); err != nil {
			t.Fatal(err)
		}
		email := roleKey + "@example.invalid"
		h.createUser(email, roleKey)
		s := h.mustLogin(email)
		for _, r := range h.router.Routes {
			if r.Permission != perm {
				continue
			}
			w := h.do(req{method: r.Method, path: concretePath(r.Path), body: map[string]any{}, sess: s})
			if w.Code != http.StatusForbidden {
				t.Errorf("%s %s without %s: %d %s", r.Method, r.Path, perm, w.Code, w.Body.String())
			}
		}
	}

	// 3. A student account (no staff permissions) cannot reach any admin route.
	h.createUser("student001@example.invalid", "student")
	student := h.mustLogin("student001@example.invalid")
	for _, r := range h.router.Routes {
		if strings.HasPrefix(r.Path, "/api/v1/admin/") {
			expectStatus(t, h.do(req{method: r.Method, path: concretePath(r.Path), body: map[string]any{}, sess: student}), http.StatusForbidden, "forbidden")
		}
	}
}

func (h *harness) firstSuperAdmin(email string) uuid.UUID {
	h.t.Helper()
	id, err := auth.CreateFirstSuperAdmin(context.Background(), h.pool, email, "Test Super Admin", testPassword)
	if err != nil {
		h.t.Fatal(err)
	}
	return id
}

func setRoles(h *harness, s *session, userID uuid.UUID, roles ...string) *httpResponse {
	w := h.do(req{method: http.MethodPut, path: "/api/v1/admin/users/" + userID.String() + "/roles", body: map[string]any{"role_keys": roles}, sess: s})
	return &httpResponse{w.Code, w.Body.String()}
}

type httpResponse struct {
	code int
	body string
}

func (r *httpResponse) expect(t *testing.T, status int, code string) {
	t.Helper()
	if r.code != status || (code != "" && !strings.Contains(r.body, `"code":"`+code+`"`)) {
		t.Fatalf("got %d %s, want %d %s", r.code, r.body, status, code)
	}
}

// AC-12 and brief 11 safeguards.
func TestSuperAdminAndRoleProtections(t *testing.T) {
	h := newHarness(t)
	adminID := h.firstSuperAdmin("owner@example.invalid")
	h.createUser("ict@example.invalid", "ict_admin")
	staffID := h.createUser("staff@example.invalid", "trainer")

	owner := h.mustLogin("owner@example.invalid")
	ict := h.mustLogin("ict@example.invalid")

	// ICT Admin manages users but cannot create, remove or disable Super Admins.
	setRoles(h, ict, staffID, "super_admin").expect(t, http.StatusForbidden, "needs_super_admin")
	setRoles(h, ict, adminID).expect(t, http.StatusForbidden, "needs_super_admin")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/admin/users/" + adminID.String() + "/disable", sess: ict}), http.StatusForbidden, "needs_super_admin")
	setRoles(h, ict, staffID, "trainer", "content_editor").expect(t, http.StatusNoContent, "")

	// system.admin is not grantable through role configuration, by anyone.
	ictRole := h.roleID("ict_admin").String()
	for _, s := range []*session{owner, ict} {
		w := h.do(req{method: http.MethodPut, path: "/api/v1/admin/roles/" + ictRole + "/permissions", body: map[string]any{"permission_keys": []string{"users.manage", "system.admin"}}, sess: s})
		expectStatus(t, w, http.StatusUnprocessableEntity, "system_admin_not_grantable")
		w = h.do(req{method: http.MethodPost, path: "/api/v1/admin/roles", body: map[string]any{"key": "sneaky", "name": "Sneaky", "permission_keys": []string{"system.admin"}}, sess: s})
		expectStatus(t, w, http.StatusUnprocessableEntity, "system_admin_not_grantable")
	}
	// The Super Admin role itself cannot be edited (it always holds everything).
	expectStatus(t, h.do(req{method: http.MethodPut, path: "/api/v1/admin/roles/" + h.roleID("super_admin").String() + "/permissions", body: map[string]any{"permission_keys": []string{}}, sess: owner}), http.StatusConflict, "protected_role")

	// The last active Super Admin cannot be removed or disabled.
	setRoles(h, owner, adminID).expect(t, http.StatusConflict, "last_super_admin")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/admin/users/" + adminID.String() + "/disable", sess: owner}), http.StatusConflict, "cannot_disable_self")

	// With a second Super Admin, one can be removed, but never the last.
	setRoles(h, owner, staffID, "super_admin").expect(t, http.StatusNoContent, "")
	second := h.mustLogin("staff@example.invalid")
	setRoles(h, second, adminID).expect(t, http.StatusNoContent, "")
	setRoles(h, second, staffID).expect(t, http.StatusConflict, "last_super_admin")
	var active int64
	active, _ = dbq.New(h.pool).CountActiveSuperAdmins(context.Background())
	if active != 1 {
		t.Fatalf("%d active Super Admins, want 1", active)
	}
	if h.auditCount("user.roles_changed") != 3 {
		t.Fatalf("role changes audited %d times, want 3", h.auditCount("user.roles_changed"))
	}

	// Disabling signs the person out and blocks sign-in.
	h.createUser("leaver@example.invalid", "trainer")
	leaver := h.mustLogin("leaver@example.invalid")
	var leaverID uuid.UUID
	_ = h.pool.QueryRow(context.Background(), `SELECT id FROM users WHERE email = 'leaver@example.invalid'`).Scan(&leaverID)
	if w := h.do(req{method: http.MethodPost, path: "/api/v1/admin/users/" + leaverID.String() + "/disable", sess: second}); w.Code != http.StatusNoContent {
		t.Fatalf("disable: %d %s", w.Code, w.Body.String())
	}
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: leaver}), http.StatusUnauthorized, "unauthenticated")
	if s, _ := h.login("leaver@example.invalid", testPassword); s != nil {
		t.Fatal("disabled user signed in")
	}
}

func TestRolesInUseAndStarterRolesCannotBeDeleted(t *testing.T) {
	h := newHarness(t)
	h.firstSuperAdmin("owner@example.invalid")
	staffID := h.createUser("staff@example.invalid", "trainer")
	owner := h.mustLogin("owner@example.invalid")

	w := h.do(req{method: http.MethodPost, path: "/api/v1/admin/roles", body: map[string]any{"key": "exam_officer", "name": "Exam Officer", "permission_keys": []string{"students.view"}}, sess: owner})
	if w.Code != http.StatusCreated {
		t.Fatalf("create role: %d %s", w.Code, w.Body.String())
	}
	roleID := h.roleID("exam_officer").String()
	setRoles(h, owner, staffID, "trainer", "exam_officer").expect(t, http.StatusNoContent, "")

	expectStatus(t, h.do(req{method: http.MethodDelete, path: "/api/v1/admin/roles/" + roleID, sess: owner}), http.StatusConflict, "role_in_use")
	setRoles(h, owner, staffID, "trainer").expect(t, http.StatusNoContent, "")
	if w := h.do(req{method: http.MethodDelete, path: "/api/v1/admin/roles/" + roleID, sess: owner}); w.Code != http.StatusNoContent {
		t.Fatalf("delete unused role: %d %s", w.Code, w.Body.String())
	}
	expectStatus(t, h.do(req{method: http.MethodDelete, path: "/api/v1/admin/roles/" + h.roleID("content_editor").String(), sess: owner}), http.StatusConflict, "protected_role")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/admin/roles", body: map[string]any{"key": "Bad Key", "name": "x"}, sess: owner}), http.StatusUnprocessableEntity, "invalid_role_key")
	setRoles(h, owner, staffID, "no_such_role").expect(t, http.StatusUnprocessableEntity, "unknown_role")
}

// Two Super Admins removing each other at the same moment must not leave
// none: the super_admin row lock serialises them.
func TestConcurrentSuperAdminRemovalLeavesOne(t *testing.T) {
	h := newHarness(t)
	a := h.firstSuperAdmin("a@example.invalid")
	b := h.createUser("b@example.invalid", "super_admin")
	ctx := context.Background()

	principalFor := func(id uuid.UUID) *auth.Principal {
		perms, err := dbq.New(h.pool).ListUserPermissionKeys(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return &auth.Principal{UserID: id, Permissions: perms}
	}
	pa, pb := principalFor(a), principalFor(b)

	for round := 0; round < 5; round++ {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); errs[0] = h.access.SetUserRoles(ctx, pa, b, nil, audit.Request{}) }()
		go func() { defer wg.Done(); errs[1] = h.access.SetUserRoles(ctx, pb, a, nil, audit.Request{}) }()
		wg.Wait()

		failed := 0
		for _, err := range errs {
			if errors.Is(err, access.ErrLastSuperAdmin) {
				failed++
			} else if err != nil {
				t.Fatal(err)
			}
		}
		n, _ := dbq.New(h.pool).CountActiveSuperAdmins(ctx)
		if n != 1 || failed != 1 {
			t.Fatalf("round %d: %d Super Admins left, %d refused; want 1 and 1", round, n, failed)
		}
		// Restore both for the next round.
		h.exec(`INSERT INTO user_roles (user_id, role_id) SELECT u, (SELECT id FROM roles WHERE key='super_admin') FROM unnest($1::uuid[]) u ON CONFLICT DO NOTHING`, []uuid.UUID{a, b})
	}
}

func TestFirstRunSetup(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for _, bad := range [][3]string{
		{"not-an-email", "Owner", testPassword},
		{"owner@example.invalid", "", testPassword},
		{"owner@example.invalid", "Owner", "short"},
	} {
		if _, err := auth.CreateFirstSuperAdmin(ctx, h.pool, bad[0], bad[1], bad[2]); err == nil {
			t.Errorf("setup accepted %q", bad)
		}
	}
	id := h.firstSuperAdmin("Owner@Example.invalid")
	if _, err := auth.CreateFirstSuperAdmin(ctx, h.pool, "second@example.invalid", "Second", testPassword); !errors.Is(err, auth.ErrAlreadySetUp) {
		t.Fatalf("second setup: %v, want ErrAlreadySetUp", err)
	}
	perms, _ := dbq.New(h.pool).ListUserPermissionKeys(ctx, id)
	if !slices.Contains(perms, access.SystemAdmin) {
		t.Fatal("setup user lacks system.admin")
	}
	h.mustLogin("owner@example.invalid")
	if h.auditCount("setup.super_admin_created") != 1 {
		t.Fatal("setup not audited")
	}
}
