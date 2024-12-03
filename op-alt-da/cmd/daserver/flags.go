package main

import (
	"errors"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

const (
	ListenAddrFlagName        = "addr"
	PortFlagName              = "port"
	S3BucketFlagName          = "s3.bucket"
	S3EndpointFlagName        = "s3.endpoint"
	S3AccessKeyIDFlagName     = "s3.access-key-id"
	S3AccessKeySecretFlagName = "s3.access-key-secret"
	FileStorePathFlagName     = "file.path"
	GenericCommFlagName       = "generic-commitment"
)

const EnvVarPrefix = "OP_ALTDA_SERVER"

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(EnvVarPrefix, name)
}

var (
	ListenAddrFlag = &cli.StringFlag{
		Name:     ListenAddrFlagName,
		Usage:    "server listening address",
		Value:    "127.0.0.1",
		EnvVars:  prefixEnvVars("ADDR"),
		Required: true,
	}
	PortFlag = &cli.IntFlag{
		Name:     PortFlagName,
		Usage:    "server listening port",
		Value:    3100,
		EnvVars:  prefixEnvVars("PORT"),
		Required: true,
	}
	FileStorePathFlag = &cli.StringFlag{
		Name:    FileStorePathFlagName,
		Usage:   "path to directory for file storage",
		EnvVars: prefixEnvVars("FILESTORE_PATH"),
	}
	GenericCommFlag = &cli.BoolFlag{
		Name:    GenericCommFlagName,
		Usage:   "enable generic commitments for testing. Not for production use.",
		EnvVars: prefixEnvVars("GENERIC_COMMITMENT"),
		Value:   false,
	}
	S3BucketFlag = &cli.StringFlag{
		Name:    S3BucketFlagName,
		Usage:   "bucket name for S3 storage",
		EnvVars: prefixEnvVars("S3_BUCKET"),
	}
	S3EndpointFlag = &cli.StringFlag{
		Name:    S3EndpointFlagName,
		Usage:   "endpoint for S3 storage",
		Value:   "",
		EnvVars: prefixEnvVars("S3_ENDPOINT"),
	}
	S3AccessKeyIDFlag = &cli.StringFlag{
		Name:    S3AccessKeyIDFlagName,
		Usage:   "access key id for S3 storage",
		Value:   "",
		EnvVars: prefixEnvVars("S3_ACCESS_KEY_ID"),
	}
	S3AccessKeySecretFlag = &cli.StringFlag{
		Name:    S3AccessKeySecretFlagName,
		Usage:   "access key secret for S3 storage",
		Value:   "",
		EnvVars: prefixEnvVars("S3_ACCESS_KEY_SECRET"),
	}
)

// Flags contains the list of configuration options available to the binary.
var Flags = []cli.Flag{
	ListenAddrFlag,
	PortFlag,
	FileStorePathFlag,
	S3BucketFlag,
	S3EndpointFlag,
	S3AccessKeyIDFlag,
	S3AccessKeySecretFlag,
	GenericCommFlag,
}

func init() {
	Flags = append(Flags, oplog.CLIFlags(EnvVarPrefix)...)
}

type CLIConfig struct {
	FileStoreDirPath  string
	S3Bucket          string
	S3Endpoint        string
	S3AccessKeyID     string
	S3AccessKeySecret string
	UseGenericComm    bool
}

func ReadCLIConfig(ctx *cli.Context) CLIConfig {
	return CLIConfig{
		FileStoreDirPath:  ctx.String(FileStorePathFlagName),
		S3Bucket:          ctx.String(S3BucketFlagName),
		S3Endpoint:        ctx.String(S3EndpointFlagName),
		S3AccessKeyID:     ctx.String(S3AccessKeyIDFlagName),
		S3AccessKeySecret: ctx.String(S3AccessKeySecretFlagName),
		UseGenericComm:    ctx.Bool(GenericCommFlagName),
	}
}

func (c CLIConfig) Check() error {
	if !c.S3Enabled() && !c.FileStoreEnabled() {
		return errors.New("at least one storage backend must be enabled")
	}
	if c.S3Enabled() && c.FileStoreEnabled() {
		return errors.New("only one storage backend can be enabled")
	}
	if c.S3Enabled() && (c.S3Bucket == "" || c.S3Endpoint == "" || c.S3AccessKeyID == "" || c.S3AccessKeySecret == "") {
		return errors.New("all S3 flags must be set")
	}
	return nil
}

func (c CLIConfig) S3Enabled() bool {
	return !(c.S3Bucket == "" && c.S3Endpoint == "" && c.S3AccessKeyID == "" && c.S3AccessKeySecret == "")
}

func (c CLIConfig) S3Config() S3Config {
	return S3Config{
		Bucket:          c.S3Bucket,
		Endpoint:        c.S3Endpoint,
		AccessKeyID:     c.S3AccessKeyID,
		AccessKeySecret: c.S3AccessKeySecret,
	}
}

func (c CLIConfig) FileStoreEnabled() bool {
	return c.FileStoreDirPath != ""
}
