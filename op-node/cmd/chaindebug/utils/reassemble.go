package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type FrameWithMetadata struct {
	TxHash         common.Hash  `json:"transaction_hash"`
	InclusionBlock uint64       `json:"inclusion_block"`
	Timestamp      uint64       `json:"timestamp"`
	BlockHash      common.Hash  `json:"block_hash"`
	Frame          derive.Frame `json:"frame"`
}

func listTransactions(ctx context.Context, txsDir string, inbox common.Address, sender common.Address) ([]TransactionWithMetadata, error) {

	var txs []TransactionWithMetadata
	entries, err := os.ReadDir(txsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read existing blocks dir: %w", err)
	}

	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p := filepath.Join(txsDir, entry.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to read %q: %w", p, err)
		}
		var x TransactionWithMetadata
		if err := json.Unmarshal(data, &x); err != nil {
			return nil, fmt.Errorf("failed to read block %s: %w", p, err)
		}
		if x.Sender != sender {
			continue
		}
		if to := x.Tx.To(); to == nil || *to != inbox {
			continue
		}
		txs = append(txs, x)
	}
	sort.Slice(txs, func(i, j int) bool {
		if txs[i].BlockNumber == txs[j].BlockNumber {
			return txs[i].TxIndex < txs[j].TxIndex
		} else {
			return txs[i].BlockNumber < txs[j].BlockNumber
		}
	})
	return txs, nil
}

func transactionsToFrames(txns []TransactionWithMetadata) []FrameWithMetadata {
	var out []FrameWithMetadata
	for _, tx := range txns {
		for _, frame := range tx.Frames {
			fm := FrameWithMetadata{
				TxHash:         tx.Tx.Hash(),
				InclusionBlock: tx.BlockNumber,
				BlockHash:      tx.BlockHash,
				Timestamp:      tx.BlockTime,
				Frame:          frame,
			}
			out = append(out, fm)
		}
	}
	return out
}

type BlockSeal struct {
	Hash   common.Hash `json:"hash"`
	Number uint64      `json:"number"`
	Time   uint64      `json:"timestamp"`
}

type ChannelWithMetadata struct {
	ID             derive.ChannelID         `json:"id"`
	IsReady        bool                     `json:"is_ready"`
	InvalidFrames  bool                     `json:"invalid_frames"`
	InvalidBatches bool                     `json:"invalid_batches"`
	Frames         []FrameWithMetadata      `json:"frames"`
	Batches        []derive.Batch           `json:"batches"`
	BatchTypes     []int                    `json:"batch_types"`
	ComprAlgos     []derive.CompressionAlgo `json:"compr_algos"`
	CompletedIn    BlockSeal                `json:"completed_in"` // when ready to be derived from
}

type ImpliedBlock struct {
	ParentHash common.Hash      `json:"parent_hash"` // parent L2 block hash
	EpochNum   rollup.Epoch     `json:"epoch_num"`   // aka l1 num
	EpochHash  common.Hash      `json:"epoch_hash"`  // l1 block hash
	Timestamp  uint64           `json:"timestamp"`   // l2 block timestamp
	SpanL2     uint64           `json:"span_l2"`     // number of L2 blocks contained
	LastEpoch  uint64           `json:"last_epoch"`  // last L1 origin that was mentioned
	IncludedIn BlockSeal        `json:"included_in"` // L1 block that batch was included in
	Channel    derive.ChannelID `json:"channel"`
}

type ReassembleConfig struct {
	TxsDir           string // txs dir
	ChannelsDir      string // channels dir
	ImpliedBlocksDir string
}

// Channels loads all transactions from the given input directory that are submitted to the
// specified batch inbox and then re-assembles all channels & writes the re-assembled channels
// to the out directory.
func Channels(ctx context.Context, config *ReassembleConfig, logger log.Logger, rollupCfg *rollup.Config) error {
	if err := os.MkdirAll(config.ChannelsDir, 0750); err != nil {
		return err
	}
	if err := os.MkdirAll(config.ImpliedBlocksDir, 0750); err != nil {
		return err
	}
	frames, err := LoadFrames(ctx, config.TxsDir, logger,
		rollupCfg.BatchInboxAddress, rollupCfg.Genesis.SystemConfig.BatcherAddr)
	if err != nil {
		return err
	}
	framesByChannel := make(map[derive.ChannelID][]FrameWithMetadata)
	for _, frame := range frames {
		framesByChannel[frame.Frame.ID] = append(framesByChannel[frame.Frame.ID], frame)
	}
	for id, frames := range framesByChannel {
		logger.Info("Processing frames of channel", "id", id)
		ch := ProcessFrames(rollupCfg, id, frames)
		filename := path.Join(config.ChannelsDir, fmt.Sprintf("%s.json", id.String()))
		if err := writeChannel(ch, filename); err != nil {
			return fmt.Errorf("failed to write channel: %w", err)
		}
		for _, b := range ch.Batches {
			block := ImpliedBlock{
				ParentHash: common.Hash{},
				EpochNum:   0,
				EpochHash:  common.Hash{},
				Timestamp:  0,
				SpanL2:     0,
				IncludedIn: ch.CompletedIn,
				Channel:    ch.ID,
			}
			if sb, ok := b.AsSingularBatch(); ok {
				block.ParentHash = sb.ParentHash
				block.EpochNum = sb.EpochNum
				block.EpochHash = sb.EpochHash
				block.Timestamp = sb.Timestamp
				block.SpanL2 = 1
				block.LastEpoch = uint64(sb.EpochNum)
			}
			if sb, ok := b.AsSpanBatch(); ok {
				copy(block.ParentHash[:], sb.ParentCheck[:])
				copy(block.EpochHash[:], sb.L1OriginCheck[:])
				block.Timestamp = sb.GetTimestamp()
				block.EpochNum = sb.GetStartEpochNum()
				block.SpanL2 = uint64(sb.GetBlockCount())
				block.LastEpoch = sb.GetBlockEpochNum(sb.GetBlockCount() - 1)
			}
			p := filepath.Join(config.ImpliedBlocksDir,
				fmt.Sprintf("%08d_%s.json", block.Timestamp, block.Channel))
			if err := writeJSON(p, block); err != nil {
				return fmt.Errorf("failed to write implied block %q: %w", p, err)
			}
		}
	}
	return nil
}

