package shared

type Operator interface {
	Get(key string) (Operator, error)
	List(prefix string) ([]Operator, error)
	Remove(key string) error
	State() (string, error)
	Info() (interface{}, error)
}
