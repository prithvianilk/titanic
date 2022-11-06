package main

import (
	"net/http"
	"net/rpc"
	"os"
	"time"

	"sync"

	"github.com/gin-gonic/gin"
	"github.com/prithvianilk/titanic/internal"
)

type ServerState int

const (
	follower ServerState = iota
	leader
	candidate
)

const (
	electionStartDuration   = time.Millisecond * 150
	heartbeatRepeatDuration = time.Millisecond * 50
)

var addrs = []string{":3530", ":3630", ":6700"}

type RaftApp struct {
	addr          string
	state         ServerState
	mu            sync.Mutex
	term          int
	lastResetTime time.Time
	kvmap         map[string]string
}

type RequestVoteArgs struct {
	term int
}

type RequestVoteReply struct {
	term        int
	voteGranted bool
}

func (app *RaftApp) startElectionTimer() {
	app.mu.Lock()
	app.lastResetTime = time.Now()
	currentTerm := app.term
	app.mu.Unlock()
	resetCheckTime := time.Millisecond * 10
	ticker := time.NewTicker(resetCheckTime)
	for {
		<-ticker.C
		app.mu.Lock()

		if app.state == leader {
			app.mu.Unlock()
			return
		}

		hasTermChanged := app.term != currentTerm
		if hasTermChanged {
			app.mu.Unlock()
			return
		}

		timeSinceLastReset := time.Since(app.lastResetTime)
		shouldStartElection := timeSinceLastReset >= electionStartDuration
		if shouldStartElection {
			app.startNewElection()
			app.mu.Unlock()
			return
		}

		app.mu.Unlock()
	}
}

func (app *RaftApp) startNewElection() {
	app.state = candidate
	app.term++
	replicaCount := len(addrs)
	votesReceived := 1

	for _, addr := range addrs {
		go func(addr string) {
			if addr == app.addr {
				return
			}
			client, err := rpc.DialHTTP("tcp", internal.LocalHostAddr+addr)
			if err != nil {
				return
			}
			args := RequestVoteArgs{app.term}
			var reply RequestVoteReply
			err = client.Call("RequestVote", args, &reply)
			if err != nil {
				return
			} else if app.state != candidate {
				return
			} else if reply.term > app.term {
				app.makeFollower(reply.term)
				return
			} else if !reply.voteGranted {
				return
			}
			votesReceived++
			shouldMakeLeader := votesReceived*2 >= replicaCount
			if shouldMakeLeader {
				app.makeLeader()
				return
			}
		}(addr)
	}

	go app.startElectionTimer()
}

func (app *RaftApp) makeLeader() {
	app.state = leader
	go func() {
		ticker := time.NewTicker(heartbeatRepeatDuration)
		defer ticker.Stop()
		for {
			<-ticker.C
			app.sendHeartbeats()
		}
	}()
}

func (app *RaftApp) sendHeartbeats() {
	for _, addr := range addrs {
		client, err := rpc.DialHTTP("tcp", internal.LocalHostAddr+addr)
		if err != nil {
			continue
		}
		args := AppendEntriesArgs{}
		var reply AppendEntriesReply
		client.Call("AppendEntries", args, &reply)
	}
}

func (app *RaftApp) makeFollower(term int) {
	app.state = follower
	app.term = term
	app.lastResetTime = time.Now()
}

func main() {
	app := NewRaftApp()
	go app.startElectionTimer()
	app.startAPIServer()
}

func (app *RaftApp) Get(key string, value *string) {

}

func (app *RaftApp) Put(kvPair internal.KVPair, success *bool) {

}

func (app *RaftApp) startAPIServer() {
	router := gin.Default()
	router.GET("/:key", func(c *gin.Context) {
		key := c.Param("key")
		value := app.kvmap[key]
		c.String(http.StatusOK, value)
	})
	router.PUT("/:key/:value", func(c *gin.Context) {
		key, value := c.Param("key"), c.Param("value")
		app.kvmap[key] = value
		c.String(http.StatusOK, "")
	})
	router.Run(app.addr)
}

func NewRaftApp() *RaftApp {
	var app RaftApp
	app.addr = ":" + os.Args[1]
	app.term = 1
	app.state = follower
	app.kvmap = make(map[string]string)
	return &app
}
