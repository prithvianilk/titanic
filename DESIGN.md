# A simple raft implementation

# Rough ( and naive ) game plan

- Design a system that acts as a reliable kv store and acheives consensus on state via RAFT.
- End result: We can start up 5 processes and handle failure of upto 2 processes.
- Written in Go. (Nice std networking lib, concurrency primitives)
- Less than 500 LOC? No need to over complicate and add extra features (extended stuff from RAFT paper is not required. These are added in grey lines to the right of the paragraph).

## TASKS

- [ ] Leader Election (Section 5.2) and State management (Figure 2): Divya and Raghav
- [ ] Log Replication (Section 5.3) and Safety (Section 5.4): Anisha and Prithvi

## References

- [Paper](https://raft.github.io/raft.pdf)
- [Website w/ visualisation](https://raft.github.io/)
- [Another visualisation](http://thesecretlivesofdata.com/raft/)
- [Another impl in Go (last resort)](https://eli.thegreenplace.net/2020/implementing-raft-part-0-introduction/)

## Design

- The system we are building a reliable, key value store backed by a RAFT based leader election and consensus algorithm.
- Parts of the system:
  - The server can be requested via curl.
  - We need server code that can start and run a server on some port P.

## Client API

- GET
  `curl -X GET http://localhost:3530/x`
- PUT
  `curl -X PUT http://localhost:3530/x/1`
