package infos

type Storage interface {
	Update(info Information) error
	Delete(info Information) error
	Get(id string) (Information, error)
}
