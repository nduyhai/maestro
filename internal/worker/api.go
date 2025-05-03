package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
	"github.com/google/uuid"
	"github.com/nduyhai/maestro/internal/task"
)

type API struct {
	Worker *Worker
	logger *httplog.Logger
}

func NewAPI(worker *Worker, logger *httplog.Logger) *API {
	return &API{Worker: worker, logger: logger}

}
func (a *API) CollectStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(a.Worker.CollectStats())
}

func (a *API) StartTaskHandler(w http.ResponseWriter, r *http.Request) {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()

	te := task.Event{}
	err := d.Decode(&te)
	if err != nil {
		msg := fmt.Sprintf("Error unmarshalling body: %v\n", err)
		a.logger.Info(msg)
		w.WriteHeader(http.StatusBadRequest)
		e := ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        msg,
		}
		_ = json.NewEncoder(w).Encode(e)
		return
	}

	a.Worker.AddTask(te.Task)
	a.logger.Info(fmt.Sprintf("Task added: %v", te.Task))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(te.Task)
}

func (a *API) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(a.Worker.GetTasks())
}

func (a *API) StopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		a.logger.Info("No taskID passed in request.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tID, _ := uuid.Parse(taskID)
	_, ok := a.Worker.DB[tID]
	if !ok {
		log.Printf("No task with ID %v found", tID)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	taskToStop := a.Worker.DB[tID]
	taskCopy := *taskToStop
	taskCopy.State = task.Completed
	a.Worker.AddTask(taskCopy)

	a.logger.Info("Added task to stop container", slog.Any("ID", taskToStop.ID), slog.Any("ContainerID", taskToStop.ContainerID))
	w.WriteHeader(http.StatusNoContent)
}

type ErrResponse struct {
	HTTPStatusCode int
	Message        string
}
