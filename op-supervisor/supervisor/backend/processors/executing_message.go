package processors

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/interoptypes"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type EventDecoderFn func(*ethTypes.Log, depset.ChainIndexFromID) (*types.ExecutingMessage, error)

func DecodeExecutingMessageLog(l *ethTypes.Log, depSet depset.ChainIndexFromID) (*types.ExecutingMessage, error) {
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
	index, err := depSet.ChainIndexFromID(eth.ChainID(msg.Identifier.ChainID))
	if err != nil {
		return nil, err
	}
	return &types.ExecutingMessage{
		Chain:     index,
		BlockNum:  msg.Identifier.BlockNumber,
		LogIdx:    msg.Identifier.LogIndex,
		Timestamp: msg.Identifier.Timestamp,
		Hash:      logHash,
	}, nil
}
