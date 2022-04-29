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
	ctx    context.Context //context
	cancel context.CancelFunc

	proposeC       chan string            // proposed messages (client)
	confChangeC    chan raftpb.ConfChange // proposed cluster config changes
	communicationC chan []raftpb.Message  // notify upper-layer applications to send messages
	commitC        chan *commit           // entries committed to log (server)
	errorC         chan error             // errors from raft session
	raftChan       chan raftpb.Message    // raft messages

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

var defaultSnapshotCount uint64 = 10000 // set 10 for test

func newRaftNode(id int, ctx context.Context, peers []raft.Peer, join bool, basePath string, readyC chan bool, getSnapshot func() ([]byte, error)) (chan *snap.Snapshotter, *raftNode) {

	ctx, cancel := context.WithCancel(ctx)

	var rc = &raftNode{
		ctx:    ctx,
		cancel: cancel,

		proposeC:       make(chan string),
		confChangeC:    make(chan raftpb.ConfChange),
		communicationC: make(chan []raftpb.Message),
		commitC:        make(chan *commit),
		errorC:         make(chan error),
		raftChan:       make(chan raftpb.Message),

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
	go rc.startRaft(readyC)

	return rc.snapshotterReady, rc
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
			err := cc.Unmarshal(ents[i].Data)
			if err != nil {
				logger.Fatalf("unmarshal conf change error: %v", err)
			}
			rc.confState = *rc.node.ApplyConfChange(cc)
			switch cc.Type {
			case raftpb.ConfChangeAddNode:
			case raftpb.ConfChangeRemoveNode:
			}
		}
	}

	var applyDoneC chan struct{}

	if len(data) > 0 {
		applyDoneC = make(chan struct{}, 1)
		select {
		case rc.commitC <- &commit{data, applyDoneC}:
		case <-rc.ctx.Done():
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
		err = w.Close()
		if err != nil {
			logger.Infof("Raft %v close wal error (%v)", rc.id, err)
		}
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
		err := rc.raftStorage.ApplySnapshot(*snapshot)
		if err != nil {
			logger.Infof("%v err: %v", rc.id, err)
		}
	}
	err = rc.raftStorage.SetHardState(st)
	if err != nil {
		logger.Fatalf("%v", err)
	}

	// append to storage so raft starts at the right place in log
	err = rc.raftStorage.Append(ents)
	if err != nil {
		logger.Fatalf("%v", err)
	}

	return w
}

func (rc *raftNode) startRaft(readyC chan bool) {
	if !fileutil.Exist(rc.snapdir) {
		if err := os.MkdirAll(rc.snapdir, 0750); err != nil {
			logger.Fatalf("Cannot create dir for snapshot (%v)", err)
		}
	}
	rc.snapshotter = snap.New(zap.NewExample(), rc.snapdir)

	oldWal := wal.Exist(rc.waldir)
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

	if oldWal || rc.join {
		rc.node = raft.RestartNode(c)
	} else {
		rc.node = raft.StartNode(c, rc.peers)
	}

	readyC <- true

	go rc.serveChannels()
}

// stop closes http, closes all channels, and stops raft.
func (rc *raftNode) stop() {
	rc.node.Stop()
	close(rc.commitC)
	close(rc.errorC)
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

var snapshotCatchUpEntriesN uint64 = 10000 // set 10 for test

func (rc *raftNode) maybeTriggerSnapshot(applyDoneC <-chan struct{}) {
	if rc.appliedIndex-rc.snapshotIndex <= rc.snapCount {
		return
	}

	// wait until all committed entries are applied (or server is closed)
	if applyDoneC != nil {
		select {
		case <-applyDoneC:
		case <-rc.ctx.Done():
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
		for rc.proposeC != nil && rc.confChangeC != nil {
			select {
			case prop, ok := <-rc.proposeC:
				if !ok {
					rc.proposeC = nil
				} else {
					// blocks until accepted by raft state machine
					err := rc.node.Propose(rc.ctx, []byte(prop))
					if err != nil {
						logger.Errorf("propose failed: %v", err)
					}
				}

			case cc, ok := <-rc.confChangeC:
				if !ok {
					rc.confChangeC = nil
				} else {
					err := rc.node.ProposeConfChange(rc.ctx, cc)
					if err != nil {
						logger.Errorf("propose conf change failed: %v", err)
					}
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
			err := rc.wal.Save(rd.HardState, rd.Entries)
			if err != nil {
				logger.Errorf("wal.Save failed: %v", err)
			}
			if !raft.IsEmptySnap(rd.Snapshot) {
				err := rc.saveSnap(rd.Snapshot)
				if err != nil {
					logger.Errorf("saveSnap failed: %v", err)
				}
				err = rc.raftStorage.ApplySnapshot(rd.Snapshot)
				if err != nil {
					logger.Errorf("ApplySnapshot failed: %v", err)
				}
				rc.publishSnapshot(rd.Snapshot)
			}
			err = rc.raftStorage.Append(rd.Entries)
			if err != nil {
				logger.Errorf("raftStorage.Append failed: %v", err)
			}
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
			rc.cancel()
			return
		}
	}
}
