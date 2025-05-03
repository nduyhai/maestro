package worker

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
	"github.com/google/uuid"
	"github.com/nduyhai/maestro/internal/task"
	"log"
	"net/http"
)

type API struct {
	Worker *Worker
	logger *httplog.Logger
}

func NewAPI(worker *Worker, logger *httplog.Logger) *API {
	return &API{Worker: worker, logger: logger}

}

func (a *API) StartTaskHandler(w http.ResponseWriter, r *http.Request) {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()

	te := task.Event{}
	err := d.Decode(&te)
	if err != nil {
		msg := fmt.Sprintf("Error unmarshalling body: %v\n", err)
		a.logger.Info(msg)
		w.WriteHeader(400)
		e := ErrResponse{
			HTTPStatusCode: 400,
			Message:        msg,
		}
		_ = json.NewEncoder(w).Encode(e)
		return
	}

	a.Worker.AddTask(te.Task)
	a.logger.Info(fmt.Sprintf("Task added: %v", te.Task))
	w.WriteHeader(201)
	_ = json.NewEncoder(w).Encode(te.Task)
}

func (a *API) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(a.Worker.GetTasks())
}

func (a *API) StopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		a.logger.Info("No taskID passed in request.")
		w.WriteHeader(400)
		return
	}

	tID, _ := uuid.Parse(taskID)
	_, ok := a.Worker.DB[tID]
	if !ok {
		log.Printf("No task with ID %v found", tID)
		w.WriteHeader(404)
		return
	}
	taskToStop := a.Worker.DB[tID]
	taskCopy := *taskToStop
	taskCopy.State = task.Completed
	a.Worker.AddTask(taskCopy)

	a.logger.Info("Added task %v to stop container %v", taskToStop.ID, taskToStop.ContainerID)
	w.WriteHeader(204)
}

type ErrResponse struct {
	HTTPStatusCode int
	Message        string
}
