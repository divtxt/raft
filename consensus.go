package raft

import (
	"sync/atomic"
	"time"
)

type TimeSettings struct {
	tickerDuration  time.Duration
	electionTimeout time.Duration
}

const (
	RPC_CHANNEL_BUFFER_SIZE = 100
)

type ConsensusModule struct {
	// -- External components - these fields meant to be immutable
	persistentState PersistentState
	log             Log
	rpcSender       RpcSender

	// -- Config - these fields meant to be immutable
	thisServerId ServerId
	allServerIds []ServerId
	timeSettings TimeSettings

	// -- State
	stopped             int32
	serverState         ServerState
	volatileState       VolatileState
	electionTimeoutTime time.Time

	// -- Channels
	rpcChannel chan bool
	ticker     *time.Ticker
}

// Initialize a consensus module with the given components and settings.
// A goroutine that handles consensus processing is created.
func NewConsensusModule(
	persistentState PersistentState,
	log Log,
	rpcSender RpcSender,
	thisServerId ServerId,
	allServerIds []ServerId,
	timeSettings TimeSettings,
) *ConsensusModule {
	// FIXME: param checks

	now := time.Now()
	rpcChannel := make(chan bool, RPC_CHANNEL_BUFFER_SIZE)
	ticker := time.NewTicker(timeSettings.tickerDuration)

	cm := &ConsensusModule{
		// -- External components
		persistentState,
		log,
		rpcSender,

		// -- Config
		thisServerId,
		allServerIds,
		timeSettings,

		// -- State
		0,
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,
		VolatileState{},
		now, // temporary value

		// -- Channels
		rpcChannel,
		ticker,
	}

	cm.resetElectionTimeoutTime()

	// Start the go routine
	go cm.processor()

	return cm
}

// Check if the goroutine is shutdown
func (cm *ConsensusModule) IsStopped() bool {
	return atomic.LoadInt32(&cm.stopped) != 0
}

// Get the current server state
func (cm *ConsensusModule) GetServerState() ServerState {
	return ServerState(atomic.LoadUint32((*uint32)(&cm.serverState)))
}

// Process the given RPC
func (cm *ConsensusModule) ProcessRpc(appendEntries AppendEntries) (AppendEntriesReply, error) {
	success, err := cm._processRpc_AppendEntries(appendEntries)
	return AppendEntriesReply{cm.persistentState.GetCurrentTerm(), success}, err
}

// Stop the consensus module. Stops the goroutine that does the
// processing and prevents any further
func (cm *ConsensusModule) StopAsync() {
	close(cm.rpcChannel) // atomic & will panic if already closed
}

// -- protected methods

// Set the current server state
func (cm *ConsensusModule) setServerState(serverState ServerState) {
	atomic.StoreUint32((*uint32)(&cm.serverState), (uint32)(serverState))
}

func (cm *ConsensusModule) resetElectionTimeoutTime() {
	cm.electionTimeoutTime = time.Now().Add(cm.timeSettings.electionTimeout)
}

func (cm *ConsensusModule) processor() {
	defer func() {
		// Mark the server as shutdown
		atomic.StoreInt32(&cm.stopped, 1)
	}()

loop:
	for {
		select {
		case _, ok := <-cm.rpcChannel:
			if !ok {
				// WARN: rpc channel closed - exiting processor()
				break loop
			}
			// FIXME: implement rpc receive here!
		case now, ok := <-cm.ticker.C:
			if !ok {
				// FATAL: ticker channel closed - exiting processor()
				// FIXME: better handling than panic()
				panic("ticker channel closed - exiting processor()")
				break loop
			}
			cm.tick(now)
		}
	}
}

func (cm *ConsensusModule) tick(now time.Time) {

	switch cm.GetServerState() {
	case FOLLOWER:
		// #5.2-p1s5: If a follower receives no communication over a period
		// of time called the election timeout, then it assumes there is no
		// viable leader and begins an election to choose a new leader.
		if now.After(cm.electionTimeoutTime) {
			// #5.2-p2s1: To begin an election, a follower increments its
			// current term and transitions to candidate state.
			// #5.2-p2s2: It then votes for itself and issues RequestVote RPCs
			// in parallel to each of the other servers in the cluster.
			newTerm := cm.persistentState.GetCurrentTerm() + 1
			cm.setServerState(CANDIDATE)
			cm.persistentState.SetCurrentTermAndVotedFor(newTerm, cm.thisServerId)
			lastLogIndex := cm.log.getIndexOfLastEntry()
			var lastLogTerm TermNo
			if lastLogIndex > 0 {
				lastLogTerm = cm.log.getTermAtIndex(lastLogIndex)
			} else {
				lastLogTerm = 0
			}
			for _, serverId := range cm.allServerIds {
				rpcRequestVote := &RpcRequestVote{newTerm, lastLogIndex, lastLogTerm}
				cm.rpcSender.SendAsync(serverId, rpcRequestVote)
			}
		}
	case CANDIDATE:
	case LEADER:
	default:
	}

}
