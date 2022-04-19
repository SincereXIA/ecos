package moon

import (
	"context"
	"ecos/utils/logger"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
	"log"
	"os"
	"path"
	"time"
)

type commit struct {
	data       []string
	applyDoneC chan<- struct{}
}

type raftNode struct {
	proposeC       <-chan string            // proposed messages (client)
	confChangeC    <-chan raftpb.ConfChange // proposed cluster config changes
	communicationC chan<- []raftpb.Message  // notify upper-layer applications to send messages
	commitC        chan<- *commit           // entries committed to log (server)
	errorC         chan<- error             // errors from raft session
	raftChan       chan raftpb.Message      // raft messages
	nodeReadyC     chan<- bool              // signals when raft node is ready

	id          int // client ID for raft session
	peers       []raft.Peer
	join        bool   // node is joining an existing cluster
	waldir      string // path to WAL directory
	snapdir     string // path to snapshot directory
	getSnapshot func() ([]byte, error)

	confState     raftpb.ConfState
	snapshotIndex uint64
	appliedIndex  uint64

	// raft backing for the commit/error channel
	node        raft.Node
	raftStorage *raft.MemoryStorage
	wal         *wal.WAL

	snapshotter      *snap.Snapshotter
	snapshotterReady chan *snap.Snapshotter // signals when snapshotter is ready

	snapCount uint64

	stopc chan struct{} // signals proposal channel closed

	logger *zap.Logger
}

var defaultSnapshotCount uint64 = 100 // set 10 for test

func newRaftNode(id int, peers []raft.Peer, join bool, basePath string, getSnapshot func() ([]byte, error)) (chan<- string,
	chan<- raftpb.ConfChange, <-chan []raftpb.Message, <-chan *commit, <-chan error, <-chan bool, <-chan *snap.Snapshotter, *raftNode) {

	proposeC := make(chan string)
	confChangeC := make(chan raftpb.ConfChange)
	communicationC := make(chan []raftpb.Message)
	commitC := make(chan *commit)
	errorC := make(chan error)
	nodeReadyC := make(chan bool)
	raftChan := make(chan raftpb.Message)
	var rc = &raftNode{
		proposeC:       proposeC,
		confChangeC:    confChangeC,
		communicationC: communicationC,
		commitC:        commitC,
		errorC:         errorC,
		nodeReadyC:     nodeReadyC,
		raftChan:       raftChan,

		id:          id,
		peers:       peers,
		join:        join,
		waldir:      path.Join(basePath, "raft"),
		snapdir:     path.Join(basePath, "snap"),
		getSnapshot: getSnapshot,
		snapCount:   defaultSnapshotCount,
		stopc:       make(chan struct{}),

		logger: zap.NewExample(),

		snapshotterReady: make(chan *snap.Snapshotter, 1),
		// rest of structure populated after WAL replay
	}
	go rc.startRaft()
	return proposeC, confChangeC, communicationC, commitC, errorC, nodeReadyC, rc.snapshotterReady, rc
}

func (rc *raftNode) saveSnap(snap raftpb.Snapshot) error {
	walSnap := walpb.Snapshot{
		Index:     snap.Metadata.Index,
		Term:      snap.Metadata.Term,
		ConfState: &snap.Metadata.ConfState,
	}
	// save the snapshot file before writing the snapshot to the wal.
	// This makes it possible for the snapshot file to become orphaned, but prevents
	// a WAL snapshot entry from having no corresponding snapshot file.
	if err := rc.snapshotter.SaveSnap(snap); err != nil {
		return err
	}
	if err := rc.wal.SaveSnapshot(walSnap); err != nil {
		return err
	}
	return rc.wal.ReleaseLockTo(snap.Metadata.Index)
}

func (rc *raftNode) entriesToApply(ents []raftpb.Entry) (nents []raftpb.Entry) {
	if len(ents) == 0 {
		return ents
	}
	firstIdx := ents[0].Index
	if firstIdx > rc.appliedIndex+1 {
		logger.Fatalf("first index of committed entry[%d] should <= progress.appliedIndex[%d]+1", firstIdx, rc.appliedIndex)
	}
	if rc.appliedIndex-firstIdx+1 < uint64(len(ents)) {
		nents = ents[rc.appliedIndex-firstIdx+1:]
	}
	return nents
}

