package alaya

// MetaStorage Store ObjectMeta
// MetaStorage 负责对象元数据的持久化存储
type MetaStorage interface {
	RecordMeta(meta *ObjectMeta) error
	GetMeta(objID string) (meta *ObjectMeta, err error)
	GetMetaInPG()
}
