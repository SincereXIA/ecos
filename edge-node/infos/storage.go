package infos

import "errors"

type StorageUpdateFunc func(info Information)

type Storage interface {
	Update(id string, info Information) error
	Delete(id string) error
	Get(id string) (Information, error)
	GetAll() ([]Information, error)

	// SetOnUpdate set a function, it will be called when any info update
	SetOnUpdate(name string, f StorageUpdateFunc)
	CancelOnUpdate(name string)
}

type StorageFactory interface {
	GetStorage(infoType InfoType) Storage
}

type StorageRegister struct {
	storageMap map[InfoType]Storage
}

// Register while add an info storage into StorageRegister,
// So StorageRegister can process request of this type of info.
func (register *StorageRegister) Register(infoType InfoType, storage Storage) {
	register.storageMap[infoType] = storage
}

func (register *StorageRegister) GetStorage(infoType InfoType) Storage {
	return register.storageMap[infoType]
}

// Update will store the Information into corresponding Storage,
// the Storage must Register before.
func (register *StorageRegister) Update(id string, info Information) error {
	if storage, ok := register.storageMap[info.infoType]; ok {
		return storage.Update(id, info)
	}
	// TODO (zhang)
	return errors.New("info type not support")
}

// Delete will delete the Information from corresponding Storage
// by id, the Storage must Register before.
func (register *StorageRegister) Delete(infoType InfoType, id string) error {
	if storage, ok := register.storageMap[infoType]; ok {
		return storage.Delete(id)
	}
	// TODO (zhang)
	return errors.New("info type not support")
}

// Get will return the Information requested from corresponding Storage
// by id, the Storage must Register before.
func (register *StorageRegister) Get(infoType InfoType, id string) (Information, error) {
	if storage, ok := register.storageMap[infoType]; ok {
		return storage.Get(id)
	}
	// TODO (zhang)
	return Information{}, errors.New("info type not support")
}

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
}

func (builder *StorageRegisterBuilder) GetStorageRegister() *StorageRegister {
	builder.registerAllStorage()
	return builder.register
}

func NewStorageRegisterBuilder(factory StorageFactory) *StorageRegisterBuilder {
	return &StorageRegisterBuilder{
		register: &StorageRegister{
			storageMap: map[InfoType]Storage{},
		},
		storageFactory: factory,
	}
}
