package versions

import (
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/serialize"
)

func DetectVersion(path string) (StateVersion, error) {
	if !serialize.IsBinaryFile(path) {
		return VersionSingleThreaded, nil
	}

	var f io.ReadCloser
	f, err := ioutil.OpenDecompressed(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %q: %w", path, err)
	}
	defer f.Close()

	var ver StateVersion
	bin := serialize.NewBinaryReader(f)
	if err := bin.ReadUInt(&ver); err != nil {
		return 0, err
	}

	switch ver {
	case VersionSingleThreaded, VersionMultiThreaded, VersionSingleThreaded2, VersionMultiThreaded64, VersionMultiThreaded64_v2, VersionMultiThreaded_v2, VersionMultiThreaded64_v3:
		return ver, nil
	default:
		return 0, fmt.Errorf("%w: %d", ErrUnknownVersion, ver)
	}
}
