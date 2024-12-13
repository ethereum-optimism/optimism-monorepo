package cannon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	kvtypes "github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
)

type CannonTraceProvider struct {
	logger         log.Logger
	dir            string
	prestate       string
	generator      utils.ProofGenerator
	gameDepth      types.Depth
	preimageLoader *utils.PreimageLoader
	stateConverter vm.StateConverter
	cfg            vm.Config

	types.PrestateProvider

	// lastStep stores the last step in the actual trace if known. 0 indicates unknown.
	// Cached as an optimisation to avoid repeatedly attempting to execute beyond the end of the trace.
	lastStep uint64
}

func NewTraceProvider(logger log.Logger, m vm.Metricer, cfg vm.Config, vmCfg vm.OracleServerExecutor, prestateProvider types.PrestateProvider, prestate string, localInputs utils.LocalGameInputs, dir string, gameDepth types.Depth) *CannonTraceProvider {
	return &CannonTraceProvider{
		logger:    logger,
		dir:       dir,
		prestate:  prestate,
		generator: vm.NewExecutor(logger, m, cfg, vmCfg, prestate, localInputs),
		gameDepth: gameDepth,
		preimageLoader: utils.NewPreimageLoader(func() (utils.PreimageSource, error) {
			return kvstore.NewDiskKV(logger, vm.PreimageDir(dir), kvtypes.DataFormatFile)
		}),
		PrestateProvider: prestateProvider,
		stateConverter:   NewStateConverter(cfg),
		cfg:              cfg,
	}
}

func (p *CannonTraceProvider) Get(ctx context.Context, pos types.Position) (common.Hash, error) {
	traceIndex := pos.TraceIndex(p.gameDepth)
	if !traceIndex.IsUint64() {
		return common.Hash{}, errors.New("trace index out of bounds")
	}
	proof, err := p.loadProof(ctx, traceIndex.Uint64())
	if err != nil {
		return common.Hash{}, err
	}
	value := proof.ClaimValue

	if value == (common.Hash{}) {
		return common.Hash{}, errors.New("proof missing post hash")
	}
	return value, nil
}

func (p *CannonTraceProvider) GetStepData(ctx context.Context, pos types.Position) ([]byte, []byte, *types.PreimageOracleData, error) {
	traceIndex := pos.TraceIndex(p.gameDepth)
	if !traceIndex.IsUint64() {
		return nil, nil, nil, errors.New("trace index out of bounds")
	}
	proof, err := p.loadProof(ctx, traceIndex.Uint64())
	if err != nil {
		return nil, nil, nil, err
	}
	value := ([]byte)(proof.StateData)
	if len(value) == 0 {
		return nil, nil, nil, errors.New("proof missing state data")
	}
	data := ([]byte)(proof.ProofData)
	if data == nil {
		return nil, nil, nil, errors.New("proof missing proof data")
	}
	oracleData, err := p.preimageLoader.LoadPreimage(proof)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load preimage: %w", err)
	}
	return value, data, oracleData, nil
}

func (p *CannonTraceProvider) GetL2BlockNumberChallenge(_ context.Context) (*types.InvalidL2BlockNumberChallenge, error) {
	return nil, types.ErrL2BlockNumberValid
}

