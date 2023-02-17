package op_e2e

import (
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/stretchr/testify/require"
)

func BuildErigon(t *testing.T) string {
	buildPath := filepath.Join(t.TempDir(), "erigon")

	gt := gomega.NewWithT(t)
	cmd := exec.Command("go", "build", "-o", buildPath, "github.com/ledgerwatch/erigon/cmd/erigon")
	cmd.Dir = filepath.Join(".", "erigon-build-hack")
	sess, err := gexec.Start(cmd, os.Stdout, os.Stderr)
	gt.Expect(err).NotTo(gomega.HaveOccurred())
	gt.Eventually(sess, time.Minute).Should(gexec.Exit(0))

	return buildPath
}

type ErigonRunner struct {
	Name    string
	BinPath string
	DataDir string
	WebPort int
	JWTPath string
	ChainID uint64
	Genesis *core.Genesis
}

func (er *ErigonRunner) Run(t *testing.T) ErigonInstance {
	if er.BinPath == "" {
		er.BinPath = BuildErigon(t)
	}

	if er.DataDir == "" {
		er.DataDir = t.TempDir()
	}

	if er.JWTPath == "" {
		er.JWTPath = writeDefaultJWT(t)
	}

	if er.ChainID == 0 {
		er.ChainID = 901
	}

	if er.Genesis == nil {
		er.Genesis = &core.Genesis{
			Alloc: core.GenesisAlloc{
				common.Address{1}: core.GenesisAccount{Balance: big.NewInt(1)},
			},
			Config:     &params.ChainConfig{ChainID: big.NewInt(901)},
			Difficulty: big.NewInt(0),
		}
	}

	genesisPath := filepath.Join(er.DataDir, "genesis.json")
	o, err := os.Create(genesisPath)
	require.NoError(t, err)
	err = json.NewEncoder(o).Encode(er.Genesis)
	require.NoError(t, err)

	gt := gomega.NewWithT(t)
	cmd := exec.Command(
		er.BinPath,
		"--datadir", er.DataDir,
		"init", genesisPath,
	)
	sess, err := gexec.Start(cmd, os.Stdout, os.Stderr)
	gt.Expect(err).NotTo(gomega.HaveOccurred())
	gt.Eventually(sess.Err, time.Minute).Should(gbytes.Say("Successfully wrote genesis state"))

	cmd = exec.Command(
		er.BinPath,
		"--chain", "dev",
		"--datadir", er.DataDir,
		"--log.console.verbosity", "dbug",
		"--externalcl",
		"--ws",
		"--mine",
		// "--miner.etherbase=0x123463a4B065722E99115D6c222f267d9cABb524",
		// "--miner.sigfile", "/home/boba/datadir/nodekey",
		"--http.port", strconv.Itoa(er.WebPort),
		"--http.addr", "127.0.0.1",
		"--http.api", "eth,debug,net,engine,erigon,web3",
		"--private.api.addr=127.0.0.1:9090",
		"--allow-insecure-unlock",
		"--authrpc.addr=127.0.0.1",
		"--authrpc.port=8551",
		"--authrpc.vhosts=*",
		"--authrpc.jwtsecret", er.JWTPath,
		"--networkid", "901",
	)
	sess, err = gexec.Start(
		cmd,
		gexec.NewPrefixedWriter(er.Name, os.Stdout),
		gexec.NewPrefixedWriter(er.Name, os.Stderr),
	)
	gt.Expect(err).NotTo(gomega.HaveOccurred())
	gt.Eventually(sess.Err, time.Minute).Should(gbytes.Say("\\[15/15 Finish\\] DONE"))

	return ErigonInstance{
		Session: sess,
		Runner:  er,
	}
}

type ErigonInstance struct {
	Session *gexec.Session
	Runner  *ErigonRunner
}

func (ei *ErigonInstance) Shutdown() {
	ei.Session.Terminate()
	select {
	case <-time.After(5 * time.Second):
		ei.Session.Kill()
	case <-ei.Session.Exited:
	}
}
