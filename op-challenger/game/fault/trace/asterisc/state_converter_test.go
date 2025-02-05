package asterisc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const testBinary = "./somewhere/asterisc"

func TestStateConverter(t *testing.T) {
	setup := func(t *testing.T) (*StateConverter, *capturingExecutor) {
		vmCfg := vm.Config{
			VmBin: testBinary,
		}
		executor := &capturingExecutor{}
		converter := NewStateConverter(vmCfg)
		converter.cmdExecutor = executor.exec
		return converter, executor
	}

	t.Run("Valid", func(t *testing.T) {
		converter, executor := setup(t)
		data := VMState{
			Witness:   []byte{1, 2, 3, 4},
			StateHash: common.Hash{0xab},
			Step:      42,
			Exited:    true,
			PC:        11,
		}
		ser, err := json.Marshal(data)
		require.NoError(t, err)
		executor.stdOut = string(ser)
		proof, step, exited, err := converter.ConvertStateToProof(context.Background(), "foo.json")
		require.NoError(t, err)
		require.Equal(t, data.Exited, exited)
		require.Equal(t, data.Step, step)
		require.Equal(t, data.StateHash, proof.ClaimValue)
		require.Equal(t, data.Witness, proof.StateData)
		require.NotNil(t, proof.ProofData, "later validations require this to be non-nil")

		require.Equal(t, testBinary, executor.binary)
		require.Equal(t, []string{"witness", "--input", "foo.json"}, executor.args)
	})

	t.Run("CommandError", func(t *testing.T) {
		converter, executor := setup(t)
		executor.err = errors.New("boom")
		_, _, _, err := converter.ConvertStateToProof(context.Background(), "foo.json")
		require.ErrorIs(t, err, executor.err)
	})

	t.Run("InvalidOutput", func(t *testing.T) {
		converter, executor := setup(t)
		executor.stdOut = "blah blah"
		_, _, _, err := converter.ConvertStateToProof(context.Background(), "foo.json")
		require.ErrorContains(t, err, "failed to parse state data")
	})
}

type capturingExecutor struct {
	binary string
	args   []string

	stdOut string
	stdErr string
	err    error
}

func (c *capturingExecutor) exec(_ context.Context, binary string, args ...string) (string, string, error) {
	c.binary = binary
	c.args = args
	return c.stdOut, c.stdErr, c.err
}
