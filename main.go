package main

import (
	"context"
	"go.etcd.io/bbolt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nduyhai/maestro/internal/manager"
	"github.com/samber/lo"
	"resty.dev/v3"

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
	logger := NewLogger()
	db := make(map[uuid.UUID]*task.Task)
	w := worker.Worker{
		Queue:  arrayqueue.New(),
		DB:     db,
		Logger: logger,
	}

	fx.New(
		fx.Supply(logger),
		fx.Supply(&w),
		fx.Provide(worker.NewAPI),
		fx.Provide(manager.NewManager),
		fx.Provide(manager.NewAPI),

		fx.Provide(NewBolt),

		fx.Provide(NewResty),
		fx.Provide(NewWorkers),

		fx.Provide(fx.Annotate(NewRoute, fx.As(new(http.Handler)))),
		fx.Invoke(server.RegisterRoutes),
		fx.Invoke(runTasks),
	).Run()
}

func NewRoute(logger *httplog.Logger, workerApi *worker.API, managerApi *manager.API) *chi.Mux {
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

	r.Route("/manager", func(r chi.Router) {
		r.Post("/tasks", managerApi.StartTaskHandler)
		r.Get("/tasks", managerApi.GetTasksHandler)
		r.Delete("/tasks/{taskID}", managerApi.StopTaskHandler)
	})

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

func NewResty(lifecycle fx.Lifecycle) *resty.Client {
	client := resty.New()
	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})
	return client
}

func NewWorkers() []string {
	return lo.Map(lo.Range(4), func(item int, index int) string {
		return "localhost:8080"
	})
}

func NewBolt(lifecycle fx.Lifecycle) (*bbolt.DB, error) {
	db, err := bbolt.Open("maestro.db", 0600, nil)
	if err != nil {

		return nil, err
	}
	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})
	return db, nil
}
