package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"go.uber.org/fx"
)

func RegisterRoutes(
	lifecycle fx.Lifecycle,
	route http.Handler,
) {

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           route,
		ReadHeaderTimeout: 10 * time.Second,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Println("Starting HTTP server on :8080")
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Fatalf("Failed to start server: %v", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("Shutting down HTTP server...")
			// Create a timeout context for shutdown
			shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			// Shutdown the server
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Printf("HTTP server shutdown error: %v", err)
				return err
			}

			log.Println("HTTP server gracefully stopped")
			return nil
		},
	})
}
