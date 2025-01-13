package nat

var _ Validator = &Gate{}

// A Gate is a collection of suites and/or tests.
type Gate struct {
	ID         string
	Validators []Validator
}

// Run runs all the tests in the suite.
func (s Gate) Run(cfg Config) (bool, error) {
	for _, validator := range s.Validators {
		ok, err := validator.Run(cfg)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// Name returns the id of the gate.
func (g Gate) Name() string {
	return g.ID
}
