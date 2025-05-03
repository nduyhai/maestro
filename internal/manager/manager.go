package manager

import (
	"fmt"

	"github.com/nduyhai/maestro/internal/task"

	"github.com/emirpasic/gods/queues"
	"github.com/google/uuid"
)

type Manager struct {
	Pending       queues.Queue
	TaskDB        map[string][]*task.Task
	EventDB       map[string][]*task.Event
	Workers       []string
	WorkerTaskMap map[string]uuid.UUID
	TaskWorkerMap map[string]uuid.UUID
}

func (m *Manager) SelectWorker() {
	fmt.Println("I will select an appropriate worker")
}

func (m *Manager) UpdateTasks() {
	fmt.Println("I will update tasks")
}

func (m *Manager) SendWork() {
	fmt.Println("I will send work to workers")
}
