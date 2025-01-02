package experimental

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

type TransactionWithMetadata struct {
	TxIndex     uint64             `json:"tx_index"`
	BlockNumber uint64             `json:"block_number"`
	BlockHash   common.Hash        `json:"block_hash"`
	BlockTime   uint64             `json:"block_time"`
	Sender      common.Address     `json:"sender"`
	ValidSender bool               `json:"valid_sender"`
	Frames      []derive.Frame     `json:"frames"`
	FrameErrs   []string           `json:"frame_parse_error"`
	ValidFrames []bool             `json:"valid_data"`
	Tx          *types.Transaction `json:"tx"`
}

type L1Entry struct {
	Header   sources.RPCHeader `json:"header"`
	BatchTxs []common.Hash     `json:"batchTxs"`
}

func onL1Block(cfg *rollup.Config, logger log.Logger,
	beacon *sources.L1BeaconClient, outDir string) (func(ctx context.Context, bl *sources.RPCBlock) error, error) {

	blocksDir := filepath.Join(outDir, "l1-blocks")
	txsDir := filepath.Join(outDir, "l1-txs")
	if err := os.MkdirAll(blocksDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to make blocks dir: %w", err)
	}
	if err := os.MkdirAll(txsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to make txs dir: %w", err)
	}

	return func(ctx context.Context, bl *sources.RPCBlock) error {
		entry := &L1Entry{
			Header: bl.RPCHeader,
		}
		signer := cfg.L1Signer()

		blobIndex := 0 // index of each blob in the block's blob sidecar
		for i, tx := range bl.Transactions {
			if tx.To() != nil && *tx.To() == cfg.BatchInboxAddress {
				sender, err := signer.Sender(tx)
				if err != nil {
					return fmt.Errorf("invalid signer: %w", err)
				}
				// We assume the batcher key never changed.
				// Not technically correct, but simpler now.
				if sender != cfg.Genesis.SystemConfig.BatcherAddr {
					continue
				}
				var datas []hexutil.Bytes
				if tx.Type() != types.BlobTxType {
					datas = append(datas, tx.Data())
					// no need to increment blobIndex because no blobs
				} else {
					if beacon == nil {
						log.Error("Unable to handle blob transaction because L1 Beacon API not provided", "tx", tx.Hash())
						blobIndex += len(tx.BlobHashes())
						continue
					}
					var hashes []eth.IndexedBlobHash
					for _, h := range tx.BlobHashes() {
						idh := eth.IndexedBlobHash{
							Index: uint64(blobIndex),
							Hash:  h,
						}
						hashes = append(hashes, idh)
						blobIndex += 1
					}

					l1Ref := eth.L1BlockRef{
						Hash:       bl.Hash,
						Number:     uint64(bl.Number),
						ParentHash: bl.ParentHash,
						Time:       uint64(bl.Time),
					}
					blobs, err := retry.Do[[]*eth.Blob](ctx, 10, retry.Exponential(), func() ([]*eth.Blob, error) {
						return beacon.GetBlobs(ctx, l1Ref, hashes)
					})
					if err != nil {
						return fmt.Errorf("failed to fetch blobs: %w", err)
					}
					for _, blob := range blobs {
						data, err := blob.ToData()
						if err != nil {
							return fmt.Errorf("failed to parse blobs: %w", err)
						}
						datas = append(datas, data)
					}
				}
				var frameErrors []string
				var frames []derive.Frame
				var validFrames []bool
				for _, data := range datas {
					validFrame := true
					frameError := ""
					framesPerData, err := derive.ParseFrames(data)
					if err != nil {
						logger.Error("Found a transaction with invalid data", "txHash", tx.Hash(), "err", err)
						validFrame = false
						frameError = err.Error()
					} else {
						frames = append(frames, framesPerData...)
					}
					frameErrors = append(frameErrors, frameError)
					validFrames = append(validFrames, validFrame)
				}
				txm := &TransactionWithMetadata{
					Tx:          tx,
					Sender:      sender,
					TxIndex:     uint64(i),
					BlockNumber: uint64(bl.Number),
					BlockHash:   bl.Hash,
					BlockTime:   uint64(bl.Time),
					Frames:      frames,
					FrameErrs:   frameErrors,
					ValidFrames: validFrames,
				}
				entry.BatchTxs = append(entry.BatchTxs, tx.Hash())

				filename := filepath.Join(txsDir, fmt.Sprintf("%s.json", tx.Hash()))
				if err := writeJSON(filename, txm); err != nil {
					return fmt.Errorf("failed to write tx json %q: %w", filename, err)
				}
			} else {
				blobIndex += len(tx.BlobHashes())
			}
		}

		filename := filepath.Join(blocksDir, fmt.Sprintf("%08d_%s.json", uint64(bl.Number), bl.Hash))
		if err := writeJSON(filename, bl); err != nil {
			return fmt.Errorf("failed to write block json %q: %w", filename, err)
		}

		logger.Info("Processed L1 block", "block", entry.Header.BlockID())
		return nil
	}, nil
}

func writeJSON(filePath string, data any) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	return enc.Encode(data)
}
