package bootstrap

import (
	"errors"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

const (
	ArtifactsLocatorFlagName                = "artifacts-locator"
	WithdrawalDelaySecondsFlagName          = "withdrawal-delay-seconds"
	MinProposalSizeBytesFlagName            = "min-proposal-size-bytes"
	ChallengePeriodSecondsFlagName          = "challenge-period-seconds"
	ProofMaturityDelaySecondsFlagName       = "proof-maturity-delay-seconds"
	DisputeGameFinalityDelaySecondsFlagName = "dispute-game-finality-delay-seconds"
	MIPSVersionFlagName                     = "mips-version"
	VmFlagName                              = "vm"
	GameKindFlagName                        = "game-kind"
	GameTypeFlagName                        = "game-type"
	AbsolutePrestateFlagName                = "absolute-prestate"
	MaxGameDepthFlagName                    = "max-game-depth"
	SplitDepthFlagName                      = "split-depth"
	ClockExtensionFlagName                  = "clock-extension"
	MaxClockDurationFlagName                = "max-clock-duration"
	AnchorStateRegistryProxyFlagName        = "anchor-state-registry-proxy"
	L2ChainIdFlagName                       = "l2-chain-id"
	ProposerFlagName                        = "proposer"
	ChallengerFlagName                      = "challenger"
	PreimageOracleFlagName                  = "preimage-oracle"
	ReleaseFlagName                         = "release"
	DelayedWethProxyFlagName                = "delayed-weth-proxy"
	DelayedWethImplFlagName                 = "delayed-weth-impl"
)

var (
	ArtifactsLocatorFlag = &cli.StringFlag{
		Name:    ArtifactsLocatorFlagName,
		Usage:   "Locator for artifacts.",
		EnvVars: deployer.PrefixEnvVar("ARTIFACTS_LOCATOR"),
	}
	WithdrawalDelaySecondsFlag = &cli.Uint64Flag{
		Name:    WithdrawalDelaySecondsFlagName,
		Usage:   "Withdrawal delay in seconds.",
		EnvVars: deployer.PrefixEnvVar("WITHDRAWAL_DELAY_SECONDS"),
		Value:   standard.WithdrawalDelaySeconds,
	}
	MinProposalSizeBytesFlag = &cli.Uint64Flag{
		Name:    MinProposalSizeBytesFlagName,
		Usage:   "Minimum proposal size in bytes.",
		EnvVars: deployer.PrefixEnvVar("MIN_PROPOSAL_SIZE_BYTES"),
		Value:   standard.MinProposalSizeBytes,
	}
	ChallengePeriodSecondsFlag = &cli.Uint64Flag{
		Name:    ChallengePeriodSecondsFlagName,
		Usage:   "Challenge period in seconds.",
		EnvVars: deployer.PrefixEnvVar("CHALLENGE_PERIOD_SECONDS"),
		Value:   standard.ChallengePeriodSeconds,
	}
	ProofMaturityDelaySecondsFlag = &cli.Uint64Flag{
		Name:    ProofMaturityDelaySecondsFlagName,
		Usage:   "Proof maturity delay in seconds.",
		EnvVars: deployer.PrefixEnvVar("PROOF_MATURITY_DELAY_SECONDS"),
		Value:   standard.ProofMaturityDelaySeconds,
	}
	DisputeGameFinalityDelaySecondsFlag = &cli.Uint64Flag{
		Name:    DisputeGameFinalityDelaySecondsFlagName,
		Usage:   "Dispute game finality delay in seconds.",
		EnvVars: deployer.PrefixEnvVar("DISPUTE_GAME_FINALITY_DELAY_SECONDS"),
		Value:   standard.DisputeGameFinalityDelaySeconds,
	}
	MIPSVersionFlag = &cli.Uint64Flag{
		Name:    MIPSVersionFlagName,
		Usage:   "MIPS version.",
		EnvVars: deployer.PrefixEnvVar("MIPS_VERSION"),
		Value:   standard.MIPSVersion,
	}
	VmFlag = &cli.StringFlag{
		Name:    VmFlagName,
		Usage:   "VM contract address.",
		EnvVars: deployer.PrefixEnvVar("VM"),
	}
	GameKindFlag = &cli.StringFlag{
		Name:    GameKindFlagName,
		Usage:   "Game kind (FaultDisputeGame or PermissionedDisputeGame).",
		EnvVars: deployer.PrefixEnvVar("GAME_KIND"),
		Value:   "FaultDisputeGame",
	}
	GameTypeFlag = &cli.StringFlag{
		Name:    GameTypeFlagName,
		Usage:   "Game type (integer or fractional).",
		EnvVars: deployer.PrefixEnvVar("GAME_TYPE"),
	}
	AbsolutePrestateFlag = &cli.StringFlag{
		Name:    AbsolutePrestateFlagName,
		Usage:   "Absolute prestate.",
		EnvVars: deployer.PrefixEnvVar("ABSOLUTE_PRESTATE"),
		Value:   standard.DisputeAbsolutePrestate.Hex(),
	}
	MaxGameDepthFlag = &cli.Uint64Flag{
		Name:    MaxGameDepthFlagName,
		Usage:   "Max game depth.",
		EnvVars: deployer.PrefixEnvVar("MAX_GAME_DEPTH"),
		Value:   standard.DisputeMaxGameDepth,
	}
	SplitDepthFlag = &cli.Uint64Flag{
		Name:    SplitDepthFlagName,
		Usage:   "Split depth.",
		EnvVars: deployer.PrefixEnvVar("SPLIT_DEPTH"),
		Value:   standard.DisputeSplitDepth,
	}
	ClockExtensionFlag = &cli.Uint64Flag{
		Name:    ClockExtensionFlagName,
		Usage:   "Clock extension.",
		EnvVars: deployer.PrefixEnvVar("CLOCK_EXTENSION"),
		Value:   standard.DisputeClockExtension,
	}
	MaxClockDurationFlag = &cli.Uint64Flag{
		Name:    MaxClockDurationFlagName,
		Usage:   "Max clock duration.",
		EnvVars: deployer.PrefixEnvVar("MAX_CLOCK_DURATION"),
		Value:   standard.DisputeMaxClockDuration,
	}
	DelayedWethProxyFlag = &cli.StringFlag{
		Name:    DelayedWethProxyFlagName,
		Usage:   "Delayed WETH proxy.",
		EnvVars: deployer.PrefixEnvVar("DELAYED_WETH_PROXY"),
	}
	DelayedWethImplFlag = &cli.StringFlag{
		Name:    DelayedWethImplFlagName,
		Usage:   "Delayed WETH implementation.",
		EnvVars: deployer.PrefixEnvVar("DELAYED_WETH_IMPL"),
		Value:   common.Address{}.Hex(),
	}
	AnchorStateRegistryProxyFlag = &cli.StringFlag{
		Name:    AnchorStateRegistryProxyFlagName,
		Usage:   "Anchor state registry proxy.",
		EnvVars: deployer.PrefixEnvVar("ANCHOR_STATE_REGISTRY_PROXY"),
	}
	L2ChainIdFlag = &cli.Uint64Flag{
		Name:    L2ChainIdFlagName,
		Usage:   "L2 chain ID.",
		EnvVars: deployer.PrefixEnvVar("L2_CHAIN_ID"),
	}
	ProposerFlag = &cli.StringFlag{
		Name:    ProposerFlagName,
		Usage:   "Proposer address (permissioned game only).",
		EnvVars: deployer.PrefixEnvVar("PROPOSER"),
		Value:   common.Address{}.Hex(),
	}
	ChallengerFlag = &cli.StringFlag{
		Name:    ChallengerFlagName,
		Usage:   "Challenger address (permissioned game only).",
		EnvVars: deployer.PrefixEnvVar("CHALLENGER"),
		Value:   common.Address{}.Hex(),
	}
	PreimageOracleFlag = &cli.StringFlag{
		Name:    PreimageOracleFlagName,
		Usage:   "Preimage oracle address.",
		EnvVars: deployer.PrefixEnvVar("PREIMAGE_ORACLE"),
		Value:   common.Address{}.Hex(),
	}
	ReleaseFlag = &cli.StringFlag{
		Name:    ReleaseFlagName,
		Usage:   "Release to deploy.",
		EnvVars: deployer.PrefixEnvVar("RELEASE"),
	}
)

var OPCMFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	ReleaseFlag,
}

