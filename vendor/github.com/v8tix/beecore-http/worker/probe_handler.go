package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// GracefulShutdownProbeHandler returns an HTTP handler that starts a
// long-running dummy job (a 60-iteration, one-second-per-tick loop) so
// operators can verify a service shuts down gracefully — the job should
// observe context cancellation and stop cleanly instead of being killed
// mid-work. Relocated from beecore-http/web, which had no other reason to
// depend on this package: every consumer of web.Handlers was forced to
// pull in worker's dependency graph just to expose this one debug
// endpoint.
func GracefulShutdownProbeHandler(cfg *Cfg, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		work := func(ctx context.Context, traceID string, _ any, logger *slog.Logger) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("Recovered from panic in work", "trace_id", traceID, "error", rec)
				}
			}()

			for i := 0; i < 60; i++ {
				select {
				case <-ctx.Done():
					logger.Info("Work cancelled", "trace_id", traceID, "reason", ctx.Err())
					return
				default:
					logger.Info("Working", "trace_id", traceID, "iteration", i)
					time.Sleep(time.Second)
				}
			}
		}

		jobs := map[string]JobFunc{"background-work": work}
		w1 := New(cfg, jobs)

		for key := range jobs {
			go func(k string) {
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()

				traceID := uuid.NewString()
				if _, err := w1.Start(ctx, traceID, k, nil, logger); err != nil {
					logger.Error("Failed to start job", "error", err)
				}
			}(key)
		}

		resp, err := json.MarshalIndent(map[string]string{"status": "available"}, "", "\t")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp = append(resp, '\n')

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
	}
}
