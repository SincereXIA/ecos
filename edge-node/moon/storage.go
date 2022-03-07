package moon

import (
	"ecos/utils/common"
	"ecos/utils/logger"
	"encoding/json"
	gorocksdb "github.com/SUMStudio/grocksdb"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type Storage interface {
	// Save function saves ents and state to the underlying stable stableStorage.
	// Save MUST block until st and ents are on stable stableStorage.
	Save(st raftpb.HardState, ents []raftpb.Entry) error
	// SaveSnap function saves snapshot to the underlying stable stableStorage.
	SaveSnap(snap raftpb.Snapshot) error

	Close()
}

const ( // 字段在数据库中的key值
	hardstate = "hardstate"
	snapshot  = "snapshot"
	entris    = "entris"
)

var ( // rocksdb的设置参数
	opts         = gorocksdb.NewDefaultOptions()
	readOptions  = gorocksdb.NewDefaultReadOptions()
	writeOptions = gorocksdb.NewDefaultWriteOptions()
)

type RocksdbStorage struct {
	db *gorocksdb.DB
}

func (s *RocksdbStorage) Save(st raftpb.HardState, ents []raftpb.Entry) error {
	hardstateData, err := json.Marshal(st)
	if err != nil {
		return err
	}

	err = s.db.Put(writeOptions, []byte(hardstate), hardstateData)
	if err != nil {
		return err
	}
	entsData, err := json.Marshal(ents)
	if err != nil {
		return err
	}
	err = s.db.Put(writeOptions, []byte(entris), entsData)
	if err != nil {
		return err
	}
	return nil
}

func (s *RocksdbStorage) SaveSnap(snap raftpb.Snapshot) error {
	snapData, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	err = s.db.Put(writeOptions, []byte(snapshot), snapData)
	if err != nil {
		return err
	}
	return nil
}

func (s *RocksdbStorage) Close() {
	s.db.Close()
}

func NewStorage(dataBaseDir string) Storage {
	err := common.InitPath(dataBaseDir)
	if err != nil {
		logger.Errorf("mkdir err: %v", err)
		return nil
	}
	opts.SetCreateIfMissing(true)
	db, err := gorocksdb.OpenDb(opts, dataBaseDir)
	if err != nil {
		logger.Errorf("open database failed, err:%v\n", err)
		return nil
	}
	logger.Infof("open database " + dataBaseDir + " success")
	return &RocksdbStorage{
		db: db,
	}
}

func LoadSnap(dataBaseDir string) (snap raftpb.Snapshot) {
	db, err := gorocksdb.OpenDb(opts, dataBaseDir)
	defer db.Close()
	if err != nil {
		logger.Errorf("Open database failed, err:%v\n", err)
	}
	snapData, err := db.Get(readOptions, []byte(snapshot))
	defer snapData.Free()
	if err != nil {
		logger.Errorf("load snapshot failed, err:%v\n", err)
	}
	json.Unmarshal(snapData.Data(), &snap)
	return
}

func ReadState(dataBaseDir string) (st raftpb.HardState, ents []raftpb.Entry) {
	db, err := gorocksdb.OpenDb(opts, dataBaseDir)
	defer db.Close()
	if err != nil {
		logger.Errorf("Open database failed, err:%v\n", err)
	}
	stData, err := db.Get(readOptions, []byte(hardstate))
	defer stData.Free()
	if err != nil {
		logger.Errorf("load hardstate info failed, err:%v\n", err)
	}
	json.Unmarshal(stData.Data(), &st)
	entsData, err := db.Get(readOptions, []byte(entris))
	defer entsData.Free()
	if err != nil {
		logger.Errorf("load ents info failed, err:%v\n", err)
	}
	json.Unmarshal(entsData.Data(), &ents)
	return
}
