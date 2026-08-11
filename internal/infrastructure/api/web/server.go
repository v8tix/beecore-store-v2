package web

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/v8tix/beecore-eda/config"
)

const (
	defaultIdleTimeout    = time.Minute
	defaultReadTimeout    = 10 * time.Second
	defaultWriteTimeout   = 30 * time.Second
	defaultShutdownPeriod = 20 * time.Second
)

// serveHTTP mirrors application.serveHTTP in the source repo's
// cmd/web/server.go.
func (a *app) serveHTTP(cfg *config.Cfg) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Web.StorePort),
		Handler:      a.routes(),
		ErrorLog:     toStdLogger(a.logger),
		IdleTimeout:  defaultIdleTimeout,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	shutdownErrorChan := make(chan error, 1)

	go func() {
		quitChan := make(chan os.Signal, 1)
		signal.Notify(quitChan, syscall.SIGINT, syscall.SIGTERM)
		s := <-quitChan
		a.logger.Info("serveHTTP", "signal received", s.String())

		ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownPeriod)
		defer cancel()

		shutdownErrorChan <- srv.Shutdown(ctx)
	}()

	a.logger.Info("serveHTTP", "starting server on", fmt.Sprintf("http://localhost%s", srv.Addr))

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}

	// wait for shutdown result
	if err := <-shutdownErrorChan; err != nil {
		a.logger.Error("serveHTTP", "shutdown error", err)
		return fmt.Errorf("shutdown: %w", err)
	}

	cfg.AuthService.Stop()
	a.logger.Info("serveHTTP", "stopped server on", fmt.Sprintf("http://localhost%s", srv.Addr))

	a.wg.Wait()
	return nil
}

// toStdLogger bridges slog.Logger to stdlib log.Logger. Mirrors the free
// function of the same name in the source repo's cmd/web/server.go.
func toStdLogger(l *slog.Logger) *log.Logger {
	return log.New(slogWriter{logger: l}, "", 0)
}

type slogWriter struct {
	logger *slog.Logger
}

func (w slogWriter) Write(p []byte) (n int, err error) {
	w.logger.Error(string(p)) // logs as ERROR; change to .Info or .Warn if needed
	return len(p), nil
}
