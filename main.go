package main

import (
	"fmt"
	"github.com/docker/docker/client"
	"github.com/emirpasic/gods/queues/arrayqueue"
	"log/slog"
	"net/http"
	"time"

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
	t := task.Task{
		ID:    uuid.New(),
		Name:  "postgres-container-02",
		State: task.Scheduled,
		Image: "postgres:latest",
	}

	// first time the worker will see the task
	fmt.Println("starting task")
	w.AddTask(t)
	result := w.RunTask()
	if result.Error != nil {
		panic(result.Error)
	}

	t.ContainerID = result.ContainerID
	fmt.Printf("task %s is running in container %s\n", t.ID, t.ContainerID)
	fmt.Println("Sleepy time")
	time.Sleep(time.Second * 30)
	fmt.Printf("stopping task %s\n", t.ID)
	t.State = task.Completed
	w.AddTask(t)
	result = w.RunTask()
	if result.Error != nil {
		panic(result.Error)
	}

	fx.New(
		fx.Provide(NewLogger),
		fx.Provide(fx.Annotate(NewRoute, fx.As(new(http.Handler)))),
		fx.Invoke(server.RegisterRoutes),
	).Run()
}

func createContainer() (*task.Docker, *task.DockerResult) {
	c := task.Config{
		Name:  "postgres-container-01",
		Image: "postgres:latest",
		Env: []string{
			"POSTGRES_USER=maestro",
			"POSTGRES_PASSWORD=thesecret",
		},
	}

	dc, _ := client.NewClientWithOpts(client.FromEnv)
	d := task.Docker{
		Client: dc,
		Config: c,
	}

	result := d.Run()
	if result.Error != nil {
		fmt.Printf("%v\n", result.Error)
		return nil, nil
	}

	fmt.Printf(
		"Container %s is running with config %v\n", result.ContainerID, c)
	return &d, &result
}

func stopContainer(d *task.Docker, id string) *task.DockerResult {
	result := d.Stop(id)
	if result.Error != nil {
		fmt.Printf("%v\n", result.Error)
		return nil
	}

	fmt.Printf(
		"Container %s has been stopped and removed\n", result.ContainerID)
	return &result
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
