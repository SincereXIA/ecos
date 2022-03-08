package alaya

import (
	"ecos/edge-node/object"
	"ecos/utils/errno"
	"ecos/utils/logger"
	gorocksdb "github.com/SUMStudio/grocksdb"
	"google.golang.org/protobuf/proto"
)

// MetaStorage Store ObjectMeta
// MetaStorage 负责对象元数据的持久化存储
type MetaStorage interface {
	RecordMeta(meta *object.ObjectMeta) error
	GetMeta(objID string) (meta *object.ObjectMeta, err error)
	GetMetaInPG(pgID uint64, off int, len int) ([]*object.ObjectMeta, error)
	Close()
}

var ( // rocksdb的设置参数
	opts         = &gorocksdb.Options{}
	readOptions  = &gorocksdb.ReadOptions{}
	writeOptions = &gorocksdb.WriteOptions{}
)

type MemoryMetaStorage struct {
	metaMap map[string]*object.ObjectMeta
}

type StableMetaStorage struct {
	db *gorocksdb.DB
}

func (s *MemoryMetaStorage) RecordMeta(meta *object.ObjectMeta) error {
	s.metaMap[meta.ObjId] = meta
	return nil
}

func (s *MemoryMetaStorage) GetMeta(objID string) (meta *object.ObjectMeta, err error) {
	if m, ok := s.metaMap[objID]; ok {
		return m, nil
	}
	return nil, errno.MetaNotExist
}

func (s *MemoryMetaStorage) GetMetaInPG(pgID uint64, off int, len int) ([]*object.ObjectMeta, error) {
	return nil, nil
}

func (s *MemoryMetaStorage) Close() {
	// need do nothing
}

func NewMemoryMetaStorage() *MemoryMetaStorage {
	return &MemoryMetaStorage{
		metaMap: make(map[string]*object.ObjectMeta),
	}
}

func (s *StableMetaStorage) RecordMeta(meta *object.ObjectMeta) error {
	metaData, err := proto.Marshal(meta)
	if err != nil {
		logger.Errorf("Marshal failed")
		return err
	}
	id := meta.ObjId
	err = s.db.Put(writeOptions, []byte(id), metaData)
	if err != nil {
		logger.Infof("write database failed, err:%v", err)
		return err
	}
	return nil
}

func (s *StableMetaStorage) GetMeta(objID string) (meta *object.ObjectMeta, err error) {
	metaData, err := s.db.Get(readOptions, []byte(objID))
	if err != nil {
		logger.Errorf("get metaData failed, err:%v", err)
		return nil, errno.MetaNotExist
	}
	M := object.ObjectMeta{}
	err = proto.Unmarshal(metaData.Data(), &M)
	if err != nil {
		return nil, err
	}
	metaData.Free()
	return &M, nil
}

func (s *StableMetaStorage) GetMetaInPG(pgID uint64, off int, len int) ([]*object.ObjectMeta, error) {
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
	logger.Infof("open database: " + dataBaseDir + " success")
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
