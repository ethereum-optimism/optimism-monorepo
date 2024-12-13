package processors

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type LogStorage interface {
	SealBlock(chain types.ChainID, block eth.BlockRef) error
	AddLog(chain types.ChainID, logHash common.Hash, parentBlock eth.BlockID, logIdx uint32, execMsg *types.ExecutingMessage) error
}

type ChainsDBClientForLogProcessor interface {
	SealBlock(chain types.ChainID, block eth.BlockRef) error
	AddLog(chain types.ChainID, logHash common.Hash, parentBlock eth.BlockID, logIdx uint32, execMsg *types.ExecutingMessage) error
}

type logProcessor struct {
	chain        types.ChainID
	logStore     LogStorage
	eventDecoder EventDecoderFn
}

func NewLogProcessor(chain types.ChainID, logStore LogStorage) LogProcessor {
	return &logProcessor{
		chain:        chain,
		logStore:     logStore,
		eventDecoder: DecodeExecutingMessageLog,
	}
}

// ProcessLogs processes logs from a block and stores them in the log storage
// for any logs that are related to executing messages, they are decoded and stored
func (p *logProcessor) ProcessLogs(_ context.Context, block eth.BlockRef, rcpts ethTypes.Receipts) error {
	for _, rcpt := range rcpts {
		for _, l := range rcpt.Logs {
			// log hash represents the hash of *this* log as a potentially initiating message
			logHash := logToLogHash(l)
			// The log may be an executing message emitted by the CrossL2Inbox
			execMsg, err := p.eventDecoder(l)
			if err != nil {
				return fmt.Errorf("invalid log %d from block %s: %w", l.Index, block.ID(), err)
			}
			// executing messages have multiple entries in the database
			// they should start with the initiating message and then include the execution
			if err := p.logStore.AddLog(p.chain, logHash, block.ParentID(), uint32(l.Index), execMsg); err != nil {
				return fmt.Errorf("failed to add log %d from block %s: %w", l.Index, block.ID(), err)
			}
		}
	}
	if err := p.logStore.SealBlock(p.chain, block); err != nil {
		return fmt.Errorf("failed to seal block %s: %w", block.ID(), err)
	}
	return nil
}

// logToLogHash transforms a log into a hash that represents the log.
// it is the concatenation of the log's address and the hash of the log's payload,
// which is then hashed again. This is the hash that is stored in the log storage.
// The address is hashed into the payload hash to save space in the log storage,
// and because they represent paired data.
func logToLogHash(l *ethTypes.Log) common.Hash {
	payloadHash := crypto.Keccak256Hash(types.LogToMessagePayload(l))
	return types.PayloadHashToLogHash(payloadHash, l.Address)
}
