package raft

// Quorum formula
func QuorumSizeForClusterSize(clusterSize uint) uint {
	return (clusterSize / 2) + 1
}
