package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/emirpasic/gods/queues/arrayqueue"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"
	"github.com/nduyhai/maestro/internal/server"
	"github.com/nduyhai/maestro/internal/task"
	"github.com/nduyhai/maestro/internal/worker"
	"go.uber.org/fx"

	"github.com/google/uuid"
)

func main() {
	db := make(map[uuid.UUID]*task.Task)
	w := worker.Worker{
		Queue: arrayqueue.New(),
		DB:    db,
	}

	fx.New(
		fx.Provide(NewLogger),
		fx.Supply(&w),
		fx.Provide(worker.NewAPI),
		fx.Provide(fx.Annotate(NewRoute, fx.As(new(http.Handler)))),
		fx.Invoke(server.RegisterRoutes),
		fx.Invoke(runTasks),
	).Run()
}

func NewRoute(logger *httplog.Logger, workerApi *worker.API) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(httplog.RequestLogger(logger))
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/greeting", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("welcome"))
	})
	r.Post("/tasks", workerApi.StartTaskHandler)
	r.Get("/tasks", workerApi.GetTasksHandler)
	r.Delete("/tasks/{taskID}", workerApi.StopTaskHandler)
	r.Get("/stats", workerApi.CollectStats)

	return r
}

func NewLogger() *httplog.Logger {
	return httplog.NewLogger("maestro", httplog.Options{
		JSON:             true,
		LogLevel:         slog.LevelDebug,
		Concise:          true,
		RequestHeaders:   true,
		MessageFieldName: "message",
		Tags: map[string]string{
			"version": "v0.0.1",
			"env":     "dev",
		},
		QuietDownRoutes: []string{
			"/",
			"/health",
		},
		QuietDownPeriod: 10 * time.Second,
		SourceFieldName: "maestro",
	})
}

func runTasks(lifecycle fx.Lifecycle, w *worker.Worker, logger *httplog.Logger) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("starting tasks")
			go func() {
				for {
					if w.Queue.Size() != 0 {
						result := w.RunTask()
						if result.Error != nil {
							logger.Error("Error running task:", slog.Any("error", result.Error))
						} else {
							logger.Info("No tasks to process currently.")
						}
						logger.Info("Sleeping for 10 seconds.")
						time.Sleep(10 * time.Second)
					}
				}
			}()
			return nil
		},
	})

}
