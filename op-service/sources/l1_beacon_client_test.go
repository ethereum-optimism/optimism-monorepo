package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"testing"

	client_mocks "github.com/ethereum-optimism/optimism/op-service/client/mocks"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/mocks"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/stretchr/testify/require"
)

func makeTestBlobSidecar(index uint64) (eth.IndexedBlobHash, *eth.BlobSidecar) {
	blob := kzg4844.Blob{}
	// make first byte of test blob match its index so we can easily verify if is returned in the
	// expected order
	blob[0] = byte(index)
	commit, _ := kzg4844.BlobToCommitment(&blob)
	proof, _ := kzg4844.ComputeBlobProof(&blob, commit)
	hash := eth.KZGToVersionedHash(commit)

	idh := eth.IndexedBlobHash{
		Index: index,
		Hash:  hash,
	}
	sidecar := eth.BlobSidecar{
		Index:         eth.Uint64String(index),
		Blob:          eth.Blob(blob),
		KZGCommitment: eth.Bytes48(commit),
		KZGProof:      eth.Bytes48(proof),
	}
	return idh, &sidecar
}

func TestBlobsFromSidecars(t *testing.T) {
	indices := []uint64{5, 7, 2}

	// blobs should be returned in order of their indices in the hashes array regardless
	// of the sidecar ordering
	index0, sidecar0 := makeTestBlobSidecar(indices[0])
	index1, sidecar1 := makeTestBlobSidecar(indices[1])
	index2, sidecar2 := makeTestBlobSidecar(indices[2])

	hashes := []eth.IndexedBlobHash{index0, index1, index2}

	// put the sidecars in scrambled order to confirm error
	sidecars := []*eth.BlobSidecar{sidecar2, sidecar0, sidecar1}
	_, err := blobsFromSidecars(sidecars, hashes)
	require.Error(t, err)

	// too few sidecars should error
	sidecars = []*eth.BlobSidecar{sidecar0, sidecar1}
	_, err = blobsFromSidecars(sidecars, hashes)
	require.Error(t, err)

	// correct order should work
	sidecars = []*eth.BlobSidecar{sidecar0, sidecar1, sidecar2}
	blobs, err := blobsFromSidecars(sidecars, hashes)
	require.NoError(t, err)
	// confirm order by checking first blob byte against expected index
	for i := range blobs {
		require.Equal(t, byte(indices[i]), blobs[i][0])
	}

	// mangle a proof to make sure it's detected
	badProof := *sidecar0
	badProof.KZGProof[11]++
	sidecars[1] = &badProof
	_, err = blobsFromSidecars(sidecars, hashes)
	require.Error(t, err)

	// mangle a commitment to make sure it's detected
	badCommitment := *sidecar0
	badCommitment.KZGCommitment[13]++
	sidecars[1] = &badCommitment
	_, err = blobsFromSidecars(sidecars, hashes)
	require.Error(t, err)

	// mangle a hash to make sure it's detected
	sidecars[1] = sidecar0
	hashes[2].Hash[17]++
	_, err = blobsFromSidecars(sidecars, hashes)
	require.Error(t, err)
}

func TestBlobsFromSidecars_EmptySidecarList(t *testing.T) {
	hashes := []eth.IndexedBlobHash{}
	sidecars := []*eth.BlobSidecar{}
	blobs, err := blobsFromSidecars(sidecars, hashes)
	require.NoError(t, err)
	require.Empty(t, blobs, "blobs should be empty when no sidecars are provided")
}

func toAPISideCars(sidecars []*eth.BlobSidecar) []*eth.APIBlobSidecar {
	var out []*eth.APIBlobSidecar
	for _, s := range sidecars {
		out = append(out, &eth.APIBlobSidecar{
			Index:             s.Index,
			Blob:              s.Blob,
			KZGCommitment:     s.KZGCommitment,
			KZGProof:          s.KZGProof,
			SignedBlockHeader: eth.SignedBeaconBlockHeader{},
		})
	}
	return out
}

func TestBeaconClientNoErrorPrimary(t *testing.T) {
	indices := []uint64{5, 7, 2}
	index0, sidecar0 := makeTestBlobSidecar(indices[0])
	index1, sidecar1 := makeTestBlobSidecar(indices[1])
	index2, sidecar2 := makeTestBlobSidecar(indices[2])

	hashes := []eth.IndexedBlobHash{index0, index1, index2}
	sidecars := []*eth.BlobSidecar{sidecar0, sidecar1, sidecar2}
	apiSidecars := toAPISideCars(sidecars)

	ctx := context.Background()
	p := mocks.NewBeaconClient(t)
	f := mocks.NewBlobSideCarsFetcher(t)
	c := NewL1BeaconClient(p, L1BeaconClientConfig{}, f)
	p.EXPECT().BeaconGenesis(ctx).Return(eth.APIGenesisResponse{Data: eth.ReducedGenesisData{GenesisTime: 10}}, nil)
	p.EXPECT().ConfigSpec(ctx).Return(eth.APIConfigResponse{Data: eth.ReducedConfigData{SecondsPerSlot: 2}}, nil)
	// Timestamp 12 = Slot 1
	p.EXPECT().BeaconBlobSideCars(ctx, false, uint64(1), hashes).Return(eth.APIGetBlobSidecarsResponse{Data: apiSidecars}, nil)

	resp, err := c.GetBlobSidecars(ctx, eth.L1BlockRef{Time: 12}, hashes)
	require.Equal(t, sidecars, resp)
	require.NoError(t, err)
}

