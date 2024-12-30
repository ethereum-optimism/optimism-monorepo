package sync

import (
	"github.com/ethereum/go-ethereum/log"
)

const (
	DBLocalSafe Database = "local_safe"
	DBCrossSafe Database = "cross_safe"
)

// Databases maps a database alias to its actual name on disk
var Databases = map[Database]string{
	DBLocalSafe: "local_safe.db",
	DBCrossSafe: "cross_safe.db",
}

type Database string

func (d Database) String() string {
	return string(d)
}

func (d Database) File() string {
	return Databases[d]
}

// Config contains all configuration for the Server or Client.
type Config struct {
	DataDir string
	Logger  log.Logger
}
