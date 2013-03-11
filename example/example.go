package main

import (
	"github.com/dforsyth/donut"
	"launchpad.net/gozk/zookeeper"
	"log"
	"math"
	"os"
	"time"
)

type ExampleBalancer struct {
	c *donut.Cluster
}

func (b *ExampleBalancer) Init(c *donut.Cluster) {
	b.c = c
}

func (b *ExampleBalancer) maxOwn() int {
	bal := float64(len(b.c.Tasks())) / float64(len(b.c.Nodes()))
	return int(math.Ceil(bal))
}

func (b *ExampleBalancer) CanClaim() bool {
	if len(b.c.Tasks()) <= 1 {
		return len(b.c.Owned()) < len(b.c.Tasks())
	}
	return len(b.c.Owned()) < b.maxOwn()
}

func (b *ExampleBalancer) HandoffList() (handoff []string) {
	owned := b.c.Owned()
	give := len(owned) - b.maxOwn()
	if give <= 0 {
		return
	}

	for _, own := range owned {
		handoff = append(handoff, own)
		if len(handoff) == give {
			break
		}
	}
	log.Printf("handoff list length is %d", len(handoff))
	return
}

type ExampleListener struct {
	c                *donut.Cluster
	killers          map[string]chan byte
	config           *donut.Config
	dings            int
	jobs             int
	apiHost, apiPort string
}

func (l *ExampleListener) OnJoin(zk *zookeeper.Conn) {
	log.Println("Joining!")
	// Create an assigned task for this node as soon as it joins...
	data := make(map[string]interface{})
	// assign this task specifically to this node
	// data["example"] = l.config.NodeId
	donut.CreateTask("example", zk, l.config, "task-"+l.config.NodeId, data)
	go func() {
		// only do this task for 5 seconds
		time.Sleep(60 * time.Second)
		donut.CompleteTask("example", zk, l.config, "task-"+l.config.NodeId)
	}()
}

func (l *ExampleListener) OnLeave() {
	log.Println("Leaving!")
}

func (l *ExampleListener) StartTask(taskId string, data map[string]interface{}) {
	log.Printf("Starting task %s!", taskId)
	l.killers[taskId] = make(chan byte)
	l.jobs++
	for {
		select {
		case <-l.killers[taskId]:
			log.Printf("%s killed!", taskId)
			return
		default:
		}
		log.Printf("ding %s!", taskId)
		l.dings++
		time.Sleep(time.Second)
	}
}

func (l *ExampleListener) EndTask(taskId string) {
	log.Printf("Ending task %s!", taskId)
	l.killers[taskId] <- 0
	l.jobs--
}

func (l *ExampleListener) Information() map[string]interface{} {
	information := make(map[string]interface{})
	information["node_id"] = l.config.NodeId
	information["completed_iterations"] = l.dings
	information["jobs"] = l.jobs
	return information
}

func (l *ExampleListener) APIHost() string {
	return l.apiHost
}

func (l *ExampleListener) APIPort() string {
	return l.apiPort
}

func main() {
	if len(os.Args) < 4 {
		panic("usage: example <nodeId> <apiHost> <apiPort>")
	}
	listener := &ExampleListener{
		killers: make(map[string]chan byte),
		apiHost: os.Args[2],
		apiPort: os.Args[3],
	}
	config := donut.NewConfig()

	config.Servers = "localhost:2181"
	config.NodeId = "node-" + os.Args[1]
	log.Printf("node id is %s", config.NodeId)

	c := donut.NewCluster("example", config, &ExampleBalancer{}, listener)
	listener.c = c
	listener.config = config
	c.Join()
	<-make(chan byte)
}
