package nat

var _ NATer = &nat{}

type NATer interface {
	Run() (bool, error)
}

type nat struct {
	config Config
}

func New(config *Config) NATer {
	return &nat{
		config: *config,
	}
}

// Run runs the acceptance tests and returns true if the tests pass
func (t *nat) Run() (bool, error) {
	allPassed := true
	for _, validator := range t.config.Validators {
		ok, err := validator.Run(t.config)
		if err != nil {
			return false, err
		}
		if !ok {
			allPassed = false
		}
	}
	return allPassed, nil
}
