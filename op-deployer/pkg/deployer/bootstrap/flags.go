package bootstrap

import (
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
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
)

var OPCMFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	ArtifactsLocatorFlag,
	WithdrawalDelaySecondsFlag,
	MinProposalSizeBytesFlag,
	ChallengePeriodSecondsFlag,
	ProofMaturityDelaySecondsFlag,
	DisputeGameFinalityDelaySecondsFlag,
	MIPSVersionFlag,
}

var DelayedWETHFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	ArtifactsLocatorFlag,
}

var Commands = []*cli.Command{
	{
		Name:   "opcm",
		Usage:  "Bootstrap an instance of OPCM.",
		Flags:  cliapp.ProtectFlags(OPCMFlags),
		Action: OPCMCLI,
	},
	{
		Name:   "delayedweth",
		Usage:  "Bootstrap an instance of DelayedWETH.",
		Flags:  cliapp.ProtectFlags(DelayedWETHFlags),
		Action: DelayedWETHCLI,
	},
}
