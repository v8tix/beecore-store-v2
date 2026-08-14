package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// readinessPingTimeout bounds how long the readiness probe waits on
// Redis. It stays well under typical kubelet probe timeouts so a slow
// Redis fails the probe instead of piling up concurrent checks.
const readinessPingTimeout = 2 * time.Second

// healthcheck is the liveness probe: it only confirms the process is up
// and serving HTTP, with no dependency checks. Mirrors the /healthcheck
// endpoint every other beecore microservice exposes.
func (a *app) healthcheck(w http.ResponseWriter, _ *http.Request) {
	writeHealthJSON(w, http.StatusOK, map[string]string{"status": "available"})
}

// readiness reports whether this instance can currently serve traffic.
// Unlike healthcheck, it pings Redis — the only stateful dependency this
// app holds a client for (session storage, read on every authenticated
// request) — so k8s stops routing traffic here before sessions can
// actually be resolved.
func (a *app) readiness(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Redis.Client == nil {
		writeHealthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "redis dependency is not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), readinessPingTimeout)
	defer cancel()

	if err := a.cfg.Redis.Client.Ping(ctx).Err(); err != nil {
		a.logError(r, err)
		writeHealthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "redis is not reachable"})
		return
	}

	writeHealthJSON(w, http.StatusOK, map[string]string{"status": "available"})
}

func writeHealthJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
