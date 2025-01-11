package nat

var _ Validator = &Test{}

type Test struct {
	Name string
	Fn   func(cfg Config) (bool, error)
}

func (t *Test) Run(cfg Config) (bool, error) {
	return t.Fn(cfg)
}
