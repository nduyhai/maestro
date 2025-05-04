package worker

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/go-chi/httplog/v2"

	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/nduyhai/maestro/internal/task"

	"github.com/emirpasic/gods/queues"
	"github.com/google/uuid"
)

type Worker struct {
	Name      string
	Queue     queues.Queue
	DB        map[uuid.UUID]*task.Task
	TaskCount int
	Logger    *httplog.Logger
}

type Stats struct {
	Memory *mem.VirtualMemoryStat
	CPU    []cpu.InfoStat
	Disk   *disk.UsageStat
	Load   *load.AvgStat
}

func (w *Worker) CollectStats() Stats {
	memory, _ := mem.VirtualMemory()
	info, _ := cpu.Info()
	usage, _ := disk.Usage("/")
	avg, _ := load.Avg()
	return Stats{
		Memory: memory,
		CPU:    info,
		Disk:   usage,
		Load:   avg,
	}

}

func (w *Worker) RunTask() task.DockerResult {
	t, ok := w.Queue.Dequeue()
	if !ok {
		w.Logger.Error("no tasks in the queue")
		return task.DockerResult{Error: nil}
	}

	taskQueued, ok := t.(task.Task)
	if !ok {
		w.Logger.Error("error during task queue")
		return task.DockerResult{Error: nil}
	}
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
	w.Logger.Info("I will start a task")
	t.StartTime = time.Now().UTC()
	config := task.NewConfig(&t)
	d := task.NewDocker(config, w.Logger)
	result := d.Run()
	if result.Error != nil {
		w.Logger.Error("Err running task", slog.Any("error", result.Error), slog.Any("taskID", t.ID))
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
	w.Logger.Info("I will stop a task")
	config := task.NewConfig(&t)
	d := task.NewDocker(config, w.Logger)

	result := d.Stop(t.ContainerID)
	if result.Error != nil {
		w.Logger.Error("Error stopping container", slog.Any("ContainerID", t.ContainerID), slog.Any("error", result.Error))
	}
	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.DB[t.ID] = &t
	d.Logger.Info("Stopped task", slog.Any("ContainerID", t.ContainerID), slog.Any("taskID", t.ID))

	return result
}

func (w *Worker) AddTask(t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) GetTasks() []*task.Task {
	tasks, _ := lo.CoalesceSlice(slices.Collect(maps.Values(w.DB)), []*task.Task{})
	return tasks
}

func (w *Worker) InspectTask(t task.Task) task.DockerInspectResponse {
	config := task.NewConfig(&t)
	d := task.NewDocker(config, w.Logger)
	return d.Inspect(t.ContainerID)
}

func (w *Worker) UpdateTasks() {
	for {
		log.Println("Checking status of tasks")
		w.updateTasks()
		log.Println("Task updates completed")
		log.Println("Sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) updateTasks() {
	for id, t := range w.DB {
		if t.State == task.Running {
			resp := w.InspectTask(*t)
			if resp.Error != nil {
				fmt.Printf("ERROR: %v\n", resp.Error)
			}

			if resp.Container == nil {
				log.Printf("No container for running task %s\n", id)
				w.DB[id].State = task.Failed
			}

			if resp.Container.State.Status == "exited" {
				log.Printf("Container for task %s in non-running state %s",
					id, resp.Container.State.Status)
				w.DB[id].State = task.Failed
			}

			w.DB[id].HostPorts = resp.Container.NetworkSettings.NetworkSettingsBase.Ports
		}
	}
}
