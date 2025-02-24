package bootstrap

import (
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli/v2"
)

const (
	OutfileFlagName                         = "outfile"
	ArtifactsLocatorFlagName                = "artifacts-locator"
	WithdrawalDelaySecondsFlagName          = "withdrawal-delay-seconds"
	MinProposalSizeBytesFlagName            = "min-proposal-size-bytes"
	ChallengePeriodSecondsFlagName          = "challenge-period-seconds"
	ProofMaturityDelaySecondsFlagName       = "proof-maturity-delay-seconds"
	DisputeGameFinalityDelaySecondsFlagName = "dispute-game-finality-delay-seconds"
	MIPSVersionFlagName                     = "mips-version"
	ProxyOwnerFlagName                      = "proxy-owner"
	SuperchainProxyAdminOwnerFlagName       = "superchain-proxy-admin-owner"
	ProtocolVersionsOwnerFlagName           = "protocol-versions-owner"
	GuardianFlagName                        = "guardian"
	PausedFlagName                          = "paused"
	RequiredProtocolVersionFlagName         = "required-protocol-version"
	RecommendedProtocolVersionFlagName      = "recommended-protocol-version"
)

var (
	OutfileFlag = &cli.StringFlag{
		Name:    OutfileFlagName,
		Usage:   "Output file. Use - for stdout.",
		EnvVars: deployer.PrefixEnvVar("OUTFILE"),
		Value:   "-",
	}
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
		Usage:   "PreimageOracle minimum proposal size in bytes.",
		EnvVars: deployer.PrefixEnvVar("MIN_PROPOSAL_SIZE_BYTES"),
		Value:   standard.MinProposalSizeBytes,
	}
	ChallengePeriodSecondsFlag = &cli.Uint64Flag{
		Name:    ChallengePeriodSecondsFlagName,
		Usage:   "PreimageOracle challenge period in seconds.",
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
	ProxyOwnerFlag = &cli.StringFlag{
		Name:    ProxyOwnerFlagName,
		Usage:   "Proxy owner address.",
		EnvVars: deployer.PrefixEnvVar("PROXY_OWNER"),
		Value:   common.Address{}.Hex(),
	}
	SuperchainProxyAdminOwnerFlag = &cli.StringFlag{
		Name:    SuperchainProxyAdminOwnerFlagName,
		Usage:   "Owner address for the superchain proxy admin",
		EnvVars: deployer.PrefixEnvVar("SUPERCHAIN_PROXY_ADMIN_OWNER"),
		Value:   common.Address{}.Hex(),
	}
	ProtocolVersionsOwnerFlag = &cli.StringFlag{
		Name:    ProtocolVersionsOwnerFlagName,
		Usage:   "Owner address for protocol versions",
		EnvVars: deployer.PrefixEnvVar("PROTOCOL_VERSIONS_OWNER"),
		Value:   common.Address{}.Hex(),
	}
	GuardianFlag = &cli.StringFlag{
		Name:    GuardianFlagName,
		Usage:   "Guardian address",
		EnvVars: deployer.PrefixEnvVar("GUARDIAN"),
		Value:   common.Address{}.Hex(),
	}
	PausedFlag = &cli.BoolFlag{
		Name:    PausedFlagName,
		Usage:   "Initial paused state",
		EnvVars: deployer.PrefixEnvVar("PAUSED"),
	}
	RequiredProtocolVersionFlag = &cli.StringFlag{
		Name:    RequiredProtocolVersionFlagName,
		Usage:   "Required protocol version (semver)",
		EnvVars: deployer.PrefixEnvVar("REQUIRED_PROTOCOL_VERSION"),
	}
	RecommendedProtocolVersionFlag = &cli.StringFlag{
		Name:    RecommendedProtocolVersionFlagName,
		Usage:   "Recommended protocol version (semver)",
		EnvVars: deployer.PrefixEnvVar("RECOMMENDED_PROTOCOL_VERSION"),
	}
	L1ContractsReleaseFlag = &cli.StringFlag{
		Name:    "l1-contracts-release",
		Usage:   "Release version to set OPCM implementations for, of the format `op-contracts/vX.Y.Z`.",
		EnvVars: deployer.PrefixEnvVar("L1_CONTRACTS_RELEASE"),
	}
	SuperchainConfigProxyFlag = &cli.StringFlag{
		Name:    "superchain-config-proxy",
		Usage:   "Superchain config proxy.",
		EnvVars: deployer.PrefixEnvVar("SUPERCHAIN_CONFIG_PROXY"),
	}
	ProtocolVersionsProxyFlag = &cli.StringFlag{
		Name:    "protocol-versions-proxy",
		Usage:   "Protocol versions proxy.",
		EnvVars: deployer.PrefixEnvVar("PROTOCOL_VERSIONS_PROXY"),
	}
	UpgradeControllerFlag = &cli.StringFlag{
		Name:    "upgrade-controller",
		Usage:   "Upgrade controller.",
		EnvVars: deployer.PrefixEnvVar("UPGRADE_CONTROLLER"),
	}
	UseInteropFlag = &cli.BoolFlag{
		Name:    "use-interop",
		Usage:   "If true, deploy Interop implementations.",
		EnvVars: deployer.PrefixEnvVar("USE_INTEROP"),
	}
	ConfigFileFlag = &cli.StringFlag{
		Name:    "config",
		Usage:   "Path to a JSON file",
		EnvVars: deployer.PrefixEnvVar("CONFIG"),
	}
)

var ImplementationsFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	OutfileFlag,
	ArtifactsLocatorFlag,
	L1ContractsReleaseFlag,
	MIPSVersionFlag,
	WithdrawalDelaySecondsFlag,
	MinProposalSizeBytesFlag,
	ChallengePeriodSecondsFlag,
	ProofMaturityDelaySecondsFlag,
	DisputeGameFinalityDelaySecondsFlag,
	SuperchainConfigProxyFlag,
	ProtocolVersionsProxyFlag,
	UpgradeControllerFlag,
	UseInteropFlag,
}

var ProxyFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	OutfileFlag,
	ArtifactsLocatorFlag,
	ProxyOwnerFlag,
}

var SuperchainFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	OutfileFlag,
	ArtifactsLocatorFlag,
	SuperchainProxyAdminOwnerFlag,
	ProtocolVersionsOwnerFlag,
	GuardianFlag,
	PausedFlag,
	RequiredProtocolVersionFlag,
	RecommendedProtocolVersionFlag,
}

var ValidatorFlags = []cli.Flag{
	deployer.L1RPCURLFlag,
	deployer.PrivateKeyFlag,
	OutfileFlag,
	ArtifactsLocatorFlag,
	ConfigFileFlag,
}

var Commands = []*cli.Command{
	{
		Name:   "implementations",
		Usage:  "Bootstraps implementations.",
		Flags:  cliapp.ProtectFlags(ImplementationsFlags),
		Action: ImplementationsCLI,
		Hidden: true,
	},
	{
		Name:   "proxy",
		Usage:  "Bootstrap a ERC-1967 Proxy without an implementation set.",
		Flags:  cliapp.ProtectFlags(ProxyFlags),
		Action: ProxyCLI,
	},
	{
		Name:   "superchain",
		Usage:  "Bootstrap the Superchain configuration",
		Flags:  cliapp.ProtectFlags(SuperchainFlags),
		Action: SuperchainCLI,
	},
	{
		Name:   "validator",
		Usage:  "Bootstrap the StandardValidator contracts",
		Flags:  cliapp.ProtectFlags(ValidatorFlags),
		Action: ValidatorCLI,
	},
}
