package alaya

// MetaStorage Store ObjectMeta
// MetaStorage 负责对象元数据的持久化存储
type MetaStorage interface {
	RecordMeta(meta *ObjectMeta) error
	GetMeta(objID string) (meta *ObjectMeta, err error)
	GetMetaInPG(pgID uint64, off int, len int) ([]*ObjectMeta, error)
}

type MemoryMetaStorage struct {
}

func (s *MemoryMetaStorage) RecordMeta(meta *ObjectMeta) error {
	return nil
}

func (s *MemoryMetaStorage) GetMeta(objID string) (meta *ObjectMeta, err error) {
	return nil, nil
}

func (s *MemoryMetaStorage) GetMetaInPG(pgID uint64, off int, len int) ([]*ObjectMeta, error) {
	return nil, nil
}
