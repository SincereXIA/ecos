package alaya

import (
	"ecos/edge-node/object"
	"ecos/utils/errno"
	"encoding/json"
	"sync"
)

// MetaStorage Store ObjectMeta
// MetaStorage 负责对象元数据的持久化存储
// MetaStorage should be thread safe, because it is used by multiple raft.
type MetaStorage interface {
	RecordMeta(meta *object.ObjectMeta) error
	GetMeta(objID string) (meta *object.ObjectMeta, err error)
	List(prefix string) ([]*object.ObjectMeta, error)
	Delete(objID string) error
	CreateSnapshot() ([]byte, error)
}

// MetaStorageRegister is a collection of MetaStorage.
// It can look up MetaStorage by pgID.
type MetaStorageRegister interface {
	// NewStorage creates a new MetaStorage (if not exist).
	// if the MetaStorage already exists, it will return the existing one.
	NewStorage(pgID uint64) (MetaStorage, error)
	// GetStorage returns the MetaStorage by pgID.
	// if the MetaStorage does not exist, it will return ErrNotFound.
	GetStorage(pgID uint64) (MetaStorage, error)
	// GetAllStorage returns all MetaStorage created before.
	GetAllStorage() []MetaStorage
	// Close closes all MetaStorage.
	Close()
}

type MemoryMetaStorage struct {
	MetaMap sync.Map
}

type MemoryMetaStorageRegister struct {
	pgMap sync.Map
}

func (ms *MemoryMetaStorageRegister) NewStorage(pgID uint64) (MetaStorage, error) {
	if storage, ok := ms.pgMap.Load(pgID); ok {
		return storage.(*MemoryMetaStorage), nil
	}
	ms.pgMap.Store(pgID, &MemoryMetaStorage{})
	return ms.GetStorage(pgID)
}

func (ms *MemoryMetaStorageRegister) GetStorage(pgID uint64) (MetaStorage, error) {
	if storage, ok := ms.pgMap.Load(pgID); ok {
		return storage.(*MemoryMetaStorage), nil
	}
	return nil, errno.MetaStorageNotExist
}

func (ms *MemoryMetaStorageRegister) GetAllStorage() []MetaStorage {
	var storages []MetaStorage
	ms.pgMap.Range(func(key, value interface{}) bool {
		storages = append(storages, value.(*MemoryMetaStorage))
		return true
	})
	return storages
}

func (ms *MemoryMetaStorageRegister) Close() {
	// do nothing
	return
}

func NewMemoryMetaStorageRegister() MetaStorageRegister {
	return &MemoryMetaStorageRegister{}
}

func (s *MemoryMetaStorage) RecordMeta(meta *object.ObjectMeta) error {
	s.MetaMap.Store(meta.ObjId, meta)
	return nil
}

func (s *MemoryMetaStorage) GetMeta(objID string) (meta *object.ObjectMeta, err error) {
	if m, ok := s.MetaMap.Load(objID); ok {
		return m.(*object.ObjectMeta), nil
	}
	return nil, errno.MetaNotExist
}

func (s *MemoryMetaStorage) List(prefix string) (metas []*object.ObjectMeta, err error) {
	s.MetaMap.Range(func(key, value interface{}) bool {
		if key.(string)[:len(prefix)] == prefix {
			metas = append(metas, value.(*object.ObjectMeta))
		}
		return true
	})
	return
}

func (s *MemoryMetaStorage) Delete(objID string) error {
	s.MetaMap.Delete(objID)
	return nil
}

func (s *MemoryMetaStorage) CreateSnapshot() ([]byte, error) {
	buf, err := json.Marshal(&s.MetaMap) // TODO(zhang): it is not right
	return buf, err
}

func NewMemoryMetaStorage() *MemoryMetaStorage {
	return &MemoryMetaStorage{
		MetaMap: sync.Map{},
	}
}
