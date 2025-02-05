package asterisc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
)

// The state struct will be read from json.
// other fields included in json are specific to FPVM implementation, and not required for trace provider.
type VMState struct {
	PC        uint64        `json:"pc"`
	Exited    bool          `json:"exited"`
	Step      uint64        `json:"step"`
	Witness   hexutil.Bytes `json:"witness"`
	StateHash common.Hash   `json:"stateHash"`
}

type StateConverter struct {
	vmConfig    vm.Config
	cmdExecutor func(ctx context.Context, binary string, args ...string) (stdOut string, stdErr string, err error)
}

func NewStateConverter(vmConfig vm.Config) *StateConverter {
	return &StateConverter{
		vmConfig:    vmConfig,
		cmdExecutor: runCmd,
	}
}

func (c *StateConverter) ConvertStateToProof(ctx context.Context, statePath string) (*utils.ProofData, uint64, bool, error) {
	stdOut, stdErr, err := c.cmdExecutor(ctx, c.vmConfig.VmBin, "witness", "--input", statePath)
	if err != nil {
		return nil, 0, false, fmt.Errorf("state conversion failed: %w (%s)", err, stdErr)
	}
	var data VMState
	if err := json.Unmarshal([]byte(stdOut), &data); err != nil {
		return nil, 0, false, fmt.Errorf("failed to parse state data: %w", err)
	}
	// Extend the trace out to the full length using a no-op instruction that doesn't change any state
	// No execution is done, so no proof-data or oracle values are required.
	return &utils.ProofData{
		ClaimValue:   data.StateHash,
		StateData:    data.Witness,
		ProofData:    []byte{},
		OracleKey:    nil,
		OracleValue:  nil,
		OracleOffset: 0,
	}, data.Step, data.Exited, nil
}

func runCmd(ctx context.Context, binary string, args ...string) (stdOut string, stdErr string, err error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	stdOut = outBuf.String()
	stdErr = errBuf.String()
	return
}
