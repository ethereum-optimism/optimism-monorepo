package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/sync/errgroup"
)

type TransactionWithMetadata struct {
	TxIndex     uint64             `json:"tx_index"`
	InboxAddr   common.Address     `json:"inbox_address"`
	BlockNumber uint64             `json:"block_number"`
	BlockHash   common.Hash        `json:"block_hash"`
	BlockTime   uint64             `json:"block_time"`
	ChainId     uint64             `json:"chain_id"`
	Sender      common.Address     `json:"sender"`
	ValidSender bool               `json:"valid_sender"`
	Frames      []derive.Frame     `json:"frames"`
	FrameErrs   []string           `json:"frame_parse_error"`
	ValidFrames []bool             `json:"valid_data"`
	Tx          *types.Transaction `json:"tx"`
}

type Config struct {
	Start, End         uint64
	ChainID            *big.Int
	BatchInbox         common.Address
	BatchSenders       map[common.Address]struct{}
	OutDirectory       string
	ConcurrentRequests uint64
}

// Batches fetches & stores all transactions sent to the batch inbox address in
// the given block range (inclusive to exclusive).
// The transactions & metadata are written to the out directory.
func Batches(client *ethclient.Client, beacon *sources.L1BeaconClient, config Config) (totalValid, totalInvalid uint64) {
	if err := os.MkdirAll(config.OutDirectory, 0750); err != nil {
		log.Fatal(err)
	}
	signer := types.LatestSignerForChainID(config.ChainID)
	concurrentRequests := int(config.ConcurrentRequests)

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(concurrentRequests)

	for i := config.Start; i < config.End; i++ {
		if err := ctx.Err(); err != nil {
			break
		}
		number := i
		g.Go(func() error {
			valid, invalid, err := fetchBatchesPerBlock(ctx, client, beacon, number, signer, config)
			if err != nil {
				return fmt.Errorf("error occurred while fetching block %d: %w", number, err)
			}
			atomic.AddUint64(&totalValid, valid)
			atomic.AddUint64(&totalInvalid, invalid)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
	return
}

// fetchBatchesPerBlock gets a block & the parses all of the transactions in the block.
func fetchBatchesPerBlock(ctx context.Context, client *ethclient.Client, beacon *sources.L1BeaconClient, number uint64, signer types.Signer, config Config) (uint64, uint64, error) {
	validBatchCount := uint64(0)
	invalidBatchCount := uint64(0)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	block, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(number))
	if err != nil {
		return 0, 0, err
	}
	fmt.Println("Fetched block: ", number)
	blobIndex := 0 // index of each blob in the block's blob sidecar
	for i, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == config.BatchInbox {
			sender, err := signer.Sender(tx)
			if err != nil {
				return 0, 0, err
			}
			validSender := true
			if _, ok := config.BatchSenders[sender]; !ok {
				fmt.Printf("Found a transaction (%s) from an invalid sender (%s)\n", tx.Hash().String(), sender.String())
				invalidBatchCount += 1
				validSender = false
			}
			var data []hexutil.Bytes
			if tx.Type() != types.BlobTxType {
				data = append(data, tx.Data())
				// no need to increment blobIndex because no blobs
			} else {
				if beacon == nil {
					fmt.Printf("Unable to handle blob transaction (%s) because L1 Beacon API not provided\n", tx.Hash().String())
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
				blobs, err := beacon.GetBlobs(ctx, eth.L1BlockRef{
					Hash:       block.Hash(),
					Number:     block.Number().Uint64(),
					ParentHash: block.ParentHash(),
					Time:       block.Time(),
				}, hashes)
				if err != nil {
					log.Fatal(fmt.Errorf("failed to fetch blobs: %w", err))
				}
				for _, blob := range blobs {
					data, err := blob.ToData()
					if err != nil {
						log.Fatal(fmt.Errorf("failed to parse blobs: %w", err))
					}
					data = append(data, data)
				}
			}
			var frameErrors []string
			var frames []derive.Frame
			var validFrames []bool
			validBatch := true
			for _, data := range data {
				validFrame := true
				frameError := ""
				framesPerData, err := derive.ParseFrames(data)
				if err != nil {
					fmt.Printf("Found a transaction (%s) with invalid data: %v\n", tx.Hash().String(), err)
					validFrame = false
					validBatch = false
					frameError = err.Error()
				} else {
					frames = append(frames, framesPerData...)
				}
				frameErrors = append(frameErrors, frameError)
				validFrames = append(validFrames, validFrame)
			}
			if validSender && validBatch {
				validBatchCount += 1
			} else {
				invalidBatchCount += 1
			}
			txm := &TransactionWithMetadata{
				Tx:          tx,
				Sender:      sender,
				ValidSender: validSender,
				TxIndex:     uint64(i),
				BlockNumber: block.NumberU64(),
				BlockHash:   block.Hash(),
				BlockTime:   block.Time(),
				ChainId:     config.ChainID.Uint64(),
				InboxAddr:   config.BatchInbox,
				Frames:      frames,
				FrameErrs:   frameErrors,
				ValidFrames: validFrames,
			}
			filename := path.Join(config.OutDirectory, fmt.Sprintf("%s.json", tx.Hash().String()))
			file, err := os.Create(filename)
			if err != nil {
				return 0, 0, err
			}
			enc := json.NewEncoder(file)
			if err := enc.Encode(txm); err != nil {
				file.Close()
				return 0, 0, err
			}
			file.Close()
		} else {
			blobIndex += len(tx.BlobHashes())
		}
	}
	return validBatchCount, invalidBatchCount, nil
}
