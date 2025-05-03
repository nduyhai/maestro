package manager

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
	"github.com/google/uuid"
	"github.com/nduyhai/maestro/internal/httpx"
	"github.com/nduyhai/maestro/internal/task"
)

type API struct {
	Manager *Manager
	Logger  *httplog.Logger
}

func NewAPI(manager *Manager, logger *httplog.Logger) *API {
	return &API{Manager: manager, Logger: logger}
}

func (a *API) StartTaskHandler(w http.ResponseWriter, r *http.Request) {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()

	te := task.Event{}
	err := d.Decode(&te)
	if err != nil {
		msg := fmt.Sprintf("Error unmarshalling body: %v", err)
		a.Logger.Info(msg)
		w.WriteHeader(http.StatusBadRequest)
		e := httpx.ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Message:        msg,
		}
		_ = json.NewEncoder(w).Encode(e)
		return
	}

	a.Manager.AddTask(te)
	a.Manager.SendWork()
	a.Logger.Info(fmt.Sprintf("Task added: %v", te.Task))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(te.Task)
}

func (a *API) StopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		a.Logger.Info("No taskID passed in request.")
		w.WriteHeader(http.StatusNotFound)
	}

	tID, _ := uuid.Parse(taskID)
	taskToStop, ok := a.Manager.TaskDB[tID]
	if !ok {
		a.Logger.Info("No task with ID found", slog.String("taskID", taskID))
		w.WriteHeader(http.StatusNotFound)
	}

	te := task.Event{
		ID:        uuid.New(),
		State:     task.Completed,
		Timestamp: time.Now(),
	}

	taskCopy := *taskToStop
	taskCopy.State = task.Completed
	te.Task = taskCopy
	a.Manager.AddTask(te)

	a.Logger.Info("Added task event to stop task", slog.Any("tID", te.ID), slog.Any("ID", taskToStop.ID))
	w.WriteHeader(http.StatusOK)
}

func (a *API) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(a.Manager.GetTasks())
}
