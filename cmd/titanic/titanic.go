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
	electionStartDuration   = time.Second * 10
	heartbeatRepeatDuration = time.Second * 1
	resetCheckTime          = time.Second * 5
)

const invalidVotedFor = ""

var addrs = []string{":3530", ":3630", ":6700"}

type Titanic struct {
	addr          string
	state         ServerState
	mu            sync.Mutex
	Term          int
	lastResetTime time.Time
	kvmap         map[string]string
	votedFor      string
	log           []LogEntry
}

type LogEntry struct {
	instruction internal.KVPair
	Term        int
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
	app.Term = 1
	app.state = follower
	app.kvmap = make(map[string]string)
	return &app
}

func (app *Titanic) startElectionTimer() {
	app.mu.Lock()
	currentTerm := app.Term
	app.mu.Unlock()

	ticker := time.NewTicker(resetCheckTime)
	defer ticker.Stop()
	for {
		<-ticker.C
		app.mu.Lock()

		if app.state == leader {
			app.mu.Unlock()
			return
		}

		hasTermChanged := app.Term != currentTerm
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
	fmt.Println("Starting new election")
	app.state = candidate
	app.Term++
	currentTerm := app.Term
	replicaCount := len(addrs)
	app.votedFor = app.addr
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
			args := RequestVoteArgs{currentTerm, app.addr}
			var reply RequestVoteReply
			err = client.Call("Titanic.RequestVote", args, &reply)
			if err != nil {
				return
			}
			app.mu.Lock()
			defer app.mu.Unlock()

			if app.state != candidate {
				return
			} else if reply.Term > currentTerm {
				app.makeFollower(reply.Term)
				return
			} else if !reply.VoteGranted {
				return
			} else if reply.Term == currentTerm {
				votesReceived++
				shouldMakeLeader := (votesReceived * 2) > replicaCount
				if shouldMakeLeader {
					app.makeLeader()
					return
				}
			}
		}(addr)
	}

	go app.startElectionTimer()
}

func (app *Titanic) makeLeader() {
	fmt.Println(app.addr + " was elected leader! for Term " + fmt.Sprint(app.Term))
	app.state = leader
	go func() {
		ticker := time.NewTicker(heartbeatRepeatDuration)
		defer ticker.Stop()
		for {
			<-ticker.C
			fmt.Println("sending heartbeats")
			app.sendHeartbeats()

			app.mu.Lock()
			if app.state != leader {
				app.mu.Unlock()
				return
			}
			app.mu.Unlock()
		}
	}()
}

func (app *Titanic) sendHeartbeats() {
	app.mu.Lock()
	currentTerm := app.Term
	app.mu.Unlock()

	for _, addr := range addrs {
		client, err := rpc.DialHTTP("tcp", internal.LocalHostAddr+addr)
		if err != nil {
			continue
		}
		args := AppendEntriesArgs{
			Term: currentTerm,
		}
		go func(args AppendEntriesArgs) {
			var reply AppendEntriesReply
			err := client.Call("Titanic.AppendEntries", args, &reply)
			if err != nil {
				return
			}
			app.mu.Lock()
			defer app.mu.Unlock()
			if reply.Term > currentTerm {
				app.makeFollower(reply.Term)
				return
			}
		}(args)
	}
}

func (app *Titanic) makeFollower(Term int) {
	if app.state == leader {
		return
	}
	fmt.Printf("making follower, current state: %v\n", app.state)
	app.state = follower
	app.Term = Term
	app.votedFor = invalidVotedFor
	app.lastResetTime = time.Now()
	go app.startElectionTimer()
}

func (app *Titanic) RequestVote(reqVoteArgs *RequestVoteArgs, reqVoteReply *RequestVoteReply) error {
	app.mu.Lock()
	defer app.mu.Unlock()
	if reqVoteArgs.Term > app.Term {
		app.makeFollower(reqVoteArgs.Term)
	}

	if reqVoteArgs.Term == app.Term && (app.votedFor == invalidVotedFor || app.votedFor == reqVoteArgs.CandidateAddr) {
		app.votedFor = reqVoteArgs.CandidateAddr
		reqVoteReply.VoteGranted = true
		app.lastResetTime = time.Now()
	} else {
		reqVoteReply.VoteGranted = false
	}

	reqVoteReply.Term = app.Term
	return nil
}

func (app *Titanic) AppendEntries(appendEntriesArgs *AppendEntriesArgs, appendEntriesReply *AppendEntriesReply) error {
	app.mu.Lock()
	defer app.mu.Unlock()
	if appendEntriesArgs.Term > app.Term {
		app.makeFollower(appendEntriesArgs.Term)
	}

	appendEntriesReply.Success = false

	if appendEntriesArgs.Term == app.Term {
		if app.state != follower {
			app.makeFollower(app.Term)
		}
		app.lastResetTime = time.Now()
		appendEntriesReply.Success = true
		app.log = append(app.log, appendEntriesArgs.LogEntry)
	}

	appendEntriesReply.Term = app.Term
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
