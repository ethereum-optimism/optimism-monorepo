package processors

import (
	"fmt"

	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/interoptypes"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type EventDecoderFn func(*ethTypes.Log) (*types.ExecutingMessage, error)

func DecodeExecutingMessageLog(l *ethTypes.Log) (*types.ExecutingMessage, error) {
	if l.Address != params.InteropCrossL2InboxAddress {
		return nil, nil
	}
	if len(l.Topics) != 2 { // topics: event-id and payload-hash
		return nil, nil
	}
	if l.Topics[0] != interoptypes.ExecutingMessageEventTopic {
		return nil, nil
	}
	var msg interoptypes.Message
	if err := msg.DecodeEvent(l.Topics, l.Data); err != nil {
		return nil, fmt.Errorf("invalid executing message: %w", err)
	}
	logHash := types.PayloadHashToLogHash(msg.PayloadHash, msg.Identifier.Origin)
	return &types.ExecutingMessage{
		// TODO(#11105): translate chain index to chain ID
		Chain:     types.ChainIndex(msg.Identifier.ChainID.Uint64()),
		BlockNum:  msg.Identifier.BlockNumber,
		LogIdx:    msg.Identifier.LogIndex,
		Timestamp: msg.Identifier.Timestamp,
		Hash:      logHash,
	}, nil
}
