package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cun0/insider-case/internal/config"
	"github.com/cun0/insider-case/internal/jsonlog"
)

func Serve(cfg config.HTTPConfig, logger *jsonlog.Logger, handler http.Handler, onShutdown func(context.Context) error) error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		IdleTimeout:       60 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	shutdownError := make(chan error, 1)

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		logger.PrintInfo("shutting down server", map[string]string{
			"signal": s.String(),
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Stop background workers first
		if onShutdown != nil {
			if err := onShutdown(ctx); err != nil {
				logger.PrintError(err, map[string]string{
					"component": "shutdown_hook",
				})
			}
		}

		shutdownError <- srv.Shutdown(ctx)
	}()

	logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
	})

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Wait for shutdown result.
	if err := <-shutdownError; err != nil {
		return err
	}

	logger.PrintInfo("stopped server", map[string]string{
		"addr": srv.Addr,
	})

	return nil
}
