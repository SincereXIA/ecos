package infos

import (
	"ecos/utils/common"
	"ecos/utils/database"
	"ecos/utils/logger"
	"errors"
	gorocksdb "github.com/SUMStudio/grocksdb"
	"strconv"
	"sync"
)

type RocksDBInfoStorage struct {
	db        *gorocksdb.DB
	myCfName  string
	myHandler *gorocksdb.ColumnFamilyHandle

	onUpdateMap sync.Map

	getSnapshot         func() ([]byte, error)
	recoverFromSnapshot func(snapshot []byte) error
}

func (s *RocksDBInfoStorage) GetSnapshot() ([]byte, error) {
	return nil, nil
}

func (s *RocksDBInfoStorage) RecoverFromSnapshot(snapshot []byte) error {
	return nil
}

func (s *RocksDBInfoStorage) Update(info Information) error {
	infoData, err := info.BaseInfo().Marshal()
	if err != nil {
		logger.Errorf("Base info marshal failed, err: %v", err)
		return err
	}
	err = s.db.PutCF(database.WriteOpts, s.myHandler, []byte(info.GetID()), infoData)
	if err != nil {
		logger.Errorf("Put info into rocksdb failed, err: %v", err)
		return err
	}

	s.onUpdateMap.Range(func(key, value interface{}) bool {
		f := value.(StorageUpdateFunc)
		f(info)
		return true
	})

	return nil
}

func (s *RocksDBInfoStorage) Delete(id string) error {
	err := s.db.DeleteCF(database.WriteOpts, s.myHandler, []byte(id))
	if err != nil {
		logger.Errorf("Delete id: %v failed, err: %v", id, err)
		return err
	}
	return nil
}

func (s *RocksDBInfoStorage) Get(id string) (Information, error) {
	infoData, err := s.db.GetCF(database.ReadOpts, s.myHandler, []byte(id))
	if infoData.Data() == nil {
		return nil, errors.New("key " + id + " not found")
	}
	defer infoData.Free()
	if err != nil {
		logger.Errorf("Get id: %v from rocksdb failed, err: %v", err)
		return nil, err
	}
	baseInfo := &BaseInfo{}
	err = baseInfo.Unmarshal(infoData.Data())
	if err != nil {
		logger.Errorf("Unmarshal base info failed, err: %v", err)
	}
	return baseInfo, nil
}

func (s *RocksDBInfoStorage) GetAll() ([]Information, error) {
	result := make([]Information, 0)
	it := s.db.NewIteratorCF(database.ReadOpts, s.myHandler)
	defer it.Close()
	it.SeekToFirst()

	for it = it; it.Valid(); it.Next() {
		infoData := it.Value()
		baseInfo := &BaseInfo{}
		err := baseInfo.Unmarshal(infoData.Data())
		if err != nil {
			logger.Errorf("Unmarshal base info failed, err: %v", err)
			return nil, nil
		}
		result = append(result, baseInfo)
		infoData.Free()
	}

	return result, nil
}

func (s *RocksDBInfoStorage) SetOnUpdate(name string, f StorageUpdateFunc) {
	s.onUpdateMap.Store(name, f)
}

func (s *RocksDBInfoStorage) CancelOnUpdate(name string) {
	s.onUpdateMap.Delete(name)
}

func (factory *RocksDBInfoStorageFactory) GetSnapshot() ([]byte, error) {
	snapContent := SnapContent{
		Key:              make([][]byte, 0),
		Value:            make([][]byte, 0),
		ColumnFamilyName: make([]string, 0),
	}

	for _, name := range factory.cfNames {
		handle := factory.cfHandleMap[name]

		it := factory.db.NewIteratorCF(database.ReadOpts, handle)
		it.SeekToFirst()

		for it = it; it.Valid(); it.Next() {
			key := make([]byte, len(it.Key().Data()))
			copy(key, it.Key().Data())
			value := make([]byte, len(it.Value().Data()))
			copy(value, it.Value().Data())
			snapContent.Key = append(snapContent.Key, key)
			snapContent.Value = append(snapContent.Value, value)
			snapContent.ColumnFamilyName = append(snapContent.ColumnFamilyName, name)
			it.Key().Free()
			it.Value().Free()
		}
		it.Close()
	}

	snap, err := snapContent.Marshal()
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func (factory *RocksDBInfoStorageFactory) RecoverFromSnapshot(snapshot []byte) error {
	snapContent := SnapContent{}
	err := snapContent.Unmarshal(snapshot)
	if err != nil {
		return err
	}
	for i, name := range snapContent.ColumnFamilyName {
		if factory.cfHandleMap[name] == nil {
			handle, err := factory.db.CreateColumnFamily(database.Opts, name)
			if err != nil {
				logger.Errorf("New column family failed, err: %v", err)
				return err
			}
			factory.cfNames = append(factory.cfNames, name)
			factory.cfHandleMap[name] = handle
		}
		handle := factory.cfHandleMap[name]
		err = factory.db.PutCF(database.WriteOpts, handle, snapContent.Key[i], snapContent.Value[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (factory *RocksDBInfoStorageFactory) NewRocksDBInfoStorage(cfName string) *RocksDBInfoStorage {

	return &RocksDBInfoStorage{
		db:        factory.db,
		myCfName:  cfName,
		myHandler: factory.cfHandleMap[cfName],

		onUpdateMap:         sync.Map{},
		getSnapshot:         factory.GetSnapshot,
		recoverFromSnapshot: factory.RecoverFromSnapshot,
	}

}

type RocksDBInfoStorageFactory struct {
	basePath    string
	cfHandleMap map[string]*gorocksdb.ColumnFamilyHandle
	cfNames     []string
	db          *gorocksdb.DB
}

func (factory *RocksDBInfoStorageFactory) GetStorage(infoType InfoType) Storage {
	infoTypeStr := infoTypeToStr(infoType)
	if factory.cfHandleMap[infoTypeStr] == nil {
		err := factory.newColumnFamily(infoTypeStr)
		if err != nil {
			return nil
		}
	}
	return factory.NewRocksDBInfoStorage(infoTypeStr)
}

func (factory *RocksDBInfoStorageFactory) newColumnFamily(infoType string) error {
	handle, err := factory.db.CreateColumnFamily(database.Opts, infoType)
	if err != nil {
		logger.Errorf("New column family failed, err: %v", err)
		return err
	}
	factory.cfNames = append(factory.cfNames, infoType)
	factory.cfHandleMap[infoType] = handle
	return nil
}

func infoTypeToStr(infoType InfoType) string {
	return strconv.Itoa(int(infoType))
}

func (factory *RocksDBInfoStorageFactory) Close() {
	factory.db.Close()
}

func NewRocksDBInfoStorageFactory(basePath string) StorageFactory {
	if !common.PathExists(basePath) {
		err := common.InitPath(basePath)
		if err != nil {
			logger.Errorf("Set path %s failed, err: %v", basePath)
		}
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
	}

	handleMap := make(map[string]*gorocksdb.ColumnFamilyHandle, 0)

	for idx, cfName := range cfNames {
		handleMap[cfName] = handles[idx]
	}
	return &RocksDBInfoStorageFactory{
		basePath:    basePath,
		db:          db,
		cfHandleMap: handleMap,
		cfNames:     cfNames,
	}
}
