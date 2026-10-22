package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func newTestRouter(t *testing.T, db Pinger, logOut io.Writer) *gin.Engine {
	t.Helper()
	if logOut == nil {
		logOut = io.Discard
	}
	r, err := NewRouter(Deps{Logger: slog.New(slog.NewJSONHandler(logOut, nil)), DB: db})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r
}

func do(r http.Handler, method, path string, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not JSON: %q", w.Body.String())
	}
	return m
}

func TestHealthIgnoresDatabase(t *testing.T) {
	r := newTestRouter(t, fakePinger{err: errors.New("down")}, nil)
	w := do(r, http.MethodGet, "/api/v1/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := decode(t, w)["status"]; got != "ok" {
		t.Fatalf("status field = %v", got)
	}
}

func TestReady(t *testing.T) {
	r := newTestRouter(t, fakePinger{}, nil)
	w := do(r, http.MethodGet, "/api/v1/ready", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decode(t, w)
	if body["status"] != "ready" || body["checks"].(map[string]any)["database"] != "ok" {
		t.Fatalf("body = %v", body)
	}
}

func TestReadyDatabaseDownHidesDetail(t *testing.T) {
	var logs strings.Builder
	r := newTestRouter(t, fakePinger{err: errors.New("dial tcp 10.1.2.3:5432: secret-host")}, &logs)
	w := do(r, http.MethodGet, "/api/v1/ready", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	if strings.Contains(w.Body.String(), "secret-host") {
		t.Fatalf("response leaks error detail: %s", w.Body.String())
	}
	if !strings.Contains(logs.String(), "secret-host") {
		t.Fatal("error detail was not logged")
	}
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	r := newTestRouter(t, fakePinger{}, nil)

	w := do(r, http.MethodGet, "/api/v1/health", nil)
	if id := w.Header().Get(requestIDHeader); len(id) != 32 {
		t.Fatalf("generated request ID = %q, want 32 hex chars", id)
	}

	w = do(r, http.MethodGet, "/api/v1/health", http.Header{requestIDHeader: {"abc-12345678"}})
	if id := w.Header().Get(requestIDHeader); id != "abc-12345678" {
		t.Fatalf("request ID = %q, want caller's", id)
	}

	w = do(r, http.MethodGet, "/api/v1/health", http.Header{requestIDHeader: {"bad id\nInjected: yes"}})
	if id := w.Header().Get(requestIDHeader); len(id) != 32 {
		t.Fatalf("malformed caller ID was not replaced: %q", id)
	}
}

func TestUnknownRouteAndMethodUseErrorShape(t *testing.T) {
	r := newTestRouter(t, fakePinger{}, nil)
	for _, tc := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/api/v1/nope", http.StatusNotFound, "not_found"},
		{http.MethodPost, "/api/v1/health", http.StatusMethodNotAllowed, "method_not_allowed"},
	} {
		w := do(r, tc.method, tc.path, nil)
		if w.Code != tc.status {
			t.Errorf("%s %s status = %d, want %d", tc.method, tc.path, w.Code, tc.status)
			continue
		}
		e := decode(t, w)["error"].(map[string]any)
		if e["code"] != tc.code || e["request_id"] == "" {
			t.Errorf("%s %s error = %v", tc.method, tc.path, e)
		}
	}
}

func TestPanicReturnsJSON500(t *testing.T) {
	r := newTestRouter(t, fakePinger{}, nil)
	r.GET("/boom", func(*gin.Context) { panic("kaboom") })
	w := do(r, http.MethodGet, "/boom", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "kaboom") {
		t.Fatal("panic value leaked to client")
	}
	if decode(t, w)["error"].(map[string]any)["code"] != "internal_error" {
		t.Fatalf("body = %s", w.Body.String())
	}
}
