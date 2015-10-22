package raft

import "fmt"

// Quorum formula
func QuorumSizeForClusterSize(clusterSize uint) uint {
	return (clusterSize / 2) + 1
}

// Server ids validation.
// Returns empty string if all validations pass
// Server ids must be distinct non-empty strings.
// 'peerServerIds' must contain at least 1 element and
// must not include 'thisServerId'.
func ValidateServerIds(
	thisServerId ServerId,
	peerServerIds []ServerId,
) string {

	if len(thisServerId) == 0 {
		return "'thisServerId' is empty string"
	}

	if peerServerIds == nil {
		return "'peerServerIds' is nil"
	}
	if len(peerServerIds) < 1 {
		return "'peerServerIds' must have at least one element"
	}

	seenIds := make(map[ServerId]bool)
	for _, peerId := range peerServerIds {
		if peerId == thisServerId {
			return fmt.Sprintf("'peerServerIds' contains 'thisServerId': %v", thisServerId)
		}
		if _, ok := seenIds[peerId]; ok {
			return fmt.Sprintf("'peerServerIds' contains duplicate value: %v", peerId)
		}
		seenIds[peerId] = true
	}

	return ""
}

// Validate time settings.
// tickerDurtaion must be greater than zero.
// electionTimeout must be greater than tickerDuration.
// These checks are basic sanity checks and currently don't include the
// softer usefulness checks recommended by the raft protocol.
func ValidateTimeSettings(timeSettings TimeSettings) string {
	if timeSettings.TickerDuration.Nanoseconds() <= 0 {
		return "TickerDuration must be greater than zero"
	}
	if timeSettings.ElectionTimeoutLow.Nanoseconds() <= timeSettings.TickerDuration.Nanoseconds() {
		return "ElectionTimeoutLow must be greater than TickerDuration"
	}

	return ""
}
