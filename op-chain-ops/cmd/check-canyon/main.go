package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/mattn/go-isatty"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
)

func CalcBaseFee(parent eth.BlockInfo, elasticity uint64, canyonActive bool) *big.Int {
	denomUint := uint64(50)
	if canyonActive {
		denomUint = uint64(250)
	}
	parentGasTarget := parent.GasLimit() / elasticity
	// If the parent gasUsed is the same as the target, the baseFee remains unchanged.
	if parent.GasUsed() == parentGasTarget {
		return new(big.Int).Set(parent.BaseFee())
	}

	var (
		num   = new(big.Int)
		denom = new(big.Int)
	)

	if parent.GasUsed() > parentGasTarget {
		// If the parent block used more gas than its target, the baseFee should increase.
		// max(1, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
		num.SetUint64(parent.GasUsed() - parentGasTarget)
		num.Mul(num, parent.BaseFee())
		num.Div(num, denom.SetUint64(parentGasTarget))
		num.Div(num, denom.SetUint64(denomUint))
		baseFeeDelta := math.BigMax(num, common.Big1)

		return num.Add(parent.BaseFee(), baseFeeDelta)
	} else {
		// Otherwise if the parent block used less gas than its target, the baseFee should decrease.
		// max(0, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
		num.SetUint64(parentGasTarget - parent.GasUsed())
		num.Mul(num, parent.BaseFee())
		num.Div(num, denom.SetUint64(parentGasTarget))
		num.Div(num, denom.SetUint64(denomUint))
		baseFee := num.Sub(parent.BaseFee(), num)

		return math.BigMax(baseFee, common.Big0)
	}
}

func ManuallyEncodeReceipts(receipts types.Receipts, canyonActive bool) [][]byte {
	v := uint64(1)
	for _, receipt := range receipts {
		if receipt.Type == types.DepositTxType {
			if canyonActive {
				receipt.DepositReceiptVersion = &v
			} else {
				receipt.DepositReceiptVersion = nil
			}

		}
	}
	var out [][]byte
	for i := range receipts {
		var buf bytes.Buffer
		receipts.EncodeIndex(i, &buf)
		out = append(out, buf.Bytes())
	}
	return out
}

type rawReceipts [][]byte

func (s rawReceipts) Len() int { return len(s) }
func (s rawReceipts) EncodeIndex(i int, w *bytes.Buffer) {
	w.Write(s[i])
}
func HashList(list [][]byte) common.Hash {
	hasher := trie.NewStackTrie(nil)
	return types.DeriveSha(rawReceipts(list), hasher)
}

type L2Client interface {
	BlockByNumber(context.Context, *big.Int) (*types.Block, error)
	CodeAt(context.Context, common.Address, *big.Int) ([]byte, error)
	InfoByNumber(context.Context, uint64) (eth.BlockInfo, error)
	FetchReceipts(context.Context, common.Hash) (eth.BlockInfo, types.Receipts, error)
}

type Client struct {
	*ethclient.Client
	*sources.L1Client
}

type Args struct {
	Number     uint64
	Elasticity uint64
	Client     L2Client
}

func ValidateReceipts(ctx Args, canyonActive bool) error {
	block, err := ctx.Client.InfoByNumber(context.Background(), ctx.Number)
	if err != nil {
		return err
	}

	_, receipts, err := ctx.Client.FetchReceipts(context.Background(), block.Hash())
	if err != nil {
		return err
	}

	have := block.ReceiptHash()
	want := HashList(ManuallyEncodeReceipts(receipts, canyonActive))

	if have != want {
		return fmt.Errorf("Receipts do not look correct. canyonActive: %v. have: %v, want: %v", canyonActive, have, want)
	}

	return nil
}

func Validate1559Params(ctx Args, canyonActive bool) error {
	block, err := ctx.Client.InfoByNumber(context.Background(), ctx.Number)
	if err != nil {
		return err
	}

	parent, err := ctx.Client.InfoByNumber(context.Background(), ctx.Number-1)
	if err != nil {
		return err
	}

	want := CalcBaseFee(parent, ctx.Elasticity, canyonActive)
	have := block.BaseFee()

	if have.Cmp(want) != 0 {
		return fmt.Errorf("BaseFee does not match. canyonActive: %v. have: %v, want: %v", canyonActive, have, want)
	}

	return nil
}