// publishEntries writes committed log entries to commit channel and returns
// whether all entries could be published.
func (rc *raftNode) publishEntries(ents []raftpb.Entry) (<-chan struct{}, bool) {
	if len(ents) == 0 {
		return nil, true
	}

	data := make([]string, 0, len(ents))
	for i := range ents {
		switch ents[i].Type {
		case raftpb.EntryNormal:
			if len(ents[i].Data) == 0 {
				// ignore empty messages
				break
			}
			s := string(ents[i].Data)
			data = append(data, s)
		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			cc.Unmarshal(ents[i].Data)
			rc.confState = *rc.node.ApplyConfChange(cc)
			switch cc.Type {
			case raftpb.ConfChangeAddNode:
				//if len(cc.Context) > 0 {
				//	rc.transport.AddPeer(types.ID(cc.NodeID), []string{string(cc.Context)})
				//}
			case raftpb.ConfChangeRemoveNode:
				//if cc.NodeID == uint64(rc.id) {
				//	logger.Infof("I've been removed from the cluster! Shutting down.")
				//	return nil, false
				//}
				//rc.transport.RemovePeer(types.ID(cc.NodeID))
			}
		}
	}

	var applyDoneC chan struct{}

	if len(data) > 0 {
		applyDoneC = make(chan struct{}, 1)
		select {
		case rc.commitC <- &commit{data, applyDoneC}:
		case <-rc.stopc:
			return nil, false
		}
	}

	// after commit, update appliedIndex
	rc.appliedIndex = ents[len(ents)-1].Index

	return applyDoneC, true
}

func (rc *raftNode) loadSnapshot() *raftpb.Snapshot {
	if wal.Exist(rc.waldir) {
		walSnaps, err := wal.ValidSnapshotEntries(rc.logger, rc.waldir)
		if err != nil {
			logger.Fatalf("Raft %v error listing snapshots (%v)", rc.id, err)
		}
		snapshot, err := rc.snapshotter.LoadNewestAvailable(walSnaps)
		if err != nil && err != snap.ErrNoSnapshot {
			logger.Fatalf("Raft %v error loading snapshot (%v)", rc.id, err)
		}
		return snapshot
	}
	return &raftpb.Snapshot{}
}

// openWAL returns a WAL ready for reading.
func (rc *raftNode) openWAL(snapshot *raftpb.Snapshot) *wal.WAL {
	if !wal.Exist(rc.waldir) {
		if err := os.MkdirAll(rc.waldir, 0750); err != nil {
			logger.Fatalf("Raft %v cannot create dir for wal (%v)", rc.id, err)
		}

		w, err := wal.Create(zap.NewExample(), rc.waldir, nil)
		if err != nil {
			logger.Infof("Raft %v create wal error (%v)", rc.id, err)
		}
		w.Close()
	}

	walsnap := walpb.Snapshot{}
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	logger.Infof("loading WAL at term %d and index %d", walsnap.Term, walsnap.Index)
	w, err := wal.Open(zap.NewExample(), rc.waldir, walsnap)
	if err != nil {
		logger.Fatalf("Raft %v error loading wal (%v)", rc.id, err)
	}

	return w
}

// replayWAL replays WAL entries into the raft instance.
func (rc *raftNode) replayWAL() *wal.WAL {
	log.Printf("replaying WAL of member %d", rc.id)
	snapshot := rc.loadSnapshot()
	w := rc.openWAL(snapshot)
	_, st, ents, err := w.ReadAll()
	if err != nil {
		logger.Fatalf("failed to read WAL (%v)", err)
	}
	rc.raftStorage = raft.NewMemoryStorage()
	if snapshot != nil {
		rc.raftStorage.ApplySnapshot(*snapshot)
	}
	rc.raftStorage.SetHardState(st)

	// append to storage so raft starts at the right place in log
	rc.raftStorage.Append(ents)

	return w
}

func (rc *raftNode) startRaft() {
	if !fileutil.Exist(rc.snapdir) {
		if err := os.MkdirAll(rc.snapdir, 0750); err != nil {
			logger.Fatalf("Cannot create dir for snapshot (%v)", err)
		}
	}
	rc.snapshotter = snap.New(zap.NewExample(), rc.snapdir)

	oldwal := wal.Exist(rc.waldir)
	rc.wal = rc.replayWAL()

	// signal replay has finished
	rc.snapshotterReady <- rc.snapshotter

	c := &raft.Config{
		ID:                        uint64(rc.id),
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   rc.raftStorage,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}

	if oldwal || rc.join {
		rc.node = raft.RestartNode(c)
	} else {
		rc.node = raft.StartNode(c, rc.peers)
	}

	rc.nodeReadyC <- true

	go rc.serveChannels()
}

