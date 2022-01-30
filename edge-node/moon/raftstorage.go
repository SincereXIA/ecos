package moon

import (
	"errors"
	pb "github.com/coreos/etcd/raft/raftpb"
	"github.com/syndtr/goleveldb/leveldb"
	"sync"
)

// ErrSnapOutOfDate is returned by Storage.CreateSnapshot when a requested
// index is older than the existing snapshot.
var ErrSnapOutOfDate = errors.New("requested index is older than the existing snapshot")

var ErrUnavailable = errors.New("requested entry at index is unavailable")

// ErrCompacted is returned by Storage.Entries/Compact when a requested
// index is unavailable because it predates the last snapshot.
var ErrCompacted = errors.New("requested index is unavailable due to compaction")

type RaftStorage struct {
	// Protects access to all fields. Most methods of MemoryStorage are
	// run on the raft goroutine, but Append() is run on an application
	// goroutine.
	sync.Mutex

	hardState pb.HardState
	snapshot  pb.Snapshot
	// ents[i] has raft log position i+snapshot.Metadata.Index
	ents []pb.Entry

	db *leveldb.DB
}

// NewMemoryStorage creates an empty MemoryStorage.
func NewRaftStorage() *RaftStorage {
	return &RaftStorage{
		// When starting from scratch populate the list with a dummy entry at term zero.
		ents: make([]pb.Entry, 1),
	}
}
