package raft

import (
	"sync/atomic"
	"time"
)

type ConsensusModule struct {
	passiveConsensusModule *passiveConsensusModule

	// -- State - these fields may be accessed concurrently
	stopped int32

	// -- Channels
	rpcChannel chan rpcTuple
	ticker     *time.Ticker

	// -- Control
	stopSignal chan struct{}
	stopError  *atomic.Value
}

// Initialize a consensus module with the given components and settings.
// A goroutine that handles consensus processing is created.
// All parameters are required and cannot be nil.
// Server ids are check using ValidateServerIds().
// Time settings is checked using ValidateTimeSettings().
func NewConsensusModule(
	persistentState PersistentState,
	log Log,
	rpcSender RpcSender,
	thisServerId ServerId,
	peerServerIds []ServerId,
	timeSettings TimeSettings,
) *ConsensusModule {
	pcm, _ := newPassiveConsensusModule(
		persistentState,
		log,
		rpcSender,
		thisServerId,
		peerServerIds,
		timeSettings,
	)

	rpcChannel := make(chan rpcTuple, RPC_CHANNEL_BUFFER_SIZE)
	ticker := time.NewTicker(timeSettings.TickerDuration)

	cm := &ConsensusModule{
		pcm,

		// -- State
		0,

		// -- Channels
		rpcChannel,
		ticker,

		// -- Control
		make(chan struct{}),
		&atomic.Value{},
	}

	// Start the go routine
	go cm.processor()

	return cm
}

// Check if the goroutine is stopped.
func (cm *ConsensusModule) IsStopped() bool {
	return atomic.LoadInt32(&cm.stopped) != 0
}

// Stop the consensus module asynchronously.
// This will stop the goroutine that does the processing.
// Safe to call even if the goroutine has stopped.
// Will panic if called more than once.
func (cm *ConsensusModule) StopAsync() {
	close(cm.stopSignal)
}

// Get the panic error value that stopped the goroutine.
// The value will be nil if the goroutine is not stopped, or stopped
// without an error, or  panicked with a nil value.
func (cm *ConsensusModule) GetStopError() interface{} {
	return cm.stopError.Load()
}

// Get the current server state
func (cm *ConsensusModule) GetServerState() ServerState {
	return cm.passiveConsensusModule.getServerState()
}

// Process the given RPC message from the given peer asynchronously.
// See comments in rpctypes.go for an explanation of the messaging model.
// This method sends the rpc to the ConsensusModule's goroutine.
// Sending an unknown or unexpected rpc message will cause the
// ConsensusModule goroutine to panic and stop.
func (cm *ConsensusModule) ProcessRpcAsync(from ServerId, rpc interface{}) {
	select {
	case cm.rpcChannel <- rpcTuple{from, rpc}:
	default:
		// FIXME
		panic("oops! rpcChannel is full!")
	}
}

// -- protected methods

func (cm *ConsensusModule) processor() {
	defer func() {
		// Recover & save the panic reason
		if r := recover(); r != nil {
			cm.stopError.Store(r)
		}
		// Mark the server as stopped
		atomic.StoreInt32(&cm.stopped, 1)
		// Clean up things
		close(cm.rpcChannel)
		cm.ticker.Stop()
		// TODO: call stop event listener(s)
	}()

loop:
	for {
		select {
		case rpc, ok := <-cm.rpcChannel:
			if !ok {
				panic("FATAL: rpcChannel closed")
			}
			cm.passiveConsensusModule.rpc(rpc.from, rpc.rpc)
		case now, ok := <-cm.ticker.C:
			if !ok {
				panic("FATAL: ticker channel closed")
			}
			cm.passiveConsensusModule.tick(now)
		case <-cm.stopSignal:
			break loop
		}
	}
}
