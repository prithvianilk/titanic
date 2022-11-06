package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"

	"sync"

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

const invalidVotedFor = ""

var addrs = []string{":3530", ":3630", ":6700"}

type Titanic struct {
	addr          string
	state         ServerState
	mu            sync.Mutex
	term          int
	lastResetTime time.Time
	kvmap         map[string]string
	votedFor      string
	log           []LogEntry
}

type LogEntry struct {
	instruction internal.KVPair
	term        int
}

type RequestVoteArgs struct {
	Term          int
	CandidateAddr string
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term     int
	LogEntry LogEntry
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

func NewRaftApp() *Titanic {
	var app Titanic
	app.addr = ":" + os.Args[1]
	app.term = 1
	app.state = follower
	app.kvmap = make(map[string]string)
	return &app
}

func (app *Titanic) startElectionTimer() {
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

func (app *Titanic) startNewElection() {
	app.state = candidate
	app.term++
	replicaCount := len(addrs)
	votesReceived := 1
	fmt.Println("new election")

	for _, addr := range addrs {
		go func(addr string) {
			if addr == app.addr {
				return
			}
			client, err := rpc.DialHTTP("tcp", internal.LocalHostAddr+addr)
			if err != nil {
				return
			}
			args := RequestVoteArgs{app.term, app.addr}
			var reply RequestVoteReply
			err = client.Call("Titanic.RequestVote", args, &reply)
			if err != nil {
				return
			} else if app.state != candidate {
				return
			} else if reply.Term > app.term {
				app.makeFollower(reply.Term)
				return
			} else if !reply.VoteGranted {
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

func (app *Titanic) makeLeader() {
	fmt.Println(app.addr + " was elected leader!")
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

func (app *Titanic) sendHeartbeats() {
	for _, addr := range addrs {
		client, err := rpc.DialHTTP("tcp", internal.LocalHostAddr+addr)
		if err != nil {
			continue
		}
		args := AppendEntriesArgs{}
		var reply AppendEntriesReply
		client.Call("Titanic.AppendEntries", args, &reply)
	}
}

func (app *Titanic) makeFollower(term int) {
	app.state = follower
	app.term = term
	app.votedFor = invalidVotedFor
	app.lastResetTime = time.Now()
}

func (app *Titanic) RequestVote(reqVoteArgs *RequestVoteArgs, reqVoteReply *RequestVoteReply) error {
	app.mu.Lock()
	defer app.mu.Unlock()
	if reqVoteArgs.Term > app.term {
		app.makeFollower(reqVoteArgs.Term)
	}

	if reqVoteArgs.Term == app.term && (app.votedFor == invalidVotedFor || app.votedFor == reqVoteArgs.CandidateAddr) {
		app.votedFor = reqVoteArgs.CandidateAddr
		reqVoteReply.VoteGranted = true
		app.lastResetTime = time.Now()
	} else {
		reqVoteReply.VoteGranted = false
	}

	reqVoteReply.Term = app.term
	return nil
}

func (app *Titanic) AppendEntries(appendEntriesArgs *AppendEntriesArgs, appendEntriesReply *AppendEntriesReply) error {
	app.mu.Lock()
	defer app.mu.Unlock()
	if appendEntriesArgs.Term > app.term {
		app.makeFollower(appendEntriesArgs.Term)
	}

	appendEntriesReply.Success = false

	if appendEntriesArgs.Term == app.term {
		if app.state != follower {
			app.makeFollower(app.term)
		}
		app.lastResetTime = time.Now()
		app.log = append(app.log, appendEntriesArgs.LogEntry)
	}

	appendEntriesReply.Term = app.term
	return nil
}

func (app *Titanic) Get(key string, value *string) {

}

func (app *Titanic) Put(kvPair internal.KVPair, success *bool) {

}

func main() {
	app := NewRaftApp()
	go app.startElectionTimer()
	startRPCServer(app)
}

func startRPCServer(app *Titanic) {
	rpc.Register(app)
	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", app.addr)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	http.Serve(listener, nil)
}