// stop closes http, closes all channels, and stops raft.
func (rc *raftNode) stop() {
	close(rc.commitC)
	close(rc.errorC)
	rc.node.Stop()
}

func (rc *raftNode) publishSnapshot(snapshotToSave raftpb.Snapshot) {
	if raft.IsEmptySnap(snapshotToSave) {
		return
	}

	logger.Infof("publishing snapshot at index %d", rc.snapshotIndex)
	defer logger.Infof("finished publishing snapshot at index %d", rc.snapshotIndex)

	if snapshotToSave.Metadata.Index <= rc.appliedIndex {
		logger.Fatalf("snapshot index [%d] should > progress.appliedIndex [%d]", snapshotToSave.Metadata.Index, rc.appliedIndex)
	}
	rc.commitC <- nil // trigger kvstore to load snapshot

	rc.confState = snapshotToSave.Metadata.ConfState
	rc.snapshotIndex = snapshotToSave.Metadata.Index
	rc.appliedIndex = snapshotToSave.Metadata.Index
}

var snapshotCatchUpEntriesN uint64 = 100 // set 10 for test

func (rc *raftNode) maybeTriggerSnapshot(applyDoneC <-chan struct{}) {
	if rc.appliedIndex-rc.snapshotIndex <= rc.snapCount {
		return
	}

	// wait until all committed entries are applied (or server is closed)
	if applyDoneC != nil {
		select {
		case <-applyDoneC:
		case <-rc.stopc:
			return
		}
	}

	logger.Infof("%v start snapshot [applied index: %d | last snapshot index: %d]", rc.id, rc.appliedIndex, rc.snapshotIndex)
	data, err := rc.getSnapshot()
	if err != nil {
		logger.Fatalf("Get snapshot failed, %v", err)
	}
	snap, err := rc.raftStorage.CreateSnapshot(rc.appliedIndex, &rc.confState, data)
	if err != nil {
		panic(err)
	}
	if err := rc.saveSnap(snap); err != nil {
		panic(err)
	}

	compactIndex := uint64(1)
	if rc.appliedIndex > snapshotCatchUpEntriesN {
		compactIndex = rc.appliedIndex - snapshotCatchUpEntriesN
	}
	if err := rc.raftStorage.Compact(compactIndex); err != nil {
		panic(err)
	}

	logger.Infof("compacted log at index %d", compactIndex)
	rc.snapshotIndex = rc.appliedIndex
}

func (rc *raftNode) serveChannels() {
	snap, err := rc.raftStorage.Snapshot()
	if err != nil {
		panic(err)
	}
	rc.confState = snap.Metadata.ConfState
	rc.snapshotIndex = snap.Metadata.Index
	rc.appliedIndex = snap.Metadata.Index

	defer rc.wal.Close()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// send proposals over raft
	go func() {
		confChangeCount := uint64(0)

		for rc.proposeC != nil && rc.confChangeC != nil {
			select {
			case prop, ok := <-rc.proposeC:
				if !ok {
					rc.proposeC = nil
				} else {
					// blocks until accepted by raft state machine
					rc.node.Propose(context.TODO(), []byte(prop))
				}

			case cc, ok := <-rc.confChangeC:
				if !ok {
					rc.confChangeC = nil
				} else {
					confChangeCount++
					cc.ID = confChangeCount
					rc.node.ProposeConfChange(context.TODO(), cc)
				}
			}
		}
		// client closed channel; shutdown raft if not already
		close(rc.stopc)
	}()

	// event loop on raft state machine updates
	for {
		select {
		case <-ticker.C:
			rc.node.Tick()

		// store raft entries to wal, then publish over commit channel
		case rd := <-rc.node.Ready():
			rc.wal.Save(rd.HardState, rd.Entries)
			if !raft.IsEmptySnap(rd.Snapshot) {
				rc.saveSnap(rd.Snapshot)
				rc.raftStorage.ApplySnapshot(rd.Snapshot)
				rc.publishSnapshot(rd.Snapshot)
			}
			rc.raftStorage.Append(rd.Entries)
			// rc.transport.Send(rd.Messages)
			rc.communicationC <- rd.Messages
			applyDoneC, ok := rc.publishEntries(rc.entriesToApply(rd.CommittedEntries))
			if !ok {
				rc.stop()
				return
			}
			rc.maybeTriggerSnapshot(applyDoneC)
			rc.node.Advance()

		case m := <-rc.raftChan:
			err := rc.node.Step(context.TODO(), m)
			if err != nil {
				logger.Errorf("failed to process raft message %v", err)
				return
			}
		case <-rc.stopc:
			rc.stop()
			return
		}
	}
}
