package task

import (
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type State int

const (
	Pending State = iota
	Scheduled
	Running
	Completed
	Failed
)

type Task struct {
	ID            uuid.UUID
	Name          string
	State         State
	Image         string
	Memory        int
	Disk          int
	ExposedPorts  nat.PortMap
	BindingPorts  map[string]string
	RestartPolicy string
	StartTime     time.Time
	EndTime       time.Time
}

type Event struct {
	ID        uuid.UUID
	State     State
	Timestamp time.Time
	Task      Task
}
