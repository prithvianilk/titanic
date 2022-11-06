package main

import (
	"net/http"
	"net/rpc"
	"time"

	"sync"

	"github.com/gin-gonic/gin"
)

type ServerState int

const (
	follower ServerState = iota
	leader
	candidate
)

const electionStartDuration = time.Millisecond * 150

type RaftApp struct {
	state         ServerState
	mu            sync.Mutex
	term          int
	lastResetTime time.Time
	clients       []rpc.Client
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
	}
}

func (app *RaftApp) startNewElection() {
	app.state = candidate
	app.term++

	replicaCount := len(app.clients) + 1
	votesReceived := 1
	for _, client := range app.clients {
		go func(client rpc.Client) {
			args := RequestVoteArgs{app.term}
			var reply RequestVoteReply
			err := client.Call("RequestVote", args, &reply)
			if err != nil {
				return
			} else if app.state != candidate {
				return
			} else if reply.term > app.term {
				app.state = follower
				app.term = reply.term
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
		}(client)
	}

	go app.startElectionTimer()
}

func main() {
	const addr = ":3530"
	kvmap := make(map[string]string)
	router := gin.Default()
	router.GET("/:key", func(c *gin.Context) {
		key := c.Param("key")
		value := kvmap[key]
		c.String(http.StatusOK, value)
	})
	router.PUT("/:key/:value", func(c *gin.Context) {
		key, value := c.Param("key"), c.Param("value")
		kvmap[key] = value
		c.String(http.StatusOK, "")
	})
	router.Run(addr)
}
