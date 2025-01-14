package dripper

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-dripper/bindings"
	"github.com/ethereum-optimism/optimism/op-dripper/metrics"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type Client interface {
	CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error)
	CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
}

type DrippieContract interface {
	GetDripCount(*bind.CallOpts) (*big.Int, error)
	Created(*bind.CallOpts, *big.Int) (string, error)
	Executable(*bind.CallOpts, string) (bool, error)
}

type DriverSetup struct {
	Log    log.Logger
	Metr   metrics.Metricer
	Cfg    DripExecutorConfig
	Txmgr  txmgr.TxManager
	Client Client
}

type DripExecutor struct {
	DriverSetup

	wg   sync.WaitGroup
	done chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	mutex   sync.Mutex
	running bool

	drippieContract DrippieContract
	drippieABI      *abi.ABI
}

func NewDripExecutor(setup DriverSetup) (_ *DripExecutor, err error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Ensure context does't leak.
	defer func() {
		if err != nil || recover() != nil {
			cancel()
		}
	}()

	if setup.Cfg.DrippieAddr == nil {
		return nil, errors.New("drippie address is required")
	}

	return newDripExecutor(ctx, cancel, setup)
}

func newDripExecutor(ctx context.Context, cancel context.CancelFunc, setup DriverSetup) (*DripExecutor, error) {
	drippieContract, err := bindings.NewDrippieCaller(*setup.Cfg.DrippieAddr, setup.Client)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create drippie at address %s: %w", setup.Cfg.DrippieAddr, err)
	}

	log.Info("connected to drippie", "address", setup.Cfg.DrippieAddr)
	parsed, err := bindings.DrippieMetaData.GetAbi()
	if err != nil {
		cancel()
		return nil, err
	}

	return &DripExecutor{
		DriverSetup: setup,
		done:        make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,

		drippieContract: drippieContract,
		drippieABI:      parsed,
	}, nil
}

func (d *DripExecutor) Start() error {
	d.Log.Info("starting executor")

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.running {
		return errors.New("drip executor is already running")
	}
	d.running = true

	d.wg.Add(1)
	go d.loop()

	d.Log.Info("started executor")
	return nil
}

func (d *DripExecutor) Stop() error {
	d.Log.Info("stopping executor")

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if !d.running {
		return errors.New("drip executor is not running")
	}
	d.running = false

	d.cancel()
	close(d.done)
	d.wg.Wait()

	d.Log.Info("stopped executor")
	return nil
}

func (d *DripExecutor) loop() {
	defer d.wg.Done()
	defer d.Log.Info("loop returning")

	ctx := d.ctx
	ticker := time.NewTicker(d.Cfg.PollInterval)
	defer ticker.Stop()

	//nolint:gosimple // This is an event loop that needs to handle multiple channels
	for {
		select {
		case <-ticker.C:
			// Prioritize quit signal
			select {
			case <-d.done:
				return
			default:
			}

			drips, err := d.fetchExecutableDrips(ctx)
			if err != nil {
				d.Log.Warn("failed to fetch executable drips", "error", err)
				continue
			}

			for _, drip := range drips {
				d.executeDrip(ctx, drip)
			}
		}
	}
}

func (d *DripExecutor) fetchExecutableDrips(ctx context.Context) ([]string, error) {
	// Get total number of drips
	d.Log.Info("getting drip count")
	count, err := d.drippieContract.GetDripCount(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("failed to get drip count: %w", err)
	}
	d.Log.Info("drip count", "count", count)

	var executableDrips []string

	// Iterate through all drips
	for i := int64(0); i < count.Int64(); i++ {
		// Get drip name at index
		name, err := d.drippieContract.Created(&bind.CallOpts{Context: ctx}, big.NewInt(i))
		if err != nil {
			d.Log.Error("failed to get drip name", "index", i, "error", err)
			continue
		}

		// Check if drip is executable
		// Note: This call may revert if the drip is not executable
		executable, err := d.drippieContract.Executable(&bind.CallOpts{Context: ctx}, name)
		if err != nil {
			// Log the error but continue with next drip
			d.Log.Info("drip is not executable", "name", name, "error", err)
			continue
		}

		if executable {
			d.Log.Info("drip is executable", "name", name)
			executableDrips = append(executableDrips, name)
		} else {
			d.Log.Info("drip is not executable", "name", name)
		}
	}

	return executableDrips, nil
}

func (d *DripExecutor) executeDrip(ctx context.Context, name string) {
	cCtx, cCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cCancel()

	if err := d.sendTransaction(cCtx, name); err != nil {
		d.Log.Error("failed to send drip execution transaction", "name", name, "error", err)
		return
	}
	d.Metr.RecordDripExecuted(name)
}

func (d *DripExecutor) sendTransaction(ctx context.Context, name string) error {
	d.Log.Info("executing drip", "name", name)

	data, err := d.executeDripTxData(name)
	if err != nil {
		return err
	}
	receipt, err := d.Txmgr.Send(ctx, txmgr.TxCandidate{
		TxData:   data,
		To:       d.Cfg.DrippieAddr,
		GasLimit: 0,
	})
	if err != nil {
		return err
	}

	if receipt.Status == types.ReceiptStatusFailed {
		d.Log.Error("drip execution failed", "name", name, "tx_hash", receipt.TxHash)
	} else {
		d.Log.Info("drip executed", "name", name, "tx_hash", receipt.TxHash)
	}
	return nil
}

func (d *DripExecutor) executeDripTxData(name string) ([]byte, error) {
	return executeDripTxData(d.drippieABI, name)
}

func executeDripTxData(abi *abi.ABI, name string) ([]byte, error) {
	return abi.Pack(
		"drip",
		name)
}
