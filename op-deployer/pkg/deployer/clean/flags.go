package clean

import (
	"github.com/urfave/cli/v2"
)

var Commands = []*cli.Command{
	{
		Name:   "cache",
		Usage:  "Cleans the backing the backe of op-deployer for all the previous runs",
		Action: CacheCLI,
	},
}
