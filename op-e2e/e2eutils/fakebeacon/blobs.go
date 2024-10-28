package fakebeacon

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/log"
)

// FakeBeacon presents a beacon-node in testing, without leading any chain-building.
// This merely serves a fake beacon API, and holds on to blocks,
// to complement the actual block-building to happen in testing (e.g. through the fake consensus geth module).
type FakeBeacon struct {
	log log.Logger

	// in-memory blob store
	blobStore *e2eutils.BlobsStore
	blobsLock sync.Mutex

	beaconSrv         *http.Server
	beaconAPIListener net.Listener

	genesisTime uint64
	blockTime   uint64
}

func NewBeacon(log log.Logger, blobStore *e2eutils.BlobsStore, genesisTime uint64, blockTime uint64) *FakeBeacon {
	return &FakeBeacon{
		log:         log,
		blobStore:   blobStore,
		genesisTime: genesisTime,
		blockTime:   blockTime,
	}
}

func (f *FakeBeacon) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to open tcp listener for http beacon api server: %w", err)
	}
	f.beaconAPIListener = listener

	mux := new(http.ServeMux)
	mux.HandleFunc("/eth/v1/beacon/genesis", func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(&eth.APIGenesisResponse{Data: eth.ReducedGenesisData{GenesisTime: eth.Uint64String(f.genesisTime)}})
		if err != nil {
			f.log.Error("genesis handler err", "err", err)
		}
	})
	mux.HandleFunc("/eth/v1/config/spec", func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(&eth.APIConfigResponse{Data: eth.ReducedConfigData{SecondsPerSlot: eth.Uint64String(f.blockTime)}})
		if err != nil {
			f.log.Error("config handler err", "err", err)
		}
	})
	mux.HandleFunc("/eth/v1/beacon/blob_sidecars/", func(w http.ResponseWriter, r *http.Request) {
		blockID := strings.TrimPrefix(r.URL.Path, "/eth/v1/beacon/blob_sidecars/")
		slot, err := strconv.ParseUint(blockID, 10, 64)
		if err != nil {
			f.log.Error("could not parse block id from request", "url", r.URL.Path, "err", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		bundle, err := f.LoadBlobsBundle(slot)
		if errors.Is(err, ethereum.NotFound) {
			f.log.Error("failed to load blobs bundle - not found", "slot", slot, "err", err)
			w.WriteHeader(http.StatusNotFound)
			return
		} else if err != nil {
			f.log.Error("failed to load blobs bundle", "slot", slot, "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		query := r.URL.Query()
		rawIndices := query["indices"]
		indices := make([]uint64, 0, len(bundle.Blobs))
		if len(rawIndices) == 0 {
			// request is for all blobs
			for i := range bundle.Blobs {
				indices = append(indices, uint64(i))
			}
		} else {
			for _, raw := range rawIndices {
				ix, err := strconv.ParseUint(raw, 10, 64)
				if err != nil {
					f.log.Error("could not parse index from request", "url", r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				indices = append(indices, ix)
			}
		}

		var mockBeaconBlockRoot [32]byte
		mockBeaconBlockRoot[0] = 42
		binary.LittleEndian.PutUint64(mockBeaconBlockRoot[32-8:], slot)
		sidecars := make([]*eth.APIBlobSidecar, len(indices))
		for i, ix := range indices {
			if ix >= uint64(len(bundle.Blobs)) {
				f.log.Error("blob index from request is out of range", "url", r.URL)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			sidecars[i] = &eth.APIBlobSidecar{
				Index:         eth.Uint64String(ix),
				KZGCommitment: eth.Bytes48(bundle.Commitments[ix]),
				KZGProof:      eth.Bytes48(bundle.Proofs[ix]),
				SignedBlockHeader: eth.SignedBeaconBlockHeader{
					Message: eth.BeaconBlockHeader{
						StateRoot: mockBeaconBlockRoot,
						Slot:      eth.Uint64String(slot),
					},
				},
				InclusionProof: make([]eth.Bytes32, 0),
			}
			copy(sidecars[i].Blob[:], bundle.Blobs[ix])
		}
		if err := json.NewEncoder(w).Encode(&eth.APIGetBlobSidecarsResponse{Data: sidecars}); err != nil {
			f.log.Error("blobs handler err", "err", err)
		}
	})
	mux.HandleFunc("/eth/v1/node/version", func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(&eth.APIVersionResponse{Data: eth.VersionInformation{Version: "fakebeacon 1.2.3"}})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			f.log.Error("version handler err", "err", err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	f.beaconSrv = &http.Server{
		Handler:           mux,
		ReadTimeout:       time.Second * 20,
		ReadHeaderTimeout: time.Second * 20,
		WriteTimeout:      time.Second * 20,
		IdleTimeout:       time.Second * 20,
	}
	go func() {
		if err := f.beaconSrv.Serve(f.beaconAPIListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			f.log.Error("failed to start fake-pos beacon server for blobs testing", "err", err)
		}
	}()
	return nil
}

func (f *FakeBeacon) StoreBlobsBundle(slot uint64, bundle *engine.BlobsBundleV1) error {
	f.blobsLock.Lock()
	defer f.blobsLock.Unlock()

	// Solve for the slot timestamp.
	// slot = (timestamp - genesis) / slot_time
	// timestamp = slot * slot_time + genesis
	slotTimestamp := slot*f.blockTime + f.genesisTime

	for i, b := range bundle.Blobs {
		f.blobStore.StoreBlob(
			slotTimestamp,
			eth.IndexedBlobHash{
				Index: uint64(i),
				Hash:  eth.KZGToVersionedHash(kzg4844.Commitment(bundle.Commitments[i])),
			},
			(*eth.Blob)(b[:]),
		)
	}
	return nil
}

func (f *FakeBeacon) LoadBlobsBundle(slot uint64) (*engine.BlobsBundleV1, error) {
	f.blobsLock.Lock()
	defer f.blobsLock.Unlock()

	// Solve for the slot timestamp.
	// slot = (timestamp - genesis) / slot_time
	// timestamp = slot * slot_time + genesis
	slotTimestamp := slot*f.blockTime + f.genesisTime

	// Load blobs from the store
	blobs, err := f.blobStore.GetAllSidecars(context.Background(), slotTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to load blobs from store: %w", err)
	}

	// Convert blobs to the bundle
	out := engine.BlobsBundleV1{
		Commitments: make([]hexutil.Bytes, len(blobs)),
		Proofs:      make([]hexutil.Bytes, len(blobs)),
		Blobs:       make([]hexutil.Bytes, len(blobs)),
	}
	for _, b := range blobs {
		out.Commitments[b.Index] = hexutil.Bytes(b.KZGCommitment[:])
		out.Proofs[b.Index] = hexutil.Bytes(b.KZGProof[:])
		out.Blobs[b.Index] = hexutil.Bytes(b.Blob[:])
	}

	return &out, nil
}

func (f *FakeBeacon) Close() error {
	var out error
	if f.beaconSrv != nil {
		out = errors.Join(out, f.beaconSrv.Close())
	}
	if f.beaconAPIListener != nil {
		out = errors.Join(out, f.beaconAPIListener.Close())
	}
	return out
}

func (f *FakeBeacon) BeaconAddr() string {
	return "http://" + f.beaconAPIListener.Addr().String()
}
