package infos

import (
	"ecos/utils/common"
	"ecos/utils/database"
	"ecos/utils/logger"
	gorocksdb "github.com/SUMStudio/grocksdb"
	"strconv"
	"sync"
)

type RocksDBInfoStorage struct {
	infoType string

	db           *gorocksdb.DB
	cfHandlers   []*gorocksdb.ColumnFamilyHandle
	myHandler    *gorocksdb.ColumnFamilyHandle
	opts         *gorocksdb.Options
	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions

	onUpdateMap sync.Map
}

func (s *RocksDBInfoStorage) Update(info Information) error {
	infoData, err := info.BaseInfo().Marshal()
	if err != nil {
		logger.Errorf("Base info marshal failed, err: %v", err)
		return err
	}
	err = s.db.PutCF(s.writeOptions, s.myHandler, []byte(info.GetID()), infoData)
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
	err := s.db.DeleteCF(s.writeOptions, s.myHandler, []byte(id))
	if err != nil {
		logger.Errorf("Delete id: %v failed, err: %v", id, err)
		return err
	}
	return nil
}

func (s *RocksDBInfoStorage) Get(id string) (Information, error) {
	infoData, err := s.db.GetCF(s.readOptions, s.myHandler, []byte(id))
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
	it := s.db.NewIteratorCF(s.readOptions, s.myHandler)
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

func (s *RocksDBInfoStorage) Close() {
	s.opts.Destroy()
	s.readOptions.Destroy()
	s.writeOptions.Destroy()
	for _, handler := range s.cfHandlers {
		handler.Destroy()
	}
	s.db.Close()
}

func NewRocksDBInfoStorage(basePath string, infoType InfoType) *RocksDBInfoStorage {
	opts, readOptions, writeOptions := database.InitRocksdb()
	infoTypeString := strconv.Itoa(int(infoType))
	cfNames := make([]string, 0)
	if !common.PathExists(basePath) {
		cfNames = []string{"default"}
	} else {
		var err error
		cfNames, err = gorocksdb.ListColumnFamilies(opts, basePath)
		if err != nil {
			logger.Errorf("Get all column family names failed, err: %v", err)
		}
	}
	cfNames = append(cfNames, infoTypeString)
	cfOpts := make([]*gorocksdb.Options, 0)
	for range cfNames {
		cfOpts = append(cfOpts, opts)
	}
	db, handles, err := gorocksdb.OpenDbColumnFamilies(opts, basePath, cfNames, cfOpts)
	if err != nil {
		logger.Errorf("open data base with column families failed, err: %v", err)
		return nil
	}
	var myHandler *gorocksdb.ColumnFamilyHandle
	for idx, name := range cfNames {
		if name == infoTypeString {
			myHandler = handles[idx]
		}
	}
	return &RocksDBInfoStorage{
		infoType:     infoTypeString,
		db:           db,
		cfHandlers:   handles,
		myHandler:    myHandler,
		opts:         opts,
		readOptions:  readOptions,
		writeOptions: writeOptions,
	}

}

type RocksDBInfoStorageFactory struct {
	basePath  string
	cfHandles map[string]*gorocksdb.ColumnFamilyHandle
}

func (factory *RocksDBInfoStorageFactory) GetStorage(infoType InfoType) Storage {

	return NewRocksDBInfoStorage(factory.basePath, infoType)
}

func NewRocksDBInfoStorageFactory(basePath string) StorageFactory {
	err := common.InitParentPath(basePath)
	if err != nil {
		logger.Errorf("Init path %s failed, err: %v", basePath)
	}
	return &RocksDBInfoStorageFactory{
		basePath: basePath,
	}
}
