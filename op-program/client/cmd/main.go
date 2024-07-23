package main

import (
	"os"
	"runtime"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-program/client"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

func init() {
	// Disable mem profiling, to avoid a lot of unnecessary floating point ops
	runtime.MemProfileRate = 0
}

func main() {
	// Default to a machine parsable but relatively human friendly log format.
	// Don't do anything fancy to detect if color output is supported.
	logger := oplog.NewLogger(os.Stdout, oplog.CLIConfig{
		Level:  log.LevelInfo,
		Format: oplog.FormatLogFmt,
		Color:  false,
	})
	oplog.SetGlobalLogHandler(logger.Handler())
	client.Main(logger)
}
