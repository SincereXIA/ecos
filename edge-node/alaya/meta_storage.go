package alaya

import (
	"ecos/utils/errno"
	"ecos/utils/logger"
	"encoding/json"
	"github.com/tecbot/gorocksdb"
)

// MetaStorage Store ObjectMeta
// MetaStorage 负责对象元数据的持久化存储
type MetaStorage interface {
	RecordMeta(meta *ObjectMeta) error
	GetMeta(objID string) (meta *ObjectMeta, err error)
	GetMetaInPG(pgID uint64, off int, len int) ([]*ObjectMeta, error)
	Close()
}

var ( // rocksdb的设置参数
	opts         = &gorocksdb.Options{}
	readOptions  = &gorocksdb.ReadOptions{}
	writeOptions = &gorocksdb.WriteOptions{}
)

type MemoryMetaStorage struct {
	metaMap map[string]ObjectMeta
}

type StableMetaStorage struct {
	db *gorocksdb.DB
}

func (s *MemoryMetaStorage) RecordMeta(meta *ObjectMeta) error {
	s.metaMap[meta.ObjId] = *meta
	return nil
}

func (s *MemoryMetaStorage) GetMeta(objID string) (meta *ObjectMeta, err error) {
	if m, ok := s.metaMap[objID]; ok {
		return &m, nil
	}
	return nil, errno.MetaNotExist
}

func (s *MemoryMetaStorage) GetMetaInPG(pgID uint64, off int, len int) ([]*ObjectMeta, error) {
	return nil, nil
}

func (s *MemoryMetaStorage) Close() {

}

func NewMemoryMetaStorage() *MemoryMetaStorage {
	return &MemoryMetaStorage{
		metaMap: make(map[string]ObjectMeta),
	}
}

func (s *StableMetaStorage) RecordMeta(meta *ObjectMeta) error {
	metaData, err := json.Marshal(*meta)
	if err != nil {
		return err
	}
	id := meta.ObjId
	logger.Infof("Marshal success, objectMeta:%s", metaData)
	err = s.db.Put(writeOptions, []byte(id), metaData)
	if err != nil {
		return err
	}
	return nil
}

func (s *StableMetaStorage) GetMeta(objID string) (meta *ObjectMeta, err error) {
	metaData, err := s.db.Get(readOptions, []byte(objID))
	defer metaData.Free()
	if err != nil {
		return nil, errno.MetaNotExist
	}
	m := ObjectMeta{}
	json.Unmarshal(metaData.Data(), &m)
	return &m, nil
}

func (s *StableMetaStorage) GetMetaInPG(pgID uint64, off int, len int) ([]*ObjectMeta, error) {
	return nil, nil
}

func (s *StableMetaStorage) Close() {
	s.db.Close()
}

func NewStableMetaStorage(dataBaseDir string) *StableMetaStorage {
	db, err := gorocksdb.OpenDb(opts, dataBaseDir)
	if err != nil {
		logger.Errorf("open database failed, err:%v", err)
	}
	return &StableMetaStorage{
		db: db,
	}
}

func setRocksdbOptions() {
	opts = gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
}

func setRocksdbReadOptions() {
	readOptions = gorocksdb.NewDefaultReadOptions()
}

func setRocksdbWriteOptions() {
	writeOptions = gorocksdb.NewDefaultWriteOptions()
}

func init() { // init rocksdb param
	setRocksdbOptions()
	setRocksdbWriteOptions()
	setRocksdbReadOptions()
}
