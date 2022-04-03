package infos

import (
	"ecos/utils/errno"
)

type StorageUpdateFunc func(info Information)

type Storage interface {
	Update(info Information) error
	Delete(id string) error
	Get(id string) (Information, error)
	GetAll() ([]Information, error)
	GetSnapshot() ([]byte, error)
	RecoverFromSnapshot(snapshot []byte) error

	// SetOnUpdate set a function, it will be called when any info update
	SetOnUpdate(name string, f StorageUpdateFunc)
	CancelOnUpdate(name string)
}

type StorageFactory interface {
	GetStorage(infoType InfoType) Storage
	GetSnapshot() ([]byte, error)
	RecoverFromSnapshot(snapshot []byte) error
}

// StorageRegister can auto operate information to corresponding storage.
// It can identify and call storage by infoType.
type StorageRegister struct {
	storageMap     map[InfoType]Storage
	storageFactory StorageFactory
}

// Register while add an info storage into StorageRegister,
// So StorageRegister can process request of this type of info.
func (register *StorageRegister) Register(infoType InfoType, storage Storage) {
	register.storageMap[infoType] = storage
}

// GetStorage return the registered storage of the infoType.
func (register *StorageRegister) GetStorage(infoType InfoType) Storage {
	return register.storageMap[infoType]
}

// Update will store the Information into corresponding Storage,
// the Storage must Register before.
func (register *StorageRegister) Update(info Information) error {
	if storage, ok := register.storageMap[info.GetInfoType()]; ok {
		return storage.Update(info)
	}
	return errno.InfoTypeNotSupport
}

// Delete will delete the Information from corresponding Storage
// by id, the Storage must Register before.
func (register *StorageRegister) Delete(infoType InfoType, id string) error {
	if storage, ok := register.storageMap[infoType]; ok {
		return storage.Delete(id)
	}
	return errno.InfoTypeNotSupport
}

// Get will return the Information requested from corresponding Storage
// by id, the Storage must Register before.
func (register *StorageRegister) Get(infoType InfoType, id string) (Information, error) {
	if storage, ok := register.storageMap[infoType]; ok {
		return storage.Get(id)
	}
	return InvalidInfo{}, errno.InfoTypeNotSupport
}

// GetSnapshot will return a snapshot of all Information from corresponding Storage
func (register *StorageRegister) GetSnapshot() ([]byte, error) {
	// TODO: This way to get need to optimize
	return register.storageFactory.GetSnapshot()
}

// RecoverFromSnapshot will recover all Information from corresponding Storage
func (register *StorageRegister) RecoverFromSnapshot(snapshot []byte) error {
	return register.storageFactory.RecoverFromSnapshot(snapshot)
}

// StorageRegisterBuilder is used to build StorageRegister.
type StorageRegisterBuilder struct {
	register       *StorageRegister
	storageFactory StorageFactory
}

func (builder *StorageRegisterBuilder) getStorage(infoType InfoType) Storage {
	storage := builder.storageFactory.GetStorage(infoType)
	return storage
}

func (builder *StorageRegisterBuilder) registerAllStorage() {
	builder.register.Register(InfoType_NODE_INFO, builder.getStorage(InfoType_NODE_INFO))
	builder.register.Register(InfoType_CLUSTER_INFO, builder.getStorage(InfoType_CLUSTER_INFO))
	builder.register.Register(InfoType_USER_INFO, builder.getStorage(InfoType_USER_INFO))
	builder.register.Register(InfoType_BUCKET_INFO, builder.getStorage(InfoType_BUCKET_INFO))
	builder.register.Register(InfoType_VOLUME_INFO, builder.getStorage(InfoType_VOLUME_INFO))
}

// GetStorageRegister return a StorageRegister
func (builder *StorageRegisterBuilder) GetStorageRegister() *StorageRegister {
	builder.registerAllStorage()
	return builder.register
}

// NewStorageRegisterBuilder return a StorageRegisterBuilder
func NewStorageRegisterBuilder(factory StorageFactory) *StorageRegisterBuilder {
	return &StorageRegisterBuilder{
		register: &StorageRegister{
			storageMap:     map[InfoType]Storage{},
			storageFactory: factory,
		},
		storageFactory: factory,
	}
}
