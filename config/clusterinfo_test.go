package config_test

import (
	"errors"
	"reflect"
	"testing"

	. "github.com/divtxt/raft"
	"github.com/divtxt/raft/config"
)

func TestNewClusterInfo_Validation(t *testing.T) {
	tests := []struct {
		aids        []ServerId
		tid         ServerId
		expectedErr string
	}{
		{
			nil,
			1,
			"allServerIds is nil",
		},
		{
			[]ServerId{},
			1,
			"allServerIds must have at least 1 element",
		},
		{
			[]ServerId{1, 2},
			0,
			"thisServerId is 0",
		},
		{
			[]ServerId{1, 0},
			1,
			"allServerIds contains 0",
		},
		{
			[]ServerId{1, 2, 2},
			1,
			"allServerIds contains duplicate value: 2",
		},
		{
			[]ServerId{2, 3},
			1,
			"allServerIds does not contain thisServerId: 1",
		},
	}

	for _, test := range tests {
		_, err := config.NewClusterInfo(test.aids, test.tid)
		if e := err.Error(); e != test.expectedErr {
			t.Fatal(e)
		}
	}
}

func TestClusterInfo_Assorted(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{1, 2, 3}, 1)
	if err != nil {
		t.Fatal(err)
	}

	if ci.GetThisServerId() != 1 {
		t.Fatal()
	}

	if ci.GetClusterSize() != 3 {
		t.Fatal()
	}
	if ci.QuorumSizeForCluster() != 2 {
		t.Fatal()
	}
}

func TestClusterInfo_SOLO_Assorted(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{1}, 1)
	if err != nil {
		t.Fatal(err)
	}

	if ci.GetThisServerId() != 1 {
		t.Fatal()
	}

	if ci.GetClusterSize() != 1 {
		t.Fatal()
	}
	if ci.QuorumSizeForCluster() != 1 {
		t.Fatal()
	}
}

func TestClusterInfo_ForEach(t *testing.T) {
	ci, err := config.NewClusterInfo([]ServerId{1, 2, 3}, 1)
	if err != nil {
		t.Fatal(err)
	}

	seenIds := make([]ServerId, 0, 3)
	err = ci.ForEachPeer(func(serverId ServerId) error {
		seenIds = append(seenIds, serverId)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seenIds, []ServerId{2, 3}) {
		t.Fatal(seenIds)
	}

	seenIds = make([]ServerId, 0, 3)
	err = ci.ForEachPeer(func(serverId ServerId) error {
		seenIds = append(seenIds, serverId)
		return errors.New("foo!")
	})
	if err.Error() != "foo!" {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seenIds, []ServerId{2}) {
		t.Fatal(seenIds)
	}
}

func TestQuorumSizeForClusterSize(t *testing.T) {
	clusterSizes := []uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	expectedQrms := []uint{1, 2, 2, 3, 3, 4, 4, 5, 5, 6}

	for i, cs := range clusterSizes {
		if config.QuorumSizeForClusterSize(cs) != expectedQrms[i] {
			t.Fatal()
		}
	}
}
