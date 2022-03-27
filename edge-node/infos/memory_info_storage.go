package infos

import (
	"ecos/utils/errno"
	"sync"
)

type MemoryInfoStorage struct {
	kvStorage   sync.Map
	onUpdateMap sync.Map
}

func (s *MemoryInfoStorage) GetAll() ([]Information, error) {
	var result []Information
	s.kvStorage.Range(func(key, value interface{}) bool {
		result = append(result, value.(Information))
		return true
	})
	return result, nil
}

func (s *MemoryInfoStorage) Update(id string, info Information) error {
	s.kvStorage.Store(id, info)

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
		return Information{}, errno.InfoNotFound
	}
	return v.(Information), nil
}

func (s *MemoryInfoStorage) SetOnUpdate(name string, f StorageUpdateFunc) {
	s.onUpdateMap.Store(name, f)
}

func (s *MemoryInfoStorage) CancelOnUpdate(name string) {
	s.onUpdateMap.Delete(name)
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

func NewMemoryInfoFactory() StorageFactory {
	return &MemoryInfoStorageFactory{}
}
