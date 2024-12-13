package flags

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/urfave/cli/v2"
)

type FlagCreator func(name string, envVars []string, traceTypeInfo string) cli.Flag

// VMFlag defines a set of flags to set a VM specific option. Provides a flag to set the default plus flags to
// override the default on a per VM basis.
type VMFlag struct {
	vms          []types.TraceType
	name         string
	envVarPrefix string
	flagCreator  FlagCreator
}

func NewVMFlag(name string, envVarPrefix string, vms []types.TraceType, flagCreator FlagCreator) *VMFlag {
	return &VMFlag{
		name:         name,
		envVarPrefix: envVarPrefix,
		flagCreator:  flagCreator,
		vms:          vms,
	}
}

func (f *VMFlag) Flags() []cli.Flag {
	flags := make([]cli.Flag, 0, len(f.vms))
	// Default
	defaultEnvVar := opservice.FlagNameToEnvVarName(f.name, f.envVarPrefix)
	flags = append(flags, f.flagCreator(f.name, []string{defaultEnvVar}, ""))
	for _, vm := range f.vms {
		name := f.flagName(vm)
		envVar := opservice.FlagNameToEnvVarName(name, f.envVarPrefix)
		flags = append(flags, f.flagCreator(name, []string{envVar}, fmt.Sprintf("(%v trace type only)", vm)))
	}
	return flags
}

func (f *VMFlag) DefaultName() string {
	return f.name
}

func (f *VMFlag) IsSet(ctx *cli.Context, vm types.TraceType) bool {
	return ctx.IsSet(f.flagName(vm)) || ctx.IsSet(f.name)
}

func (f *VMFlag) String(ctx *cli.Context, vm types.TraceType) string {
	val := ctx.String(f.flagName(vm))
	if val == "" {
		val = ctx.String(f.name)
	}
	return val
}

func (f *VMFlag) SourceFlagName(ctx *cli.Context, vm types.TraceType) string {
	vmFlag := f.flagName(vm)
	if ctx.IsSet(vmFlag) {
		return vmFlag
	}
	return f.name
}

func (f *VMFlag) flagName(vm types.TraceType) string {
	return fmt.Sprintf("%v-%v", vm, f.name)
}
