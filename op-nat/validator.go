package nat

type Validator interface {
	Run(cfg Config) (bool, error)
	Name() string
}
