# A Raft Consensus Implementation in Go

![go](https://github.com/divtxt/raft/actions/workflows/go.yml/badge.svg)

An implementation of the Raft consensus protocol.
(<http://ramcloud.stanford.edu/raft.pdf>)

This package implements just the consensus module.
Other parts such as RPC and the Raft Log are interfaces that you must implement.

See [lockd](https://github.com/divtxt/lockd) for a example of how to use this module
(and implement the required interfaces).



## TODO


Later:

- [ ] Shutdown returns error and notifies instead of panic
- [x] ProcessRpcAppendEntries and ProcessRpcRequestVote return errors
- [ ] Leader commits a no-op entry at the start of its term (#8p4)
- [ ] Isolated server should not increment term (similar to #6p8)
- [x] Pluggable logging
- [ ] Log many more details e.g. leader, voters
- [ ] Add metrics & logging
- [ ] Expose raft details e.g. leader, term
- [x] Test for RPCs from senders not in cluster
- [ ] Review fatal errors to see if they can be non-fatal
- [ ] Assembling AppendEntries RPC should not block
- [x] Add errcheck to Travis build
- [ ] Add support for snapshotting & InstallSnapshot RPC
- [x] Leader uses AppendEntry instead of SetEntriesAfterIndex
- [ ] Live cluster membership changes
- [ ] Read-only nodes (replication)


Misc/Maybe:

- [ ] Tests have theoretical concurrency issues
- [ ] Servers check that they agree on cluster info
- [ ] Leader heartbeats with a majority before responding to read-only requests (#8p4)
- [ ] Election timeout based on ping times to bias selection of lower latency leader
