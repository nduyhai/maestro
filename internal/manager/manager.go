package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"

	"github.com/nduyhai/maestro/internal/node"
	"github.com/nduyhai/maestro/internal/scheduler"

	"github.com/emirpasic/gods/queues/arrayqueue"
	"github.com/go-chi/httplog/v2"
	"github.com/nduyhai/maestro/internal/httpx"
	"github.com/samber/lo"
	"resty.dev/v3"

	"github.com/nduyhai/maestro/internal/task"

	"github.com/emirpasic/gods/queues"
	"github.com/google/uuid"
)

type Manager struct {
	Pending       queues.Queue
	TaskDB        map[uuid.UUID]*task.Task
	EventDB       map[uuid.UUID]*task.Event
	Workers       []string
	WorkerTaskMap map[string][]uuid.UUID
	TaskWorkerMap map[uuid.UUID]string
	LastWorker    int
	Client        *resty.Client
	Logger        *httplog.Logger

	WorkerNodes []*node.Node
	Scheduler   scheduler.Scheduler
}

func NewManager(logger *httplog.Logger, client *resty.Client, workers []string) *Manager {

	workerTaskMap := make(map[string][]uuid.UUID)
	var nodes []*node.Node
	for w := range workers {
		workerTaskMap[workers[w]] = []uuid.UUID{}
		nAPI := fmt.Sprintf("http://%v", workers[w])
		n := node.NewNode(workers[w], nAPI)
		nodes = append(nodes, n)
	}
	return &Manager{
		Pending:       arrayqueue.New(),
		TaskDB:        make(map[uuid.UUID]*task.Task),
		EventDB:       make(map[uuid.UUID]*task.Event),
		Workers:       workers,
		WorkerTaskMap: workerTaskMap,
		TaskWorkerMap: make(map[uuid.UUID]string),
		LastWorker:    0,
		Client:        client,
		Logger:        logger,
		Scheduler: &scheduler.RoundRobin{
			Name:       "roundrobin",
			LastWorker: 0,
		},
	}
}

func (m *Manager) SelectWorker(t task.Task) (*node.Node, error) {
	m.Logger.Info("I will select an appropriate worker")

	candidates := m.Scheduler.SelectCandidateNodes(t, m.WorkerNodes)
	if candidates == nil {
		msg := fmt.Sprintf("No available candidates match resource request for task %v", t.ID)
		err := errors.New(msg)
		return nil, err
	}
	scores := m.Scheduler.Score(t, candidates)
	selectedNode := m.Scheduler.Pick(scores, candidates)

	return selectedNode, nil
}

func (m *Manager) UpdateTasks() {
	m.Logger.Info("I will update tasks")
	for _, w := range m.Workers {
		m.Logger.Info("Checking worker %v for task updates", slog.Any("worker", w))
		url := fmt.Sprintf("http://%s/tasks", w)
		resp, err := m.Client.R().Get(url)
		if err != nil {
			m.Logger.Error("Error connecting to ", slog.Any("worker", w), slog.Any("err", err))
			continue
		}

		if resp.StatusCode() != http.StatusOK {
			m.Logger.Error("Error sending request", slog.Any("err", err))
			continue
		}

		d := json.NewDecoder(resp.Body)
		var tasks []*task.Task
		err = d.Decode(&tasks)
		if err != nil {
			m.Logger.Error("Error unmarshalling tasks", slog.Any("err", err))
			continue
		}
		for _, t := range tasks {
			m.Logger.Error("Attempting to update task", slog.Any("ID", t.ID))

			_, ok := m.TaskDB[t.ID]
			if !ok {
				m.Logger.Error("Task with ID not found", slog.Any("ID", t.ID))
				return
			}

			if m.TaskDB[t.ID].State != t.State {
				m.TaskDB[t.ID].State = t.State
			}

			m.TaskDB[t.ID].StartTime = t.StartTime
			m.TaskDB[t.ID].FinishTime = t.FinishTime
			m.TaskDB[t.ID].ContainerID = t.ContainerID
		}
	}
}

func (m *Manager) SendWork() {
	m.Logger.Info("I will send work to workers")
	if m.Pending.Size() > 0 {
		e, _ := m.Pending.Dequeue()
		te := e.(task.Event)
		t := te.Task
		m.Logger.Info("Pulled %v off pending queue", slog.Any("task", t))

		m.EventDB[te.ID] = &te

		t = te.Task
		w, err := m.SelectWorker(t)
		if err != nil {
			m.Logger.Error("Error selecting worker", slog.Any("err", err))
			return
		}

		m.WorkerTaskMap[w.Name] = append(m.WorkerTaskMap[w.Name], te.Task.ID)
		m.TaskWorkerMap[t.ID] = w.Name

		t.State = task.Scheduled
		m.TaskDB[t.ID] = &t
		data, err := json.Marshal(te)
		if err != nil {
			m.Logger.Info("Unable to marshal task object", slog.Any("task", t))
			return
		}
		url := fmt.Sprintf("http://%s/tasks", w.Name)
		resp, err := m.Client.R().SetBody(data).SetContentType("application/json").Post(url)
		if err != nil {
			m.Logger.Error("Error connecting to", slog.Any("worker", w), slog.Any("err", err))
			m.Pending.Enqueue(te)
			return
		}

		d := json.NewDecoder(resp.Body)
		if resp.StatusCode() != http.StatusCreated {
			e := httpx.ErrResponse{}
			err := d.Decode(&e)
			if err != nil {
				m.Logger.Error("Error decoding response", slog.Any("err", err))
				return
			}
			m.Logger.Info("Response error", slog.Any("statusCode", e.HTTPStatusCode), slog.Any("error", e))
			return
		}
		t = task.Task{}
		err = d.Decode(&t)
		if err != nil {
			m.Logger.Error("Error decoding response", slog.Any("err", err))
			return
		}
		m.Logger.Info("task ", slog.Any("task", t))
	} else {
		m.Logger.Info("No work in the queue")
	}

}
func (m *Manager) AddTask(te task.Event) {
	m.Pending.Enqueue(te)
}

func (m *Manager) GetTasks() []*task.Task {
	tasks, _ := lo.CoalesceSlice(slices.Collect(maps.Values(m.TaskDB)), []*task.Task{})
	return tasks

}

func (m *Manager) stopTask(worker string, taskID string) {
	url := fmt.Sprintf("http://%s/tasks/%s", worker, taskID)

	resp, err := m.Client.R().Delete(url)

	if err != nil {
		m.Logger.Error("Error stopping task", slog.Any("err", err))
		return
	}

	if resp.StatusCode() != http.StatusNoContent {
		m.Logger.Error("Error sending request stopping task", slog.Any("err", err))
		return
	}
	m.Logger.Info("task has been scheduled to be stopped", slog.Any("task", taskID))
}
