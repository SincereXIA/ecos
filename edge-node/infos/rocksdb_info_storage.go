package infos

import (
	"ecos/utils/common"
	"ecos/utils/database"
	"ecos/utils/logger"
	gorocksdb "github.com/SUMStudio/grocksdb"
	"io"
	"io/ioutil"
	"os"
	"path"
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
	return s.getSnapshot()
}

func (s *RocksDBInfoStorage) RecoverFromSnapshot(snapshot []byte) error {
	return s.recoverFromSnapshot(snapshot)
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

func (factory *RocksDBInfoStorageFactory) GetSnapshot() ([]byte, error) {
	cp, err := factory.db.NewCheckpoint()
	if err != nil {
		logger.Infof("Create checkpoint failed, err: %v", err)
		return nil, err
	}
	// TODO:just for Test, need to implement a better way
	snapshot := make([]byte, 1<<25)
	cp.CreateCheckpoint(path.Join(factory.basePath, "snapshot"), 0) // what is logSizeForFlush?
	defer cp.Destroy()
	zipPath := path.Join(factory.basePath, "snapshot.zip")
	common.Zip(path.Join(factory.basePath, "snapshot"), zipPath)
	file, _ := os.Open(zipPath)
	snapshot, err = ioutil.ReadAll(file)
	return snapshot, err
}

func (factory *RocksDBInfoStorageFactory) RecoverFromSnapshot(snapshot []byte) error {
	// save snapshot to disk
	snapshotPath := path.Join(factory.basePath, "snapshot.zip")
	if common.PathExists(snapshotPath) {
		os.Remove(path.Join(factory.basePath, "snapshot.zip")) // remove local snapshot
	}

	err := ioutil.WriteFile(snapshotPath, snapshot, 0644)
	if err != nil && err != io.EOF {
		logger.Infof("Read snapshot failed, err: %v", err)
		return err
	}

	// backup the old db
	backPath := factory.basePath + ".bak"
	os.Rename(factory.basePath, backPath)
	logger.Infof("Rename %v to %v", factory.basePath, backPath)

	// recover from snapshot
	zipPath := path.Join(backPath, "snapshot.zip")
	common.UnZip(zipPath, factory.basePath)
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

func (factory *RocksDBInfoStorageFactory) Close() {
	factory.db.Close()
}

func NewRocksDBInfoStorageFactory(basePath string) StorageFactory {
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
		return nil
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
