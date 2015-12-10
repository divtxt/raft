package raft

import (
	"fmt"
)

type ClusterInfo struct {
	//
	thisServerId ServerId

	// Excludes thisServerId
	peerServerIds []ServerId
}

// Manage cluster server ids and related calculations.
//
// - Server ids must be distinct non-empty strings.
// - allServerIds must contain at least 2 elements.
// - allServerIds must include 'thisServerId'.
//
func NewClusterInfo(
	allServerIds []ServerId,
	thisServerId ServerId,
) *ClusterInfo {
	if allServerIds == nil {
		panic("allServerIds is nil")
	}
	if len(allServerIds) < 2 {
		panic("allServerIds must have at least 2 elements")
	}
	if len(thisServerId) == 0 {
		panic("thisServerId is empty string")
	}

	allServerIdsMap := make(map[ServerId]bool)
	peerServerIds := make([]ServerId, 0, len(allServerIds)-1)
	for _, serverId := range allServerIds {
		if len(serverId) == 0 {
			panic("allServerIds contains empty string")
		}
		if _, ok := allServerIdsMap[serverId]; ok {
			panic(fmt.Sprintf("allServerIds contains duplicate value: %v", serverId))
		}
		allServerIdsMap[serverId] = true
		if serverId != thisServerId {
			peerServerIds = append(peerServerIds, serverId)
		}
	}

	if _, ok := allServerIdsMap[thisServerId]; !ok {
		panic(fmt.Sprintf("allServerIds does not contain thisServerId: %v", thisServerId))
	}

	ci := &ClusterInfo{
		thisServerId,
		peerServerIds,
	}

	return ci
}

func (ci *ClusterInfo) GetThisServerId() ServerId {
	return ci.thisServerId
}

func (ci *ClusterInfo) ForEachPeer(f func(serverId ServerId)) {
	for _, serverId := range ci.peerServerIds {
		f(serverId)
	}
}

func (ci *ClusterInfo) QuorumSizeForCluster() uint {
	var clusterSize uint
	clusterSize = (uint)(len(ci.peerServerIds) + 1)
	return QuorumSizeForClusterSize(clusterSize)
}

// Quorum formula
func QuorumSizeForClusterSize(clusterSize uint) uint {
	return (clusterSize / 2) + 1
}
