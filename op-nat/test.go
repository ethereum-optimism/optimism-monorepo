package nat

import "fmt"

var _ Validator = &Test{}

type Test struct {
	ID string
	Fn func(cfg Config) (bool, error)
}

func (t Test) Run(cfg Config) (bool, error) {
	if t.Fn == nil {
		return false, fmt.Errorf("test function is nil")
	}
	return t.Fn(cfg)
}

// Name returns the id of the test.
func (t Test) Name() string {
	return t.ID
}

// Type returns the type name of the test.
func (t Test) Type() string {
	return "Test"
}
