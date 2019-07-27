package config

import (
	"errors"
	"fmt"

	. "github.com/divtxt/raft"
)

// A ClusterInfo holds the ServerIds of the servers in the Raft cluster and
// provides useful functions to work with this list.
type ClusterInfo struct {
	thisServerId         ServerId
	peerServerIds        []ServerId // Excludes thisServerId
	clusterSize          uint
	quorumSizeForCluster uint
}

// Allocate and initialize a NewClusterInfo with the given ServerIds.
//
//  - ServerIds must be distinct non-zero values.
//  - allServerIds should list all the servers in the cluster.
//  - thisServerId is the ServerId of "this" server.
//  - allServerIds must include thisServerId.
//  - allServerIds must contain at least 1 element.
//
func NewClusterInfo(
	allServerIds []ServerId,
	thisServerId ServerId,
) (*ClusterInfo, error) {
	if allServerIds == nil {
		return nil, errors.New("allServerIds is nil")
	}
	if len(allServerIds) < 1 {
		return nil, errors.New("allServerIds must have at least 1 element")
	}
	if thisServerId == 0 {
		return nil, errors.New("thisServerId is 0")
	}

	allServerIdsMap := make(map[ServerId]bool)
	clusterSize := len(allServerIds)
	peerServerIds := make([]ServerId, 0, clusterSize-1)
	for _, serverId := range allServerIds {
		if serverId == 0 {
			return nil, errors.New("allServerIds contains 0")
		}
		if _, ok := allServerIdsMap[serverId]; ok {
			return nil, fmt.Errorf("allServerIds contains duplicate value: %v", serverId)
		}
		allServerIdsMap[serverId] = true
		if serverId != thisServerId {
			peerServerIds = append(peerServerIds, serverId)
		}
	}

	if _, ok := allServerIdsMap[thisServerId]; !ok {
		return nil, fmt.Errorf("allServerIds does not contain thisServerId: %v", thisServerId)
	}

	quorumSizeForCluster := QuorumSizeForClusterSize((uint)(clusterSize))

	ci := &ClusterInfo{
		thisServerId,
		peerServerIds,
		uint(clusterSize),
		quorumSizeForCluster,
	}

	return ci, nil
}

// Get the ServerId of "this" server.
func (ci *ClusterInfo) GetThisServerId() ServerId {
	return ci.thisServerId
}

// Iterate over the list of all peer servers in the cluster and call the given
// function with it's ServerId.
//
// "Peer" servers here means all servers except for "this" server.
//
// If the function returns an error for a peer, the error is returned
// and no further peers are processed.
func (ci *ClusterInfo) ForEachPeer(f func(serverId ServerId) error) error {
	for _, serverId := range ci.peerServerIds {
		err := f(serverId)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsPeer checks if the given ServerId is a peer server in the cluster.
//
// "Peer" servers here means all servers except for "this" server.
func (ci *ClusterInfo) IsPeer(serverId ServerId) bool {
	// XXX: brute forcing for now - at what size does a map/set become more efficient?
	for _, peerServerId := range ci.peerServerIds {
		if serverId == peerServerId {
			return true
		}
	}
	return false
}

// Get the cluster size for this ClusterInfo.
func (ci *ClusterInfo) GetClusterSize() uint {
	return ci.clusterSize
}

// Get the quorum size for this ClusterInfo.
//
// Same as QuorumSizeForClusterSize() for the cluster size of this ClusterInfo.
func (ci *ClusterInfo) QuorumSizeForCluster() uint {
	return ci.quorumSizeForCluster
}

// Helper function to calculate the quorum size for a given cluster size.
//
// For example, a cluster of 5 nodes requires 3 nodes for quorum.
func QuorumSizeForClusterSize(clusterSize uint) uint {
	return (clusterSize / 2) + 1
}
