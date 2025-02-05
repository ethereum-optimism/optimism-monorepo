package p2p

import (
	"math"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/suite"
)

// PeerParamsTestSuite tests peer parameterization.
type PeerParamsTestSuite struct {
	suite.Suite
}

// TestPeerParams runs the PeerParamsTestSuite.
func TestPeerParams(t *testing.T) {
	suite.Run(t, new(PeerParamsTestSuite))
}

// TestPeerScoreConstants validates the peer score constants.
func (testSuite *PeerParamsTestSuite) TestPeerScoreConstants() {
	testSuite.Equal(0.01, DecayToZero)
}

// TestNewPeerScoreThresholds validates the peer score thresholds.
//
// This is tested to ensure that the thresholds are not modified and missed in review.
func (testSuite *PeerParamsTestSuite) TestNewPeerScoreThresholds() {
	thresholds := NewPeerScoreThresholds()
	expected := pubsub.PeerScoreThresholds{
		SkipAtomicValidation:        false,
		GossipThreshold:             -10,
		PublishThreshold:            -40,
		GraylistThreshold:           -40,
		AcceptPXThreshold:           20,
		OpportunisticGraftThreshold: 0.05,
	}
	testSuite.Equal(expected, thresholds)
}

// TestGetPeerScoreParams validates the peer score parameters.
func (testSuite *PeerParamsTestSuite) TestGetPeerScoreParams_None() {
	params, err := GetScoringParams("none", chaincfg.OPSepolia())
	testSuite.NoError(err)
	testSuite.Nil(params)
}

// TestLightPeerScoreParams validates the light peer score params.
func (testSuite *PeerParamsTestSuite) TestGetPeerScoreParams_Light() {
	cfg := chaincfg.OPSepolia()
	cfg.BlockTime = 1
	slot := time.Duration(cfg.BlockTime) * time.Second
	epoch := 6 * slot
	oneHundredEpochs := 100 * epoch

	// calculate the behavior penalty decay
	duration := 10 * epoch
	decay := math.Pow(DecayToZero, 1/float64(duration/slot))
	testSuite.Equal(0.9261187281287935, decay)

	// Test the params
	scoringParams, err := GetScoringParams("light", cfg)
	peerParams := scoringParams.PeerScoring
	testSuite.NoError(err)
	// Topics should contain options for block topic
	testSuite.Len(peerParams.Topics, 1)
	topicParams, ok := peerParams.Topics[blocksTopicV1(cfg)]
	testSuite.True(ok, "should have block topic params")
	testSuite.NotZero(topicParams.TimeInMeshQuantum)
	testSuite.Equal(peerParams.TopicScoreCap, float64(34))
	testSuite.Equal(peerParams.AppSpecificWeight, float64(1))
	testSuite.Equal(peerParams.IPColocationFactorWeight, float64(-35))
	testSuite.Equal(peerParams.IPColocationFactorThreshold, 10)
	testSuite.Nil(peerParams.IPColocationFactorWhitelist)
	testSuite.Equal(peerParams.BehaviourPenaltyWeight, float64(-16))
	testSuite.Equal(peerParams.BehaviourPenaltyThreshold, float64(6))
	testSuite.Equal(peerParams.BehaviourPenaltyDecay, decay)
	testSuite.Equal(peerParams.DecayInterval, slot)
	testSuite.Equal(peerParams.DecayToZero, DecayToZero)
	testSuite.Equal(peerParams.RetainScore, oneHundredEpochs)

	appParams := scoringParams.ApplicationScoring
	testSuite.Positive(appParams.ValidResponseCap)
	testSuite.Positive(appParams.ValidResponseWeight)
	testSuite.Positive(appParams.ValidResponseDecay)
	testSuite.Positive(appParams.ErrorResponseCap)
	testSuite.Negative(appParams.ErrorResponseWeight)
	testSuite.Positive(appParams.ErrorResponseDecay)
	testSuite.Positive(appParams.RejectedPayloadCap)
	testSuite.Negative(appParams.RejectedPayloadWeight)
	testSuite.Positive(appParams.RejectedPayloadDecay)
	testSuite.Equal(DecayToZero, appParams.DecayToZero)
	testSuite.Equal(slot, appParams.DecayInterval)
}

// TestParamsZeroBlockTime validates peer score params use default slot for 0 block time.
func (testSuite *PeerParamsTestSuite) TestParamsZeroBlockTime() {
	cfg := chaincfg.OPSepolia()
	cfg.BlockTime = 0
	slot := 2 * time.Second
	params, err := GetScoringParams("light", cfg)
	testSuite.NoError(err)
	testSuite.Equal(params.PeerScoring.DecayInterval, slot)
	testSuite.Equal(params.ApplicationScoring.DecayInterval, slot)
}
