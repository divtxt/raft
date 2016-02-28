# Raft consensus protocol

[![Build Status](https://travis-ci.org/divtxt/raft-consensus.svg?branch=master)](https://travis-ci.org/divtxt/raft-consensus)

An implementation of the Raft consensus protocol.
(<http://ramcloud.stanford.edu/raft.pdf>)

This package implements just the consensus module.
Other parts such as RPC and the Raft Log are interfaces that you must implement.

See [lockd](https://github.com/divtxt/lockd) for a example of how to use this module
(and implement the required interfaces).



## TODO


Basics:

- [ ] Change error handling from `panic()` to returning `error`.
- [ ] Expose raft details e.g. leader, term
- [ ] Add metrics & logging
- [ ] Isolated server should not increment term (similar to #6p8)

Misc/Maybe:

- [ ] Tests have theoretical concurrency issues
- [ ] Change to actor model / library
- [ ] Use custom FIFO instead of fixed size channel (perhaps metrics first?)
- [ ] Servers check that they agree on cluster info
- [ ] Election timeout based on ping times to bias selection of lower latency leader

Later:

- [ ] Add support for snapshotting & InstallSnapshot RPC
- [ ] Documentation
- [ ] Live cluster membership changes
- [ ] Read-only nodes (replication)