func ValidateWithdrawals(ctx Args, canyonActive bool) error {
	block, err := ctx.Client.BlockByNumber(context.Background(), new(big.Int).SetUint64(ctx.Number))
	if err != nil {
		return err
	}

	if canyonActive && block.Withdrawals() == nil {
		return errors.New("No nonwithdrawals in a canyon block")
	} else if canyonActive && len(block.Withdrawals()) > 0 {
		return errors.New("Withdrawals length is not zero in a canyon block")
	} else if !canyonActive && block.Withdrawals() != nil {
		return errors.New("Withdrawals in a pre-canyon block")
	}
	return nil
}

func ValidateCreate2Deployer(ctx Args, canyonActive bool) error {
	addr := common.HexToAddress("0x13b0D85CcB8bf860b6b79AF3029fCA081AE9beF2")
	code, err := ctx.Client.CodeAt(context.Background(), addr, new(big.Int).SetUint64(ctx.Number))
	if err != nil {
		return err
	}
	codeHash := crypto.Keccak256Hash(code)
	expectedCodeHash := common.HexToHash("0xb0550b5b431e30d38000efb7107aaa0ade03d48a7198a140edda9d27134468b2")

	if canyonActive && codeHash != expectedCodeHash {
		return fmt.Errorf("Canyon active but code hash does not match. have: %v, want: %v", codeHash, expectedCodeHash)
	} else if !canyonActive && codeHash == expectedCodeHash {
		return fmt.Errorf("Canyon not active but code hashes do match. codeHash: %v", codeHash)
	}

	return nil
}

// CheckActivation takes a function f which determines in a specific block follows the rules of a fork.
func CheckActivation(f func(Args, bool) error, ctx Args, forkActivated bool, valid *bool) {
	if err := f(ctx, forkActivated); err != nil {
		log.Error("Block did not follow fork rules", "err", err)
		*valid = false
	}
}

// CheckInactivation takes a function f which determines in a specific block follows the rules of a fork.
// It passes the oppose value of forkActivated & asserts that an error is returned.
func CheckInactivation(f func(Args, bool) error, ctx Args, forkActivated bool, valid *bool) {
	if err := f(ctx, !forkActivated); err == nil {
		log.Error("Block followed the wrong side of the fork rules")
		*valid = false
	}
}

func main() {
	color := isatty.IsTerminal(os.Stderr.Fd())
	handler := log.NewTerminalHandler(os.Stderr, color)
	oplog.SetGlobalLogHandler(handler)
	logger := log.NewLogger(handler)

	// Define the flag variables
	var (
		canyonActive bool
		number       uint64
		elasticity   uint64
		rpcURL       string
	)

	valid := true

	// Define and parse the command-line flags
	flag.BoolVar(&canyonActive, "canyon", false, "Set this flag to assert canyon behavior")
	flag.Uint64Var(&number, "number", 31, "Block number to check")
	flag.Uint64Var(&elasticity, "elasticity", 6, "Specify the EIP-1559 elasticity. 6 on mainnet/sepolia. 10 on goerli")
	flag.StringVar(&rpcURL, "rpc-url", "http://localhost:8545", "Specify the L2 ETH RPC URL")

	// Parse the command-line arguments
	flag.Parse()

	l2RPC, err := client.NewRPC(context.Background(), logger, rpcURL, client.WithDialAttempts(10))
	if err != nil {
		log.Crit("Error creating RPC", "err", err)
	}
	c := &rollup.Config{SeqWindowSize: 10}
	l2Cfg := sources.L1ClientDefaultConfig(c, true, sources.RPCKindBasic)
	sourceClient, err := sources.NewL1Client(l2RPC, logger, nil, l2Cfg)
	if err != nil {
		log.Crit("Error creating RPC", "err", err)
	}
	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Crit("Error creating RPC", "err", err)
	}

	client := Client{ethClient, sourceClient}

	ctx := Args{
		Number:     number,
		Elasticity: elasticity,
		Client:     client,
	}

	CheckActivation(ValidateReceipts, ctx, canyonActive, &valid)
	CheckInactivation(ValidateReceipts, ctx, canyonActive, &valid)

	CheckActivation(Validate1559Params, ctx, canyonActive, &valid)
	// Don't check in-activation for 1559 b/c at low basefees the two cannot be differentiated

	CheckActivation(ValidateWithdrawals, ctx, canyonActive, &valid)
	CheckInactivation(ValidateWithdrawals, ctx, canyonActive, &valid)

	CheckActivation(ValidateCreate2Deployer, ctx, canyonActive, &valid)
	CheckInactivation(ValidateCreate2Deployer, ctx, canyonActive, &valid)

	if !valid {
		os.Exit(1)
	} else if canyonActive {
		log.Info(fmt.Sprintf("Successfully validated block %v as a Canyon block", number))
	} else {
		log.Info(fmt.Sprintf("Successfully validated block %v as a Pre-Canyon block", number))
	}
}
