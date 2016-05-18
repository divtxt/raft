# A Raft Consensus Implementation in Go

[![Build Status](https://travis-ci.org/divtxt/raft.svg?branch=master)](https://travis-ci.org/divtxt/raft)

An implementation of the Raft consensus protocol.
(<http://ramcloud.stanford.edu/raft.pdf>)

This package implements just the consensus module.
Other parts such as RPC and the Raft Log are interfaces that you must implement.

See [lockd](https://github.com/divtxt/lockd) for a example of how to use this module
(and implement the required interfaces).



## TODO


Basics:

- [x] Rename repo from `raft-consensus` to `raft`
- [x] Change error handling from `panic()` to returning `error`.
- [x] Move Log interface to interfaces.go
- [x] Rename Log to LogAndStateMachine
- [ ] Support for single-node cluster
- [ ] Test for RPCs from senders not in cluster
- [ ] Fix TODOs
- [ ] Code review & cleanup
- [ ] Simplify/shorten names
- [ ] Expose raft details e.g. leader, term
- [ ] Add metrics & logging
- [ ] Assembling AppendEntries RPC should not block
- [ ] Isolated server should not increment term (similar to #6p8)

Misc/Maybe:

- [ ] Tests have theoretical concurrency issues
- [ ] Change to actor model / library
- [ ] Use custom FIFO instead of fixed size channel (perhaps metrics first?)
- [ ] Servers check that they agree on cluster info
- [ ] Election timeout based on ping times to bias selection of lower latency leader
- [ ] Make in-memory Log & PersistentState implementations public?

Later:

- [ ] Add errcheck to Travis build
- [ ] Add support for snapshotting & InstallSnapshot RPC
- [ ] Split SetEntriesAfterIndex for leader and follower
- [ ] Allow leader LogAndStateMachine to reject new log entry
- [ ] Documentation
- [ ] Live cluster membership changes
- [ ] Read-only nodes (replication)
