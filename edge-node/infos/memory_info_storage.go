package infos

import (
	"ecos/utils/errno"
	"sync"
)

type MemoryInfoStorage struct {
	kvStorage   sync.Map
	onUpdateMap sync.Map
}

func (s *MemoryInfoStorage) List(prefix string) ([]Information, error) {
	var infos []Information
	s.kvStorage.Range(func(key, value interface{}) bool {
		if key.(string)[:len(prefix)] == prefix {
			infos = append(infos, value.(Information))
		}
		return true
	})
	return infos, nil
}

func (s *MemoryInfoStorage) GetAll() ([]Information, error) {
	var result []Information
	s.kvStorage.Range(func(key, value interface{}) bool {
		result = append(result, value.(Information))
		return true
	})
	return result, nil
}

func (s *MemoryInfoStorage) Update(info Information) error {
	s.kvStorage.Store(info.GetID(), info)

	s.onUpdateMap.Range(func(key, value interface{}) bool {
		f := value.(StorageUpdateFunc)
		f(info)
		return true
	})

	return nil
}

func (s *MemoryInfoStorage) Delete(id string) error {
	s.kvStorage.Delete(id)
	return nil
}

func (s *MemoryInfoStorage) Get(id string) (Information, error) {
	v, ok := s.kvStorage.Load(id)
	if !ok {
		return InvalidInfo{}, errno.InfoNotFound
	}
	return v.(Information), nil
}

func (s *MemoryInfoStorage) SetOnUpdate(name string, f StorageUpdateFunc) {
	s.onUpdateMap.Store(name, f)
}

func (s *MemoryInfoStorage) CancelOnUpdate(name string) {
	s.onUpdateMap.Delete(name)
}

func (s *MemoryInfoStorage) GetSnapshot() ([]byte, error) {
	return nil, nil
}

func (s *MemoryInfoStorage) RecoverFromSnapshot(data []byte) error {
	return nil
}

func NewMemoryInfoStorage() Storage {
	return &MemoryInfoStorage{
		kvStorage: sync.Map{},
	}
}

type MemoryInfoStorageFactory struct {
}

func (factory *MemoryInfoStorageFactory) GetStorage(_ InfoType) Storage {
	return NewMemoryInfoStorage()
}

func (factory *MemoryInfoStorageFactory) GetSnapshot() ([]byte, error) {
	return nil, nil
}

func (factory *MemoryInfoStorageFactory) RecoverFromSnapshot(snapshot []byte) error {
	return nil
}

func (factory *MemoryInfoStorageFactory) Close() {

}

func NewMemoryInfoFactory() StorageFactory {
	return &MemoryInfoStorageFactory{}
}