func TestBeaconClientFallback(t *testing.T) {
	indices := []uint64{5, 7, 2}
	index0, sidecar0 := makeTestBlobSidecar(indices[0])
	index1, sidecar1 := makeTestBlobSidecar(indices[1])
	index2, sidecar2 := makeTestBlobSidecar(indices[2])

	hashes := []eth.IndexedBlobHash{index0, index1, index2}
	sidecars := []*eth.BlobSidecar{sidecar0, sidecar1, sidecar2}
	apiSidecars := toAPISideCars(sidecars)

	ctx := context.Background()
	p := mocks.NewBeaconClient(t)
	f := mocks.NewBlobSideCarsFetcher(t)
	c := NewL1BeaconClient(p, L1BeaconClientConfig{}, f)
	p.EXPECT().BeaconGenesis(ctx).Return(eth.APIGenesisResponse{Data: eth.ReducedGenesisData{GenesisTime: 10}}, nil)
	p.EXPECT().ConfigSpec(ctx).Return(eth.APIConfigResponse{Data: eth.ReducedConfigData{SecondsPerSlot: 2}}, nil)
	// Timestamp 12 = Slot 1
	p.EXPECT().BeaconBlobSideCars(ctx, false, uint64(1), hashes).Return(eth.APIGetBlobSidecarsResponse{}, errors.New("404 not found"))
	f.EXPECT().BeaconBlobSideCars(ctx, false, uint64(1), hashes).Return(eth.APIGetBlobSidecarsResponse{Data: apiSidecars}, nil)

	resp, err := c.GetBlobSidecars(ctx, eth.L1BlockRef{Time: 12}, hashes)
	require.Equal(t, sidecars, resp)
	require.NoError(t, err)

	// Second set of calls. This time rotate back to the primary
	indices = []uint64{3, 9, 11}
	index0, sidecar0 = makeTestBlobSidecar(indices[0])
	index1, sidecar1 = makeTestBlobSidecar(indices[1])
	index2, sidecar2 = makeTestBlobSidecar(indices[2])

	hashes = []eth.IndexedBlobHash{index0, index1, index2}
	sidecars = []*eth.BlobSidecar{sidecar0, sidecar1, sidecar2}
	apiSidecars = toAPISideCars(sidecars)

	// Timestamp 14 = Slot 2
	f.EXPECT().BeaconBlobSideCars(ctx, false, uint64(2), hashes).Return(eth.APIGetBlobSidecarsResponse{}, errors.New("404 not found"))
	p.EXPECT().BeaconBlobSideCars(ctx, false, uint64(2), hashes).Return(eth.APIGetBlobSidecarsResponse{Data: apiSidecars}, nil)

	resp, err = c.GetBlobSidecars(ctx, eth.L1BlockRef{Time: 14}, hashes)
	require.Equal(t, sidecars, resp)
	require.NoError(t, err)

}

func TestBeaconHTTPClient(t *testing.T) {
	c := client_mocks.NewHTTP(t)
	b := NewBeaconHTTPClient(c)

	ctx := context.Background()

	indices := []uint64{3, 9, 11}
	index0, _ := makeTestBlobSidecar(indices[0])
	index1, _ := makeTestBlobSidecar(indices[1])
	index2, _ := makeTestBlobSidecar(indices[2])

	hashes := []eth.IndexedBlobHash{index0, index1, index2}

	// mocks returning a 200 with empty list
	respBytes, _ := json.Marshal(eth.APIGetBlobSidecarsResponse{})
	slot := uint64(2)
	path := path.Join(sidecarsMethodPrefix, strconv.FormatUint(slot, 10))
	reqQuery := url.Values{}
	for i := range hashes {
		reqQuery.Add("indices", strconv.FormatUint(hashes[i].Index, 10))
	}
	headers := http.Header{}
	headers.Add("Accept", "application/json")
	c.EXPECT().Get(ctx, path, reqQuery, headers).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(respBytes))}, nil)

	// BeaconBlobSideCars should return error when client.HTTP returns a 200 with empty list
	_, err := b.BeaconBlobSideCars(ctx, false, slot, hashes)
	require.Error(t, err)
	require.Equal(t, err.Error(), fmt.Sprintf("#returned blobs(%d) != #requested blobs(%d)", 0, len(hashes)))
}

func TestClientPoolSingle(t *testing.T) {
	p := NewClientPool(1)
	for i := 0; i < 10; i++ {
		require.Equal(t, 1, p.Get())
		p.MoveToNext()
	}
}
func TestClientPoolSeveral(t *testing.T) {
	p := NewClientPool(0, 1, 2, 3)
	for i := 0; i < 25; i++ {
		require.Equal(t, i%4, p.Get())
		p.MoveToNext()
	}
}
