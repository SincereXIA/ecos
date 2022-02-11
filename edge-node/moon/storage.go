package moon

import (
	"ecos/edge-node/node"
	"ecos/utils/logger"
	"encoding/json"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/syndtr/goleveldb/leveldb"
)

type Storage interface {
	// Save function saves ents and state to the underlying stable storage.
	// Save MUST block until st and ents are on stable storage.
	Save(st raftpb.HardState, ents []raftpb.Entry) error
	// SaveSnap function saves snapshot to the underlying stable storage.
	SaveSnap(snap raftpb.Snapshot) error
	// SaveNodeInfo function saves nodeinfo to the underlying stable storage.
	SaveNodeInfo(nodeInfo node.MemoryNodeInfoStorage) error
	// DBFilePath returns the file path of database snapshot saved with given
	// id.
	// DBFilePath(id uint64) (string, error)
}

type storage struct {
	dataDir string
}

func (s *storage) Save(st raftpb.HardState, ents []raftpb.Entry) error {
	db, err := leveldb.OpenFile(s.dataDir, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	hardstateData, err := json.Marshal(st)
	if err != nil {
		return err
	}
	err = db.Put([]byte("hardstate"), hardstateData, nil)
	if err != nil {
		return err
	}
	entsData, err := json.Marshal(ents)
	if err != nil {
		return err
	}
	err = db.Put([]byte("ents"), entsData, nil)
	if err != nil {
		return err
	}
	return nil
}

func (s *storage) SaveSnap(snap raftpb.Snapshot) error {
	db, err := leveldb.OpenFile(s.dataDir, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	snapData, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	err = db.Put([]byte("snapshot"), snapData, nil)
	if err != nil {
		return err
	}
	return nil
}

func (s *storage) SaveNodeInfo(nodeInfo node.MemoryNodeInfoStorage) error {
	db, err := leveldb.OpenFile(s.dataDir, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	nodeInfoData, err := json.Marshal(nodeInfo)
	if err != nil {
		return nil
	}
	err = db.Put([]byte("nodeinfo"), nodeInfoData, nil)
	if err != nil {
		return nil
	}
	return nil
}

func NewStorage(dataDir string) Storage {
	return &storage{dataDir}
}

func ReadSnap(dataBaseDir string) (snap raftpb.Snapshot) {
	db, err := leveldb.OpenFile(dataBaseDir, nil)
	defer db.Close()
	if err != nil {
		logger.Errorf("Open database failed, err:%v\n", err)
	}
	snapData, err := db.Get([]byte("snapshot"), nil)
	if err != nil {
		logger.Errorf("load snapshot failed, err:%v\n", err)
	}
	json.Unmarshal(snapData, &snap)
	return
}

func ReadState(dataBaseDir string) (st raftpb.HardState, ents []raftpb.Entry) {
	db, err := leveldb.OpenFile(dataBaseDir, nil)
	defer db.Close()
	if err != nil {
		logger.Errorf("Open database failed, err:%v\n", err)
	}
	stData, err := db.Get([]byte("hardstate"), nil)
	if err != nil {
		logger.Errorf("load hardstate info failed, err:%v\n", err)
	}
	json.Unmarshal(stData, &st)
	entsData, err := db.Get([]byte("ents"), nil)
	if err != nil {
		logger.Errorf("load ents info failed, err:%v\n", err)
	}
	json.Unmarshal(entsData, &ents)
	return
}

func ReadNodeInfo(dataBaseDir string) (nodeInfo node.MemoryNodeInfoStorage) {
	db, err := leveldb.OpenFile(dataBaseDir, nil)
	defer db.Close()
	if err != nil {
		logger.Errorf("Open database failed, err:%v\n", err)
	}
	nodeInfoData, err := db.Get([]byte("nodeinfo"), nil)
	if err != nil {
		logger.Errorf("load nodeinfo failed, err:%v\n", err)
	}
	json.Unmarshal(nodeInfoData, &nodeInfo)
	return
}
