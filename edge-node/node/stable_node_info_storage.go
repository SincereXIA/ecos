package node

import (
	"ecos/utils/common"
	"ecos/utils/database"
	"ecos/utils/logger"
	gorocksdb "github.com/SUMStudio/grocksdb"
	"github.com/gogo/protobuf/proto"
)

const (
	history          = "history"
	nowState         = "nowState"
	committedState   = "committedState"
	uncommittedState = "uncommittedState"
)

var ( // rocksdb的设置参数
	opts         *gorocksdb.Options
	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions
)

type StableNodeInfoStorage struct {
	*MemoryNodeInfoStorage
	db *gorocksdb.DB
}

func init() {
	opts, readOptions, writeOptions = database.InitRocksdb()
}

// SaveMemoryNodeInfoStorage Save MemoryNodeInfoStorage
func (s *StableNodeInfoStorage) SaveMemoryNodeInfoStorage() {
	historyProtoData, err := proto.Marshal(&s.history)
	if err != nil {
		logger.Errorf("Marshal history failed, err:%v", err)
	}
	err = s.db.Put(writeOptions, []byte(history), historyProtoData)
	if err != nil {
		logger.Errorf("put history failed, err:%v", err)
	}

	nowStateProtoData, err := proto.Marshal(s.nowState)
	if err != nil {
		logger.Errorf("Marshal nowState failed, err:%v", err)
	}
	err = s.db.Put(writeOptions, []byte(nowState), nowStateProtoData)
	if err != nil {
		logger.Errorf("put nowState failed, err:%v", err)
	}

	committedStateProtoData, err := proto.Marshal(s.committedState)
	if err != nil {
		logger.Errorf("Marshal committedState failed, err:%v", err)
	}
	err = s.db.Put(writeOptions, []byte(committedState), committedStateProtoData)
	if err != nil {
		logger.Errorf("put committedState failed, err:%v", err)
	}

	uncommittedStateProtoData, err := proto.Marshal(s.uncommittedState)
	if err != nil {
		logger.Errorf("Marshal uncommittedState failed, err:%v", err)
	}
	err = s.db.Put(writeOptions, []byte(uncommittedState), uncommittedStateProtoData)
	if err != nil {
		logger.Errorf("put uncommittedState failed, err:%v", err)
	}
}

func (s *StableNodeInfoStorage) RecoverMemoryNodeInfoStorage() {
	// TODO
}

func (s *StableNodeInfoStorage) Close() {
	s.db.Close()
}

// NewStableNodeInfoStorage create a stableNodeInfoStorage instance
func NewStableNodeInfoStorage(dataBaseDir string) *StableNodeInfoStorage {
	err := common.InitPath(dataBaseDir)
	if err != nil {
		logger.Errorf("mkdir err: %v", err)
		return nil
	}
	opts = gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	db, err := gorocksdb.OpenDb(opts, dataBaseDir)
	if err != nil {
		logger.Errorf("open database failed, err:%v\n", err)
		return nil
	}

	return &StableNodeInfoStorage{
		NewMemoryNodeInfoStorage(),
		db,
	}
}
