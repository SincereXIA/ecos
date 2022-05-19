package alaya

import (
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/utils/common"
	"ecos/utils/database"
	"ecos/utils/errno"
	"ecos/utils/logger"
	gorocksdb "github.com/SUMStudio/grocksdb"
	"strconv"
	"strings"
)

type RocksDBMetaStorageRegister struct {
	basePath    string
	cfHandleMap map[string]*gorocksdb.ColumnFamilyHandle
	cfNames     []string
	db          *gorocksdb.DB
	storageMap  map[uint64]*RocksDBMetaStorage
}

func pgIdToStr(pgID uint64) string {
	return strconv.FormatUint(pgID, 10)
}

func (register *RocksDBMetaStorageRegister) NewStorage(pgID uint64) (MetaStorage, error) {
	pgIDStr := pgIdToStr(pgID)
	if storage, ok := register.storageMap[pgID]; ok {
		return storage, nil
	}
	if register.cfHandleMap[pgIDStr] == nil {
		err := register.newColumnFamily(pgIDStr)
		if err != nil {
			logger.Errorf("new column family failed: %v", err)
			return nil, err
		}
	}
	storage := register.NewRocksDBMetaStorage(pgIDStr)
	register.storageMap[pgID] = storage
	return storage, nil
}

func (register *RocksDBMetaStorageRegister) GetStorage(pgID uint64) (MetaStorage, error) {
	if storage, ok := register.storageMap[pgID]; ok {
		return storage, nil
	}
	return nil, errno.MetaStorageNotExist
}

func (register *RocksDBMetaStorageRegister) GetAllStorage() []MetaStorage {
	result := make([]MetaStorage, 0)
	for _, storage := range register.storageMap {
		result = append(result, storage)
	}
	return result
}

func (register *RocksDBMetaStorageRegister) Close() {
	register.db.Close()
}

type RocksDBMetaStorage struct {
	db        *gorocksdb.DB
	myCfName  string
	myHandler *gorocksdb.ColumnFamilyHandle
}

func (s *RocksDBMetaStorage) RecordMeta(meta *object.ObjectMeta) error {
	key := meta.ObjId
	value, err := meta.Marshal()

	err = s.db.PutCF(database.WriteOpts, s.myHandler, []byte(key), value)
	if err != nil {
		logger.Errorf("RocksDBMetaStorage", "RecordMeta", "PutCF error: "+err.Error())
		return err
	}
	return nil
}

func (s *RocksDBMetaStorage) GetMeta(objID string) (meta *object.ObjectMeta, err error) {
	value, err := s.db.GetCF(database.ReadOpts, s.myHandler, []byte(objID))
	defer value.Free()
	if value.Data() == nil {
		return nil, errno.MetaNotExist
	}
	if err != nil {
		logger.Errorf("RocksDBMetaStorage", "GetMeta", "GetCF error: "+err.Error())
		return nil, err
	}
	meta = &object.ObjectMeta{}
	err = meta.Unmarshal(value.Data())
	if err != nil {
		logger.Errorf("RocksDBMetaStorage", "GetMeta", "Unmarshal error: "+err.Error())
		return nil, err
	}
	return meta, nil
}

func (s *RocksDBMetaStorage) List(prefix string) ([]*object.ObjectMeta, error) {
	result := make([]*object.ObjectMeta, 0)
	it := s.db.NewIteratorCF(database.ReadOpts, s.myHandler)
	defer it.Close()

	for it.SeekToFirst(); it.Valid(); it.Next() {
		key := string(it.Key().Data())
		if strings.HasPrefix(key, prefix) {
			metaData := it.Value()
			meta := &object.ObjectMeta{}
			err := meta.Unmarshal(metaData.Data())
			if err != nil {
				logger.Errorf("RocksDBMetaStorage", "List", "Unmarshal error: "+err.Error())
				return nil, err
			}
			result = append(result, meta)
			metaData.Free()
		}
	}
	return result, nil
}

func (s *RocksDBMetaStorage) Delete(objID string) error {
	err := s.db.DeleteCF(database.WriteOpts, s.myHandler, []byte(objID))
	if err != nil {
		logger.Errorf("RocksDBMetaStorage", "Delete", "DeleteCF error: "+err.Error())
		return err
	}
	return nil
}

func (s *RocksDBMetaStorage) CreateSnapshot() ([]byte, error) {
	CfContent := &infos.CfContent{
		Keys:   make([][]byte, 0),
		Values: make([][]byte, 0),
	}

	it := s.db.NewIteratorCF(database.ReadOpts, s.myHandler)
	defer it.Close()
	for it.SeekToFirst(); it.Valid(); it.Next() {
		key := make([]byte, len(it.Key().Data()))
		copy(key, it.Key().Data())
		value := make([]byte, len(it.Value().Data()))
		copy(value, it.Value().Data())
		CfContent.Keys = append(CfContent.Keys, key)
		CfContent.Values = append(CfContent.Values, value)
		it.Key().Free()
		it.Value().Free()
	}

	snap, err := CfContent.Marshal()
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func (s *RocksDBMetaStorage) RecoverFromSnapshot(snapshot []byte) error {
	CfContent := &infos.CfContent{}
	err := CfContent.Unmarshal(snapshot)
	if err != nil {
		return err
	}

	for i := 0; i < len(CfContent.Keys); i++ {
		err = s.db.PutCF(database.WriteOpts, s.myHandler, CfContent.Keys[i], CfContent.Values[i])
		if err != nil {
			logger.Errorf("RocksDBMetaStorage", "RecoverFromSnapshot", "PutCF error: "+err.Error())
			return err
		}
	}
	return nil
}

func (s *RocksDBMetaStorage) Close() {
	//TODO implement me
}

func (register *RocksDBMetaStorageRegister) newColumnFamily(pgId string) error {
	handle, err := register.db.CreateColumnFamily(database.Opts, pgId)
	if err != nil {
		logger.Errorf("New column family failed, err: %v", err)
		return err
	}
	register.cfHandleMap[pgId] = handle
	return nil
}

func (register *RocksDBMetaStorageRegister) NewRocksDBMetaStorage(cfName string) *RocksDBMetaStorage {
	return &RocksDBMetaStorage{
		db:        register.db,
		myCfName:  cfName,
		myHandler: register.cfHandleMap[cfName],
	}
}

func NewRocksDBMetaStorageRegister(basePath string) (*RocksDBMetaStorageRegister, error) {
	err := common.InitPath(basePath)
	if err != nil {
		logger.Errorf("Set path %s failed, err: %v", basePath)
	}
	cfNames, err := gorocksdb.ListColumnFamilies(database.Opts, basePath)
	if err != nil {
		logger.Infof("List column families failed, err: %v", err)
	}
	if len(cfNames) == 0 {
		cfNames = append(cfNames, "default")
	}
	cfOpts := make([]*gorocksdb.Options, 0)
	for range cfNames {
		cfOpts = append(cfOpts, database.Opts)
	}
	db, handles, err := gorocksdb.OpenDbColumnFamilies(database.Opts, basePath, cfNames, cfOpts)
	if err != nil {
		logger.Errorf("Open rocksdb with ColumnFamilies failed, err: %v", err)
		return nil, err
	}

	handleMap := make(map[string]*gorocksdb.ColumnFamilyHandle, 0)

	for idx, cfName := range cfNames {
		handleMap[cfName] = handles[idx]
	}
	return &RocksDBMetaStorageRegister{
		basePath:    basePath,
		cfHandleMap: handleMap,
		cfNames:     cfNames,
		db:          db,
		storageMap:  make(map[uint64]*RocksDBMetaStorage, 0),
	}, nil
}
