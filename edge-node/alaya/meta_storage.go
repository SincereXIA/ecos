package alaya

import "ecos/utils/errno"

// MetaStorage Store ObjectMeta
// MetaStorage 负责对象元数据的持久化存储
type MetaStorage interface {
	RecordMeta(meta *ObjectMeta) error
	GetMeta(objID string) (meta *ObjectMeta, err error)
	GetMetaInPG(pgID uint64, off int, len int) ([]*ObjectMeta, error)
}

type MemoryMetaStorage struct {
	metaMap map[string]ObjectMeta
}

func (s *MemoryMetaStorage) RecordMeta(meta *ObjectMeta) error {
	s.metaMap[meta.ObjId] = *meta
	return nil
}

func (s *MemoryMetaStorage) GetMeta(objID string) (meta *ObjectMeta, err error) {
	if m, ok := s.metaMap[objID]; ok {
		return &m, nil
	}
	return nil, errno.MetaNotExist
}

func (s *MemoryMetaStorage) GetMetaInPG(pgID uint64, off int, len int) ([]*ObjectMeta, error) {
	return nil, nil
}

func NewMemoryMetaStorage() *MemoryMetaStorage {
	return &MemoryMetaStorage{
		metaMap: make(map[string]ObjectMeta),
	}
}