func LoadFrames(ctx context.Context, txsDir string, logger log.Logger, inbox common.Address, sender common.Address) ([]FrameWithMetadata, error) {
	txs, err := listTransactions(ctx, txsDir, inbox, sender)
	if err != nil {
		return nil, fmt.Errorf("list txs err: %w", err)
	}
	frames := transactionsToFrames(txs)
	return frames, nil
}

func writeChannel(ch ChannelWithMetadata, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	return enc.Encode(ch)
}

// ProcessFrames processes the frames for a given channel and reads batches and other relevant metadata
// from the channel. Returns a ChannelWithMetadata struct containing all the relevant data.
func ProcessFrames(rollupCfg *rollup.Config, id derive.ChannelID, frames []FrameWithMetadata) ChannelWithMetadata {
	spec := rollup.NewChainSpec(rollupCfg)
	ch := derive.NewChannel(id, eth.L1BlockRef{Number: frames[0].InclusionBlock}, rollupCfg.IsHolocene(frames[0].Timestamp))
	invalidFrame := false

	completedIn := BlockSeal{}

	for _, frame := range frames {
		if ch.IsReady() {
			fmt.Printf("Channel %v is ready despite having more frames\n", id.String())
			invalidFrame = true
			break
		}
		if err := ch.AddFrame(frame.Frame, eth.L1BlockRef{Number: frame.InclusionBlock, Time: frame.Timestamp}); err != nil {
			fmt.Printf("Error adding to channel %v. Err: %v\n", id.String(), err)
			invalidFrame = true
		}
		if frame.InclusionBlock > completedIn.Number {
			completedIn = BlockSeal{
				Hash:   frame.BlockHash,
				Number: frame.InclusionBlock,
				Time:   frame.Timestamp,
			}
		}
	}

	var (
		batches    []derive.Batch
		batchTypes []int
		comprAlgos []derive.CompressionAlgo
	)

	invalidBatches := false
	if ch.IsReady() {
		br, err := derive.BatchReader(ch.Reader(), spec.MaxRLPBytesPerChannel(ch.HighestBlock().Time), rollupCfg.IsFjord(ch.HighestBlock().Time))
		if err == nil {
			for batchData, err := br(); err != io.EOF; batchData, err = br() {
				if err != nil {
					fmt.Printf("Error reading batchData for channel %v. Err: %v\n", id.String(), err)
					invalidBatches = true
				} else {
					comprAlgos = append(comprAlgos, batchData.ComprAlgo)
					batchType := batchData.GetBatchType()
					batchTypes = append(batchTypes, int(batchType))
					switch batchType {
					case derive.SingularBatchType:
						singularBatch, err := derive.GetSingularBatch(batchData)
						if err != nil {
							invalidBatches = true
							fmt.Printf("Error converting singularBatch from batchData for channel %v. Err: %v\n", id.String(), err)
						}
						// singularBatch will be nil when errored
						batches = append(batches, singularBatch)
					case derive.SpanBatchType:
						spanBatch, err := derive.DeriveSpanBatch(batchData, rollupCfg.BlockTime, rollupCfg.Genesis.L2Time, rollupCfg.L2ChainID)
						if err != nil {
							invalidBatches = true
							fmt.Printf("Error deriving spanBatch from batchData for channel %v. Err: %v\n", id.String(), err)
						}
						// spanBatch will be nil when errored
						batches = append(batches, spanBatch)
					default:
						fmt.Printf("unrecognized batch type: %d for channel %v.\n", batchData.GetBatchType(), id.String())
					}
				}
			}
		} else {
			fmt.Printf("Error creating batch reader for channel %v. Err: %v\n", id.String(), err)
		}
	} else {
		fmt.Printf("Channel %v is not ready\n", id.String())
	}

	return ChannelWithMetadata{
		ID:             id,
		Frames:         frames,
		IsReady:        ch.IsReady(),
		InvalidFrames:  invalidFrame,
		InvalidBatches: invalidBatches,
		Batches:        batches,
		BatchTypes:     batchTypes,
		ComprAlgos:     comprAlgos,
		CompletedIn:    completedIn,
	}
}
