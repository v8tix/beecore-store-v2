package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApp_ServerError(t *testing.T) {
	logger, buf := newCapturingLogger()
	a := &app{logger: logger}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/boom", nil)

	a.serverError(w, r, errors.New("kaboom"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if body := w.Body.String(); !strings.Contains(body, "kaboom") {
		t.Fatalf("got body %q, want it to contain the underlying error message", body)
	}
	if logOutput := buf.String(); !strings.Contains(logOutput, "level=ERROR") || !strings.Contains(logOutput, "kaboom") {
		t.Fatalf("got log output %q, want an ERROR entry mentioning the error", logOutput)
	}
}

func TestApp_NotFound(t *testing.T) {
	logger, buf := newCapturingLogger()
	a := &app{logger: logger}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/missing", nil)

	a.notFound(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
	if body := w.Body.String(); !strings.Contains(body, "The requested resource could not be found") {
		t.Fatalf("got body %q, want it to contain the not-found message", body)
	}
	if logOutput := buf.String(); !strings.Contains(logOutput, "level=WARN") {
		t.Fatalf("got log output %q, want a WARN entry", logOutput)
	}
}

func TestApp_LogError(t *testing.T) {
	logger, buf := newCapturingLogger()
	a := &app{logger: logger}

	r := httptest.NewRequest(http.MethodPost, "/thing", nil)
	a.logError(r, errors.New("bad thing happened"))

	logOutput := buf.String()
	if !strings.Contains(logOutput, "level=ERROR") {
		t.Fatalf("got log output %q, want an ERROR entry", logOutput)
	}
	if !strings.Contains(logOutput, "bad thing happened") {
		t.Fatalf("got log output %q, want it to contain the error message", logOutput)
	}
	if !strings.Contains(logOutput, "POST") {
		t.Fatalf("got log output %q, want it to contain the request method", logOutput)
	}
}
