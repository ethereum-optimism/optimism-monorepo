package runner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/prestates"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/common"
)

type OnChainPrestateFetcher struct {
	m                  metrics.ContractMetricer
	gameFactoryAddress common.Address
	gameType           types.GameType
	caller             *batching.MultiCaller
}

func (f *OnChainPrestateFetcher) getPrestate(ctx context.Context, prestateBaseUrl *url.URL, prestatePath string, dataDir string, stateConverter vm.StateConverter) (string, error) {
	gameFactory := contracts.NewDisputeGameFactoryContract(f.m, f.gameFactoryAddress, f.caller)
	gameImplAddr, err := gameFactory.GetGameImpl(ctx, f.gameType)
	if err != nil {
		return "", fmt.Errorf("failed to load game impl: %w", err)
	}
	if gameImplAddr == (common.Address{}) {
		return "", nil // No prestate is set, will only work if a single prestate is specified
	}
	gameImpl, err := contracts.NewFaultDisputeGameContract(ctx, f.m, gameImplAddr, f.caller)
	if err != nil {
		return "", fmt.Errorf("failed to create fault dispute game contract bindings for %v: %w", gameImplAddr, err)
	}
	prestateHash, err := gameImpl.GetAbsolutePrestateHash(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute prestate hash for %v: %w", gameImplAddr, err)
	}
	hashFetcher := &HashPrestateFetcher{prestateHash: prestateHash}
	return hashFetcher.getPrestate(ctx, prestateBaseUrl, prestatePath, dataDir, stateConverter)
}

type HashPrestateFetcher struct {
	prestateHash common.Hash
}

func (f *HashPrestateFetcher) getPrestate(ctx context.Context, prestateBaseUrl *url.URL, prestatePath string, dataDir string, stateConverter vm.StateConverter) (string, error) {
	prestateSource := prestates.NewPrestateSource(
		prestateBaseUrl,
		prestatePath,
		filepath.Join(dataDir, "prestates"),
		stateConverter)

	prestate, err := prestateSource.PrestatePath(ctx, f.prestateHash)
	if err != nil {
		return "", fmt.Errorf("failed to get prestate %v: %w", f.prestateHash, err)
	}
	return prestate, nil
}

// NamedPrestateFetcher downloads a file with a specified name from the prestate base URL and uses it as the prestate.
// The file is re-downloaded on each run rather than being cached. This makes it possible to run the latest builds
// from develop.
type NamedPrestateFetcher struct {
	filename string
}

func (f *NamedPrestateFetcher) getPrestate(ctx context.Context, prestateBaseUrl *url.URL, _ string, dataDir string, _ vm.StateConverter) (string, error) {
	targetDir := filepath.Join(dataDir, "prestates")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("error creating prestate dir: %w", err)
	}
	prestateUrl := prestateBaseUrl.JoinPath(f.filename)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", prestateUrl.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create prestate request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch prestate from %v: %w", prestateUrl, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w from url %v: status %v", prestates.ErrPrestateUnavailable, prestateUrl, resp.StatusCode)
	}

	targetFile := filepath.Join(targetDir, f.filename)
	out, err := os.Create(targetFile)
	if err != nil {
		return "", fmt.Errorf("failed to create prestate file %v: %w", targetFile, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write prestate to %v: %w", targetFile, err)
	}
	return targetFile, nil
}
