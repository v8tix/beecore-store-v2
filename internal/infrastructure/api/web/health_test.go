package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
)

func TestApp_Healthcheck(t *testing.T) {
	a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))

	w := httptest.NewRecorder()
	a.healthcheck(w, httptest.NewRequest(http.MethodGet, "/healthcheck", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApp_Readiness(t *testing.T) {
	t.Run("no redis client configured is a 503", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))

		w := httptest.NewRecorder()
		a.readiness(w, httptest.NewRequest(http.MethodGet, "/readiness", nil))

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("an unreachable redis is a 503, bounded by the ping timeout", func(t *testing.T) {
		a := newTestApp(t, mocks.NewAuthRemote(t), mocks.NewSessionStore(t))
		// Nothing listens on this address; go-redis will fail to dial it,
		// exercising the same failure branch a down Redis would hit.
		a.cfg.Redis.Client = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})

		w := httptest.NewRecorder()
		a.readiness(w, httptest.NewRequest(http.MethodGet, "/readiness", nil))

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusServiceUnavailable)
		}
	})
}
