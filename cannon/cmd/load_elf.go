package cmd

import (
	"debug/elf"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/singlethreaded"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/versions"
	openum "github.com/ethereum-optimism/optimism/op-service/enum"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/ethereum-optimism/optimism/op-service/serialize"
)

var (
	LoadELFVMTypeFlag = &cli.StringFlag{
		Name:     "type",
		Usage:    "VM type to create state for. Valid options: " + openum.EnumString(stateVersions()),
		Required: true,
	}
	LoadELFPathFlag = &cli.PathFlag{
		Name:      "path",
		Usage:     "Path to 32/64-bit big-endian MIPS ELF file",
		TakesFile: true,
		Required:  true,
	}
	LoadELFOutFlag = &cli.PathFlag{
		Name:     "out",
		Usage:    "Output path to write state to. State is dumped to stdout if set to '-'. Not written if empty. Use file extension '.bin', '.bin.gz', or '.json' for binary, compressed binary, or JSON formats.",
		Value:    "state.bin.gz",
		Required: false,
	}
	LoadELFMetaFlag = &cli.PathFlag{
		Name:     "meta",
		Usage:    "Write metadata file, for symbol lookup during program execution. None if empty.",
		Value:    "meta.json",
		Required: false,
	}
)

func stateVersions() []string {
	vers := make([]string, len(versions.StateVersionTypes))
	for i, v := range versions.StateVersionTypes {
		vers[i] = v.String()
	}
	return vers
}

func LoadELF(ctx *cli.Context) error {
	elfPath := ctx.Path(LoadELFPathFlag.Name)
	elfProgram, err := elf.Open(elfPath)
	if err != nil {
		return fmt.Errorf("failed to open ELF file %q: %w", elfPath, err)
	}
	if elfProgram.Machine != elf.EM_MIPS {
		return fmt.Errorf("ELF is not big-endian MIPS R3000, but got %q", elfProgram.Machine.String())
	}

	var createInitialState func(f *elf.File) (mipsevm.FPVMState, error)

	var patcher = program.PatchStack
	ver, err := versions.ParseStateVersion(ctx.String(LoadELFVMTypeFlag.Name))
	if err != nil {
		return err
	}
	switch ver {
	case versions.VersionSingleThreaded2:
		createInitialState = func(f *elf.File) (mipsevm.FPVMState, error) {
			return program.LoadELF(f, singlethreaded.CreateInitialState)
		}
		patcher = func(state mipsevm.FPVMState) error {
			err := program.PatchGoGC(elfProgram, state)
			if err != nil {
				return err
			}
			return program.PatchStack(state)
		}
	case versions.VersionMultiThreaded, versions.VersionMultiThreaded64:
		createInitialState = func(f *elf.File) (mipsevm.FPVMState, error) {
			return program.LoadELF(f, multithreaded.CreateInitialState)
		}
	default:
		return fmt.Errorf("unsupported state version: %d (%s)", ver, ver.String())
	}

	state, err := createInitialState(elfProgram)
	if err != nil {
		return fmt.Errorf("failed to load ELF data into VM state: %w", err)
	}
	err = patcher(state)
	if err != nil {
		return fmt.Errorf("failed to patch state: %w", err)
	}
	meta, err := program.MakeMetadata(elfProgram)
	if err != nil {
		return fmt.Errorf("failed to compute program metadata: %w", err)
	}
	if err := jsonutil.WriteJSON[*program.Metadata](meta, ioutil.ToStdOutOrFileOrNoop(ctx.Path(LoadELFMetaFlag.Name), OutFilePerm)); err != nil {
		return fmt.Errorf("failed to output metadata: %w", err)
	}

	// Ensure the state is written with appropriate version information
	versionedState, err := versions.NewFromState(state)
	if err != nil {
		return fmt.Errorf("failed to create versioned state: %w", err)
	}
	return serialize.Write(ctx.Path(LoadELFOutFlag.Name), versionedState, OutFilePerm)
}

func CreateLoadELFCommand(action cli.ActionFunc) *cli.Command {
	return &cli.Command{
		Name:        "load-elf",
		Usage:       "Load ELF file into Cannon state",
		Description: "Load ELF file into Cannon state",
		Action:      action,
		Flags: []cli.Flag{
			LoadELFVMTypeFlag,
			LoadELFPathFlag,
			LoadELFOutFlag,
			LoadELFMetaFlag,
		},
	}
}

var LoadELFCommand = CreateLoadELFCommand(LoadELF)
