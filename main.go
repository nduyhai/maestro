package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"
	"github.com/nduyhai/maestro/internal/manager"
	"github.com/nduyhai/maestro/internal/node"
	"github.com/nduyhai/maestro/internal/server"
	"github.com/nduyhai/maestro/internal/task"
	"github.com/nduyhai/maestro/internal/worker"
	"go.uber.org/fx"

	"github.com/emirpasic/gods/queues/arrayqueue"
	"github.com/google/uuid"
)

func main() {

	t := task.Task{
		ID:     uuid.New(),
		Name:   "Task-1",
		State:  task.Pending,
		Image:  "Image-1",
		Memory: 1024,
		Disk:   1,
	}

	te := task.Event{
		ID:        uuid.New(),
		State:     task.Pending,
		Timestamp: time.Now(),
		Task:      t,
	}

	fmt.Printf("task: %v\n", t)
	fmt.Printf("task event: %v\n", te)

	w := worker.Worker{
		Name:  "worker-1",
		Queue: arrayqueue.New(),
		DB:    make(map[uuid.UUID]*task.Task),
	}
	fmt.Printf("worker: %v\n", w)
	w.CollectStats()
	w.RunTask()
	w.StartTask()
	w.StopTask()

	w.StopTask()

	m := manager.Manager{
		Pending: arrayqueue.New(),
		TaskDB:  make(map[string][]*task.Task),
		EventDB: make(map[string][]*task.Event),
		Workers: []string{w.Name},
	}

	fmt.Printf("manager: %v\n", m)
	m.SelectWorker()
	m.UpdateTasks()
	m.SendWork()

	n := node.Node{
		Name:   "Node-1",
		Ip:     "192.168.1.1",
		Cores:  4,
		Memory: 1024,
		Disk:   25,
		Role:   "worker",
	}

	fmt.Printf("node: %v\n", n)

	fx.New(
		fx.Provide(NewLogger),
		fx.Provide(fx.Annotate(NewRoute, fx.As(new(http.Handler)))),
		fx.Invoke(server.RegisterRoutes),
	).Run()
}

func NewRoute(logger *httplog.Logger) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(httplog.RequestLogger(logger))
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/greeting", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("welcome"))
	})

	return r
}

func NewLogger() *httplog.Logger {
	return httplog.NewLogger("maestro", httplog.Options{
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
