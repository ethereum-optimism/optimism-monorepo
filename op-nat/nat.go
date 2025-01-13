package nat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/ethereum-optimism/optimism/op-service/cliapp"
)

var _ cliapp.Lifecycle = &nat{}

type TestResult struct {
	Test   Validator
	Passed bool
	Error  error
}

type nat struct {
	ctx     context.Context
	log     log.Logger
	config  *Config
	version string
	results []TestResult

	running atomic.Bool
}

func New(ctx context.Context, config *Config, log log.Logger, version string) (*nat, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if err := config.Check(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &nat{
		ctx:     ctx,
		config:  config,
		log:     log,
		version: version,
	}, nil
}

// Run runs the acceptance tests and returns true if the tests pass
func (n *nat) Start(ctx context.Context) error {
	n.log.Info("Starting OpNAT")
	n.ctx = ctx
	n.running.Store(true)
	for _, validator := range n.config.Validators {
		n.log.Info("Running validator", "validator", validator.Name(), "type", validator.Type())
		passed, err := validator.Run(ctx, n.log, *n.config)
		n.log.Info("Completedvalidator", "validator", validator.Name(), "type", validator.Type(), "passed", passed, "error", err)
		if err != nil {
			n.log.Error("Error running validator", "validator", validator.Name(), "error", err)
		}
		n.addResult(validator, passed, err)
	}
	n.log.Info("OpNAT finished")
	return nil
}

func (n *nat) Stop(ctx context.Context) error {
	n.printResults()
	n.running.Store(false)
	n.log.Info("OpNAT stopped")
	return nil
}

func (n *nat) Stopped() bool {
	return n.running.Load()
}

func (n *nat) addResult(test Validator, passed bool, err error) {
	n.results = append(n.results, TestResult{
		Test:   test,
		Passed: passed,
		Error:  err,
	})
}
func (n *nat) printResults() {
	n.log.Info("Printing results...")
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Type", "ID", "Result", "Error"})
	resultRows := []table.Row{}
	overallPass := true
	for _, result := range n.results {
		resultPass := "PASS"
		if !result.Passed {
			resultPass = "FAIL"
		}
		resultRows = append(resultRows, table.Row{result.Test.Type(), result.Test.Name(), resultPass, result.Error})
		if !result.Passed {
			overallPass = false
		}
	}
	t.AppendRows(resultRows)
	t.AppendSeparator()
	t.AppendRow([]interface{}{"SUMMARY", "", overallPass, ""})
	t.Render()
}