var ImplementationsFlags = []cli.Flag{
	MIPSVersionFlag,
	WithdrawalDelaySecondsFlag,
	MinProposalSizeBytesFlag,
	ChallengePeriodSecondsFlag,
	ProofMaturityDelaySecondsFlag,
	DisputeGameFinalityDelaySecondsFlag,
}

var DelayedWETHFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	ArtifactsLocatorFlag,
	DelayedWethImplFlag,
}

var DisputeGameFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	ArtifactsLocatorFlag,
	MinProposalSizeBytesFlag,
	ChallengePeriodSecondsFlag,
	VmFlag,
	GameKindFlag,
	GameTypeFlag,
	AbsolutePrestateFlag,
	MaxGameDepthFlag,
	SplitDepthFlag,
	ClockExtensionFlag,
	MaxClockDurationFlag,
	DelayedWethProxyFlag,
	AnchorStateRegistryProxyFlag,
	L2ChainIdFlag,
	ProposerFlag,
	ChallengerFlag,
}

var BaseFPVMFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	ArtifactsLocatorFlag,
	PreimageOracleFlag,
}

var MIPSFlags = append(BaseFPVMFlags, MIPSVersionFlag)

var AsteriscFlags = BaseFPVMFlags

var Commands = []*cli.Command{
	{
		Name:   "opcm",
		Usage:  "Bootstrap an instance of OPCM.",
		Flags:  cliapp.ProtectFlags(OPCMFlags),
		Action: OPCMCLI,
	},
	{
		Name:  "implementations",
		Usage: "Bootstraps implementations.",
		Flags: cliapp.ProtectFlags(ImplementationsFlags),
		Action: func(context *cli.Context) error {
			return errors.New("not implemented yet")
		},
		Hidden: true,
	},
	{
		Name:   "delayedweth",
		Usage:  "Bootstrap an instance of DelayedWETH.",
		Flags:  cliapp.ProtectFlags(DelayedWETHFlags),
		Action: DelayedWETHCLI,
	},
	{
		Name:   "disputegame",
		Usage:  "Bootstrap an instance of a FaultDisputeGame or PermissionedDisputeGame.",
		Flags:  cliapp.ProtectFlags(DisputeGameFlags),
		Action: DisputeGameCLI,
	},
	{
		Name:   "mips",
		Usage:  "Bootstrap an instance of MIPS.",
		Flags:  cliapp.ProtectFlags(MIPSFlags),
		Action: MIPSCLI,
	},
	{
		Name:   "asterisc",
		Usage:  "Bootstrap an instance of Asterisc.",
		Flags:  cliapp.ProtectFlags(AsteriscFlags),
		Action: AsteriscCLI,
	},
}
