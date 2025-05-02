package main

import (
	"fmt"
	"time"

	"github.com/emirpasic/gods/queues/arrayqueue"
	"github.com/google/uuid"
	"github.com/nduyhai/maestro/manager"
	"github.com/nduyhai/maestro/node"
	"github.com/nduyhai/maestro/task"
	"github.com/nduyhai/maestro/worker"
)

func main() {
	fmt.Println("Hello World")

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
}
