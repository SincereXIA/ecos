package infos

import (
	"errors"
	"sync"
)

type MemoryInfoStorage struct {
	kvStorage sync.Map
	onUpdate  StorageUpdateFunc
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
	if s.onUpdate != nil {
		s.onUpdate(info)
	}
	return nil
}

func (s *MemoryInfoStorage) Delete(id string) error {
	s.kvStorage.Delete(id)
	return nil
}

func (s *MemoryInfoStorage) Get(id string) (Information, error) {
	v, ok := s.kvStorage.Load(id)
	if !ok {
		// TODO error
		return Information{}, errors.New("key not exist")
	}
	return v.(Information), nil
}

func (s *MemoryInfoStorage) SetOnUpdate(f StorageUpdateFunc) {
	s.onUpdate = f
}

func NewMemoryInfoStorage() Storage {
	return &MemoryInfoStorage{
		kvStorage: sync.Map{},
	}
}

type MemoryInfoStorageFactory struct {
}

func (factory *MemoryInfoStorageFactory) GetStorage(infoType InfoType) Storage {
	return NewMemoryInfoStorage()
}

func NewMemoryInfoFactory() StorageFactory {
	return &MemoryInfoStorageFactory{}
}
