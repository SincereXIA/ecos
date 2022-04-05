package infos

import (
	"ecos/utils/common"
	"ecos/utils/database"
	"ecos/utils/logger"
	gorocksdb "github.com/SUMStudio/grocksdb"
	"sync"
)

type RocksDBInfoStorage struct {
	db        *gorocksdb.DB
	myCfName  string
	myHandler *gorocksdb.ColumnFamilyHandle

	onUpdateMap sync.Map
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

func (factory *RocksDBInfoStorageFactory) NewRocksDBInfoStorage(cfName string) *RocksDBInfoStorage {

	return &RocksDBInfoStorage{
		db:        factory.db,
		myCfName:  cfName,
		myHandler: factory.cfHandleMap[cfName],

		onUpdateMap: sync.Map{},
	}

}

type RocksDBInfoStorageFactory struct {
	basePath    string
	cfHandleMap map[string]*gorocksdb.ColumnFamilyHandle
	cfNames     []string
	db          *gorocksdb.DB
}

func (factory *RocksDBInfoStorageFactory) GetStorage(infoType InfoType) Storage {
	if factory.cfHandleMap[string(infoType)] == nil {
		err := factory.newColumnFamily(infoType)
		if err != nil {
			return nil
		}
	}
	return factory.NewRocksDBInfoStorage(string(infoType))
}

func (factory *RocksDBInfoStorageFactory) newColumnFamily(infoType InfoType) error {
	handle, err := factory.db.CreateColumnFamily(database.Opts, string(infoType))
	if err != nil {
		logger.Errorf("New column family failed, err: %v", err)
		return err
	}
	factory.cfHandleMap[string(infoType)] = handle
	return nil
}

func NewRocksDBInfoStorageFactory(basePath string) StorageFactory {
	err := common.InitPath(basePath)
	if err != nil {
		logger.Errorf("Set path %s failed, err: %v", basePath)
	}
	cfNames, err := gorocksdb.ListColumnFamilies(database.Opts, basePath)
	if err != nil {
		logger.Errorf("List column families failed, err: %v", err)
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
