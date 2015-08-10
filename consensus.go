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
	// External things & config
	persistentState PersistentState
	log             Log
	timeSettings    TimeSettings

	// State
	stopped             int32
	serverState         ServerState
	volatileState       VolatileState
	electionTimeoutTime time.Time

	// Channels
	rpcChannel chan bool
	ticker     *time.Ticker
}

// Initialize a consensus module wrapping the given persistent state,
// log implementation, and raft time settings.
// A goroutine that handles consensus processing is created.
func NewConsensusModule(
	persistentState PersistentState,
	log Log,
	timeSettings TimeSettings,
) *ConsensusModule {
	now := time.Now()
	rpcChannel := make(chan bool, RPC_CHANNEL_BUFFER_SIZE)
	ticker := time.NewTicker(timeSettings.tickerDuration)

	cm := &ConsensusModule{
		persistentState,
		log,
		timeSettings,
		0,
		// #5.2-p1s2: When servers start up, they begin as followers
		FOLLOWER,
		VolatileState{},
		now, // temporary value
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
	stopped := atomic.LoadInt32(&cm.stopped)
	return stopped != 0
}

// Get the current server state
func (cm *ConsensusModule) GetServerState() ServerState {
	// FIXME: thread safety
	return cm.serverState
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

func (cm *ConsensusModule) resetElectionTimeoutTime() {
	cm.electionTimeoutTime = time.Now().Add(cm.timeSettings.electionTimeout)
}

func (cm *ConsensusModule) processor() {
	defer func() {
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

	switch cm.serverState {
	case FOLLOWER:
		// #5.2-p1s5: If a follower receives no communication over a period
		// of time called the election timeout, then it assumes there is no
		// viable leader and begins an election to choose a new leader.
		if now.After(cm.electionTimeoutTime) {
			// #5.2-p2s1: To begin an election, a follower increments its
			// current term and transitions to candidate state.
			newTerm := cm.persistentState.GetCurrentTerm() + 1
			cm.persistentState.SetCurrentTerm(newTerm)
			cm.serverState = CANDIDATE
		}
	case CANDIDATE:
	case LEADER:
	default:
	}

}
