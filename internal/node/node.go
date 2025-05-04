package node

type Node struct {
	Name            string
	IP              string
	Cores           int
	Memory          int
	MemoryAllocated int
	Disk            int
	DiskAllocated   int
	Role            string
	TaskCount       int
}

func NewNode(name string, IP string) *Node {
	return &Node{Name: name, IP: IP}
}
