package validations

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/lmittmann/w3"
	"github.com/lmittmann/w3/module/eth"
	"github.com/lmittmann/w3/w3types"
)

var validateFunc = w3.MustNewFunc("validate((address proxyAdminAddress,address systemConfigAddress,bytes32 absolutePrestate,uint256 chainID) input,bool allowFailure)", "string")

type validateFuncArgs struct {
	ProxyAdminAddress   common.Address
	SystemConfigAddress common.Address
	AbsolutePrestate    common.Hash
	ChainID             *big.Int
}

type Validator interface {
	Validate(ctx context.Context, input BaseValidatorInput) ([]string, error)
}

type BaseValidator struct {
	client  *rpc.Client
	release string
}

type BaseValidatorInput struct {
	ProxyAdminAddress   common.Address
	SystemConfigAddress common.Address
	AbsolutePrestate    common.Hash
	L2ChainID           *big.Int
}

func newBaseValidator(client *rpc.Client, release string) *BaseValidator {
	return &BaseValidator{client: client, release: release}
}

func (v *BaseValidator) Validate(ctx context.Context, input BaseValidatorInput) ([]string, error) {
	l1ChainID, err := ethclient.NewClient(v.client).ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	validatorAddr, err := ValidatorAddress(l1ChainID.Uint64(), v.release)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator address: %w", err)
	}

	var rawOutput []byte
	if err := w3.NewClient(v.client).CallCtx(
		ctx,
		eth.Call(&w3types.Message{
			To:   &validatorAddr,
			Func: validateFunc,
			Args: []any{
				validateFuncArgs{
					ProxyAdminAddress:   input.ProxyAdminAddress,
					SystemConfigAddress: input.SystemConfigAddress,
					AbsolutePrestate:    input.AbsolutePrestate,
					ChainID:             input.L2ChainID,
				},
				true,
			},
		}, nil, nil).Returns(&rawOutput),
	); err != nil {
		return nil, fmt.Errorf("failed to call validate: %w", err)
	}

	var output string
	if err := validateFunc.DecodeReturns(rawOutput, &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal output: %w", err)
	}
	return strings.Split(output, ","), nil
}

func NewV180Validator(client *rpc.Client) *BaseValidator {
	return newBaseValidator(client, VersionV180)
}

func NewV200Validator(client *rpc.Client) *BaseValidator {
	return newBaseValidator(client, VersionV200)
}
