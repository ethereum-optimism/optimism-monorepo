package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/util"
)

const FILESERVER_PACKAGE = "fileserver"

type FileServer struct {
	baseDir  string
	enclave  string
	dryRun   bool
	deployer DeployerFunc
}

func (f *FileServer) URL(path ...string) string {
	return fmt.Sprintf("http://%s/%s", FILESERVER_PACKAGE, strings.Join(path, "/"))
}

func (f *FileServer) Deploy(ctx context.Context, sourceDir string) error {
	// Create a temp dir in the fileserver package
	baseDir := filepath.Join(f.baseDir, FILESERVER_PACKAGE)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("error creating base directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(baseDir, "upload-content")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Copy build dir contents to tempDir
	if err := util.CopyDir(sourceDir, tempDir); err != nil {
		return fmt.Errorf("error copying directory: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteString(fmt.Sprintf("source_path: %s\n", filepath.Base(tempDir)))

	opts := []kurtosis.KurtosisDeployerOptions{
		kurtosis.WithKurtosisBaseDir(f.baseDir),
		kurtosis.WithKurtosisDryRun(f.dryRun),
		kurtosis.WithKurtosisPackageName(FILESERVER_PACKAGE),
		kurtosis.WithKurtosisEnclave(f.enclave),
	}

	d, err := f.deployer(opts...)
	if err != nil {
		return fmt.Errorf("error creating kurtosis deployer: %w", err)
	}

	_, err = d.Deploy(ctx, buf)
	if err != nil {
		return fmt.Errorf("error deploying kurtosis package: %w", err)
	}

	return nil
}
