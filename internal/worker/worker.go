package worker

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nduyhai/maestro/internal/task"

	"github.com/emirpasic/gods/queues"
	"github.com/google/uuid"
)

type Worker struct {
	Name      string
	Queue     queues.Queue
	DB        map[uuid.UUID]*task.Task
	TaskCount int
}

func (w *Worker) CollectStats() {
	fmt.Println("I will collect stats")
}

func (w *Worker) RunTask() task.DockerResult {
	t, ok := w.Queue.Dequeue()
	if !ok {
		log.Println("No tasks in the queue")
		return task.DockerResult{Error: nil}
	}

	taskQueued := t.(task.Task)
	taskPersisted := w.DB[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = &taskQueued
		w.DB[taskQueued.ID] = &taskQueued
	}

	var result task.DockerResult
	if task.ValidStateTransition(
		taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			result = w.StartTask(taskQueued)
		case task.Completed:
			result = w.StopTask(taskQueued)
		default:
			result.Error = errors.New("we should not get here")
		}
	} else {
		err := fmt.Errorf("invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
		result.Error = err
	}
	return result
}

func (w *Worker) StartTask(t task.Task) task.DockerResult {
	fmt.Println("I will start a task")
	t.StartTime = time.Now().UTC()
	config := task.NewConfig(&t)
	d := task.NewDocker(config)
	result := d.Run()
	if result.Error != nil {
		log.Printf("Err running task %v: %v\n", t.ID, result.Error)
		t.State = task.Failed
		w.DB[t.ID] = &t
		return result
	}

	t.ContainerID = result.ContainerID
	t.State = task.Running
	w.DB[t.ID] = &t

	return result
}

func (w *Worker) StopTask(t task.Task) task.DockerResult {
	fmt.Println("I will stop a task")
	config := task.NewConfig(&t)
	d := task.NewDocker(config)

	result := d.Stop(t.ContainerID)
	if result.Error != nil {
		log.Printf("Error stopping container %v: %v\n", t.ContainerID, result.Error)
	}
	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.DB[t.ID] = &t
	log.Printf("Stopped and removed container %v for task %v\n",
		t.ContainerID, t.ID)

	return result
}

func (w *Worker) AddTask(t task.Task) {
	w.Queue.Enqueue(t)
}