// loadProof will attempt to load or generate the proof data at the specified index
// If the requested index is beyond the end of the actual trace it is extended with no-op instructions.
func (p *CannonTraceProvider) loadProof(ctx context.Context, i uint64) (*utils.ProofData, error) {
	// Attempt to read the last step from disk cache
	if p.lastStep == 0 {
		step, err := utils.ReadLastStep(p.dir)
		if err != nil {
			p.logger.Warn("Failed to read last step from disk cache", "err", err)
		} else {
			p.lastStep = step
		}
	}
	// If the last step is tracked, set i to the last step to generate or load the final proof
	if p.lastStep != 0 && i > p.lastStep {
		i = p.lastStep
	}
	path := filepath.Join(p.dir, utils.ProofsDir, fmt.Sprintf("%d.json.gz", i))
	file, err := ioutil.OpenDecompressed(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := p.generator.GenerateProof(ctx, p.dir, i); err != nil {
			return nil, fmt.Errorf("generate cannon trace with proof at %v: %w", i, err)
		}
		// Try opening the file again now and it should exist.
		file, err = ioutil.OpenDecompressed(path)
		if errors.Is(err, os.ErrNotExist) {
			proof, stateStep, exited, err := p.stateConverter.ConvertStateToProof(ctx, vm.FinalStatePath(p.dir, p.cfg.BinarySnapshots))
			if err != nil {
				return nil, fmt.Errorf("cannot create proof from final state: %w", err)
			}

			if exited && stateStep <= i {
				p.logger.Warn("Requested proof was after the program exited", "proof", i, "last", stateStep)
				// The final instruction has already been applied to this state, so the last step we can execute
				// is one before its Step value.
				p.lastStep = stateStep - 1
				if err := utils.WriteLastStep(p.dir, proof, p.lastStep); err != nil {
					p.logger.Warn("Failed to write last step to disk cache", "step", p.lastStep)
				}
				return proof, nil
			} else {
				return nil, fmt.Errorf("expected proof not generated but final state was not exited, requested step %v, final state at step %v", i, stateStep)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open proof file (%v): %w", path, err)
	}
	defer file.Close()
	var proof utils.ProofData
	err = json.NewDecoder(file).Decode(&proof)
	if err != nil {
		return nil, fmt.Errorf("failed to read proof (%v): %w", path, err)
	}
	return &proof, nil
}

// CannonTraceProviderForTest is a CannonTraceProvider that can find the step referencing the preimage read
// Only to be used for testing
type CannonTraceProviderForTest struct {
	*CannonTraceProvider
}

func NewTraceProviderForTest(logger log.Logger, m vm.Metricer, cfg *config.Config, localInputs utils.LocalGameInputs, dir string, gameDepth types.Depth) *CannonTraceProviderForTest {
	p := &CannonTraceProvider{
		logger:    logger,
		dir:       dir,
		prestate:  cfg.CannonAbsolutePreState,
		generator: vm.NewExecutor(logger, m, cfg.Cannon, vm.NewOpProgramServerExecutor(logger), cfg.CannonAbsolutePreState, localInputs),
		gameDepth: gameDepth,
		preimageLoader: utils.NewPreimageLoader(func() (utils.PreimageSource, error) {
			return kvstore.NewDiskKV(logger, vm.PreimageDir(dir), kvtypes.DataFormatFile)
		}),
		stateConverter: NewStateConverter(cfg.Cannon),
		cfg:            cfg.Cannon,
	}
	return &CannonTraceProviderForTest{p}
}

func (p *CannonTraceProviderForTest) FindStep(ctx context.Context, start uint64, preimage utils.PreimageOpt) (uint64, error) {
	// Run cannon to find the step that meets the preimage conditions
	if err := p.generator.(*vm.Executor).DoGenerateProof(ctx, p.dir, start, math.MaxUint64, preimage()...); err != nil {
		return 0, fmt.Errorf("generate cannon trace (until preimage read): %w", err)
	}
	// Load the step from the state cannon finished with

	_, step, exited, err := p.stateConverter.ConvertStateToProof(ctx, vm.FinalStatePath(p.dir, p.cfg.BinarySnapshots))
	if err != nil {
		return 0, fmt.Errorf("failed to load final state: %w", err)
	}
	// Check we didn't get to the end of the trace without finding the preimage read we were looking for
	if exited {
		return 0, fmt.Errorf("preimage read not found: %w", io.EOF)
	}
	// The state is the post-state so the step we want to execute to read the preimage is step - 1.
	return step - 1, nil
}
