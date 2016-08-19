# A Raft Consensus Implementation in Go

[![Build Status](https://travis-ci.org/divtxt/raft.svg?branch=master)](https://travis-ci.org/divtxt/raft)

An implementation of the Raft consensus protocol.
(<http://ramcloud.stanford.edu/raft.pdf>)

This package implements just the consensus module.
Other parts such as RPC and the Raft Log are interfaces that you must implement.

See [lockd](https://github.com/divtxt/lockd) for a example of how to use this module
(and implement the required interfaces).



## TODO


Later:

- [ ] Isolated server should not increment term (similar to #6p8)
- [ ] Add metrics & logging
- [ ] Expose raft details e.g. leader, term
- [ ] Test for RPCs from senders not in cluster
- [ ] Review fatal errors to see if they can be non-fatal
- [ ] Assembling AppendEntries RPC should not block
- [ ] Add errcheck to Travis build
- [ ] Add support for snapshotting & InstallSnapshot RPC
- [x] Leader uses AppendEntry instead of SetEntriesAfterIndex
- [x] AppendCommandAsync takes unserialized command
- [x] AppendEntry can provide a result value and can reject command
- [ ] Documentation
- [ ] Live cluster membership changes
- [ ] Read-only nodes (replication)

Misc/Maybe:

- [ ] Tests have theoretical concurrency issues
- [ ] Change to actor model / library
- [ ] Use custom FIFO instead of fixed size channel (perhaps metrics first?)
- [ ] Servers check that they agree on cluster info
- [ ] Election timeout based on ping times to bias selection of lower latency leader
