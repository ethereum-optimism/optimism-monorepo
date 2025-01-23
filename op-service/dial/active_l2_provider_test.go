package dial

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

// endpointProviderTest is a test harness for setting up endpoint provider tests.
type endpointProviderTest struct {
	t                  *testing.T
	rollupClients      []*testutils.MockRollupClient
	ethClients         []*testutils.MockEthClient
	rollupDialOutcomes map[int]bool // true for success, false for failure
	ethDialOutcomes    map[int]bool // true for success, false for failure
}

// setupEndpointProviderTest sets up the basic structure of the endpoint provider tests.
func setupEndpointProviderTest(t *testing.T, numSequencers int) *endpointProviderTest {
	ept := &endpointProviderTest{
		t:                  t,
		rollupClients:      make([]*testutils.MockRollupClient, numSequencers),
		ethClients:         make([]*testutils.MockEthClient, numSequencers),
		rollupDialOutcomes: make(map[int]bool),
		ethDialOutcomes:    make(map[int]bool),
	}

	for i := 0; i < numSequencers; i++ {
		ept.rollupClients[i] = new(testutils.MockRollupClient)
		ept.ethClients[i] = new(testutils.MockEthClient)
		ept.rollupDialOutcomes[i] = true // by default, all dials succeed
		ept.ethDialOutcomes[i] = true    // by default, all dials succeed
	}

	return ept
}

// newActiveL2RollupProvider constructs a new ActiveL2RollupProvider using the test harness setup.
func (et *endpointProviderTest) newActiveL2RollupProvider(checkDuration time.Duration) (*ActiveL2RollupProvider, error) {
	mockRollupDialer := func(ctx context.Context, log log.Logger, url string) (RollupClientInterface, error) {
		for i, client := range et.rollupClients {
			if url == fmt.Sprintf("rollup%d", i) {
				if !et.rollupDialOutcomes[i] {
					return nil, fmt.Errorf("simulated dial failure for rollup %d", i)
				}
				return client, nil
			}
		}
		return nil, fmt.Errorf("unknown test url: %s", url)
	}

	// make the "URLs"
	rollupUrls := make([]string, len(et.rollupClients))
	for i := range et.rollupClients {
		rollupUrl := fmt.Sprintf("rollup%d", i)
		rollupUrls[i] = rollupUrl
	}

	return newActiveL2RollupProvider(
		context.Background(),
		rollupUrls,
		checkDuration,
		1*time.Minute,
		testlog.Logger(et.t, log.LevelDebug),
		mockRollupDialer,
	)
}

// newActiveL2EndpointProvider constructs a new ActiveL2EndpointProvider using the test harness setup.
func (et *endpointProviderTest) newActiveL2EndpointProvider(checkDuration time.Duration) (*ActiveL2EndpointProvider, error) {
	mockRollupDialer := func(ctx context.Context, log log.Logger, url string) (RollupClientInterface, error) {
		for i, client := range et.rollupClients {
			if url == fmt.Sprintf("rollup%d", i) {
				if !et.rollupDialOutcomes[i] {
					return nil, fmt.Errorf("simulated dial failure for rollup %d", i)
				}
				return client, nil
			}
		}
		return nil, fmt.Errorf("unknown test url: %s", url)
	}

	mockEthDialer := func(ctx context.Context, log log.Logger, url string) (EthClientInterface, error) {
		for i, client := range et.ethClients {
			if url == fmt.Sprintf("eth%d", i) {
				if !et.ethDialOutcomes[i] {
					return nil, fmt.Errorf("simulated dial failure for eth %d", i)
				}
				return client, nil
			}
		}
		return nil, fmt.Errorf("unknown test url: %s", url)
	}

	// make the "URLs"
	rollupUrls := make([]string, len(et.rollupClients))
	for i := range et.rollupClients {
		rollupUrl := fmt.Sprintf("rollup%d", i)
		rollupUrls[i] = rollupUrl
	}
	ethUrls := make([]string, len(et.ethClients))
	for i := range et.ethClients {
		ethUrl := fmt.Sprintf("eth%d", i)
		ethUrls[i] = ethUrl
	}

	return newActiveL2EndpointProvider(
		context.Background(),
		ethUrls,
		rollupUrls,
		checkDuration,
		1*time.Minute,
		testlog.Logger(et.t, log.LevelDebug),
		mockEthDialer,
		mockRollupDialer,
	)
}

func (et *endpointProviderTest) assertAllExpectations(t *testing.T) {
	for _, sequencer := range et.rollupClients {
		sequencer.AssertExpectations(t)
	}
	for _, ethClient := range et.ethClients {
		ethClient.AssertExpectations(t)
	}
}

func (et *endpointProviderTest) setRollupDialOutcome(index int, success bool) {
	et.rollupDialOutcomes[index] = success
}

// TestRollupProvider_FailoverOnInactiveSequencer verifies that the ActiveL2RollupProvider
// will switch to the next provider if the current one becomes inactive.
func TestRollupProvider_FailoverOnInactiveSequencer(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer, secondarySequencer := ept.rollupClients[0], ept.rollupClients[1]

	primarySequencer.ExpectSequencerActive(true, nil) // respond true once on creation
	primarySequencer.ExpectSequencerActive(true, nil) // respond true again when the test calls `RollupClient()` the first time

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	firstSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primarySequencer, firstSequencerUsed)

	primarySequencer.ExpectSequencerActive(false, nil) // become inactive after that
	primarySequencer.MaybeClose()
	secondarySequencer.ExpectSequencerActive(true, nil)
	secondSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, secondarySequencer, secondSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_FailoverOnInactiveSequencer verifies that the ActiveL2EndpointProvider
// will switch to the next provider if the current one becomes inactive.
func TestEndpointProvider_FailoverOnInactiveSequencer(t *testing.T) {
	// as TestActiveSequencerFailoverBehavior_RollupProviders,
	// but ensure the added `EthClient()` method also triggers the failover.
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer, secondarySequencer := ept.rollupClients[0], ept.rollupClients[1]
	primarySequencer.ExpectSequencerActive(true, nil) // primary sequencer gets hit once on creation: embedded call of `RollupClient()`
	primarySequencer.ExpectSequencerActive(true, nil) // primary sequencer gets hit twice on creation: implicit call of `EthClient()`
	primarySequencer.ExpectSequencerActive(true, nil) // respond true again when the test calls `EthClient()` the first time

	activeProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	firstSequencerUsed, err := activeProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], firstSequencerUsed)

	primarySequencer.ExpectSequencerActive(false, nil) // become inactive after that
	secondarySequencer.ExpectSequencerActive(true, nil)
	primarySequencer.MaybeClose()
	ept.ethClients[0].MaybeClose() // we close the ethclient when we switch over to the next sequencer
	secondSequencerUsed, err := activeProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[1], secondSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_FailoverOnErroredSequencer verifies that the ActiveL2RollupProvider
// will switch to the next provider if the current one returns an error.
func TestRollupProvider_FailoverOnErroredSequencer(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer, secondarySequencer := ept.rollupClients[0], ept.rollupClients[1]

	primarySequencer.ExpectSequencerActive(true, nil) // respond true once on creation
	primarySequencer.ExpectSequencerActive(true, nil) // respond true again when the test calls `RollupClient()` the first time

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	firstSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primarySequencer, firstSequencerUsed)

	primarySequencer.ExpectSequencerActive(true, errors.New("a test error")) // error-out after that
	primarySequencer.MaybeClose()
	secondarySequencer.ExpectSequencerActive(true, nil)
	secondSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, secondarySequencer, secondSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_FailoverOnErroredSequencer verifies that the ActiveL2EndpointProvider
// will switch to the next provider if the current one returns an error.
func TestEndpointProvider_FailoverOnErroredSequencer(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer, secondarySequencer := ept.rollupClients[0], ept.rollupClients[1]
	primaryEthClient, secondaryEthClient := ept.ethClients[0], ept.ethClients[1]

	primarySequencer.ExpectSequencerActive(true, nil) // primary sequencer gets hit once on creation: embedded call of `RollupClient()`
	primarySequencer.ExpectSequencerActive(true, nil) // primary sequencer gets hit twice on creation: implicit call of `EthClient()`

	activeProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	primarySequencer.ExpectSequencerActive(true, nil) // respond true again when the test calls `EthClient()` the first time
	firstSequencerUsed, err := activeProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primaryEthClient, firstSequencerUsed)

	primarySequencer.ExpectSequencerActive(true, errors.New("a test error")) // error out after that
	primarySequencer.MaybeClose()
	primaryEthClient.MaybeClose()
	secondarySequencer.ExpectSequencerActive(true, nil)

	secondSequencerUsed, err := activeProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, secondaryEthClient, secondSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_NoExtraCheckOnActiveSequencer verifies that the ActiveL2RollupProvider
// does not change if the current sequencer is active.
func TestRollupProvider_NoExtraCheckOnActiveSequencer(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer := ept.rollupClients[0]

	primarySequencer.ExpectSequencerActive(true, nil) // default test provider, which always checks, checks Active on creation

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)
	require.Same(t, primarySequencer, rollupProvider.currentRollupClient)

	primarySequencer.ExpectSequencerActive(true, nil) // default test provider, which always checks, checks again on RollupClient()

	firstSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primarySequencer, firstSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_NoExtraCheckOnActiveSequencer verifies that the ActiveL2EndpointProvider
// does not change if the current sequencer is active.
func TestEndpointProvider_NoExtraCheckOnActiveSequencer(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer := ept.rollupClients[0]

	primarySequencer.ExpectSequencerActive(true, nil) // default test provider, which always checks, checks Active twice on creation (once for internal RollupClient() call)
	primarySequencer.ExpectSequencerActive(true, nil) // default test provider, which always checks, checks Active twice on creation (once for internal EthClient() call)

	endpointProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], endpointProvider.currentEthClient)

	primarySequencer.ExpectSequencerActive(true, nil) // default test provider, which always checks, checks again on EthClient()

	firstEthClientUsed, err := endpointProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], firstEthClientUsed)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_FailoverAndReturn verifies the ActiveL2RollupProvider's ability to
// failover and then return to the primary sequencer once it becomes active again.
func TestRollupProvider_FailoverAndReturn(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer, secondarySequencer := ept.rollupClients[0], ept.rollupClients[1]

	// Primary initially active
	primarySequencer.ExpectSequencerActive(true, nil)
	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	// Primary becomes inactive, secondary active
	primarySequencer.ExpectSequencerActive(false, nil)
	primarySequencer.MaybeClose()
	secondarySequencer.ExpectSequencerActive(true, nil)

	// Fails over to secondary
	secondSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, secondarySequencer, secondSequencerUsed)

	// Primary becomes active again, secondary becomes inactive
	primarySequencer.ExpectSequencerActive(true, nil)
	secondarySequencer.ExpectSequencerActive(false, nil)
	secondarySequencer.MaybeClose()

	// Should return to primary
	thirdSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primarySequencer, thirdSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_FailoverAndReturn verifies the ActiveL2EndpointProvider's ability to
// failover and then return to the primary sequencer once it becomes active again.
func TestEndpointProvider_FailoverAndReturn(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer, secondarySequencer := ept.rollupClients[0], ept.rollupClients[1]

	// Primary initially active
	primarySequencer.ExpectSequencerActive(true, nil)
	primarySequencer.ExpectSequencerActive(true, nil) // see comment in other tests about why we expect this twice
	endpointProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	// Primary becomes inactive, secondary active
	primarySequencer.ExpectSequencerActive(false, nil)
	primarySequencer.MaybeClose()
	ept.ethClients[0].MaybeClose()
	secondarySequencer.ExpectSequencerActive(true, nil)

	// Fails over to secondary
	secondEthClient, err := endpointProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[1], secondEthClient)

	// Primary becomes active again, secondary becomes inactive
	primarySequencer.ExpectSequencerActive(true, nil)
	secondarySequencer.ExpectSequencerActive(false, nil)
	secondarySequencer.MaybeClose()
	ept.ethClients[1].MaybeClose()

	// // Should return to primary
	thirdSequencerUsed, err := endpointProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], thirdSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_InitialActiveSequencerSelection verifies that the ActiveL2RollupProvider
// selects the active sequencer correctly at the time of creation.
func TestRollupProvider_InitialActiveSequencerSelection(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer := ept.rollupClients[0]

	// Primary active at creation
	primarySequencer.ExpectSequencerActive(true, nil)

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	// Check immediately after creation without additional Active check
	require.Same(t, primarySequencer, rollupProvider.currentRollupClient)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_InitialActiveSequencerSelection verifies that the ActiveL2EndpointProvider
// selects the active sequencer correctly at the time of creation.
func TestEndpointProvider_InitialActiveSequencerSelection(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer := ept.rollupClients[0]

	// Primary active at creation
	primarySequencer.ExpectSequencerActive(true, nil)
	primarySequencer.ExpectSequencerActive(true, nil) // see comment in other tests about why we expect this twice

	rollupProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	// Check immediately after creation without additional Active check
	require.Same(t, primarySequencer, rollupProvider.currentRollupClient)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_SelectSecondSequencerIfFirstInactiveAtCreation verifies that if the first sequencer
// is inactive at the time of ActiveL2RollupProvider creation, the second active sequencer is chosen.
func TestRollupProvider_SelectSecondSequencerIfFirstInactiveAtCreation(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)

	// First sequencer is inactive, second sequencer is active
	ept.rollupClients[0].ExpectSequencerActive(false, nil)
	ept.rollupClients[0].MaybeClose()
	ept.rollupClients[1].ExpectSequencerActive(true, nil)

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	require.Same(t, ept.rollupClients[1], rollupProvider.currentRollupClient)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_SelectLastSequencerIfManyOfflineAtCreation verifies that if all but the last sequencer
// are offline at the time of ActiveL2RollupProvider creation, the last active sequencer is chosen.
func TestRollupProvider_SelectLastSequencerIfManyOfflineAtCreation(t *testing.T) {
	ept := setupEndpointProviderTest(t, 5)

	// First four sequencers are dead, last sequencer is active
	for i := 0; i < 4; i++ {
		ept.setRollupDialOutcome(i, false)
	}
	ept.rollupClients[4].ExpectSequencerActive(true, nil)

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	require.Same(t, ept.rollupClients[4], rollupProvider.currentRollupClient)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_SelectSecondSequencerIfFirstOfflineAtCreation verifies that if the first sequencer
// is inactive at the time of ActiveL2EndpointProvider creation, the second active sequencer is chosen.
func TestEndpointProvider_SelectSecondSequencerIfFirstOfflineAtCreation(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)

	// First sequencer is inactive, second sequencer is active
	ept.rollupClients[0].ExpectSequencerActive(false, nil)
	ept.rollupClients[0].MaybeClose()
	ept.rollupClients[1].ExpectSequencerActive(true, nil)
	ept.rollupClients[1].ExpectSequencerActive(true, nil) // see comment in other tests about why we expect this twice

	endpointProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	require.Same(t, ept.ethClients[1], endpointProvider.currentEthClient)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_SelectLastSequencerIfManyInactiveAtCreation verifies that if all but the last sequencer
// are inactive at the time of ActiveL2EndpointProvider creation, the last active sequencer is chosen.
func TestEndpointProvider_SelectLastSequencerIfManyInactiveAtCreation(t *testing.T) {
	ept := setupEndpointProviderTest(t, 5)

	// First four sequencers are dead, last sequencer is active
	for i := 0; i < 4; i++ {
		ept.setRollupDialOutcome(i, false)
	}
	ept.rollupClients[4].ExpectSequencerActive(true, nil)
	ept.rollupClients[4].ExpectSequencerActive(true, nil) // Double check due to embedded call of `EthClient()`

	endpointProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	require.Same(t, ept.ethClients[4], endpointProvider.currentEthClient)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_ConstructorErrorOnFirstSequencerOffline verifies that the ActiveL2RollupProvider
// constructor handles the case where the first sequencer (index 0) is offline at startup.
func TestRollupProvider_ConstructorErrorOnFirstSequencerOffline(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)

	// First sequencer is dead, second sequencer is active
	ept.rollupClients[0].ExpectSequencerActive(false, errors.New("I am offline"))
	ept.rollupClients[0].MaybeClose()
	ept.rollupClients[1].ExpectSequencerActive(true, nil)

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	require.Same(t, ept.rollupClients[1], rollupProvider.currentRollupClient)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_ConstructorErrorOnFirstSequencerOffline verifies that the ActiveL2EndpointProvider
// constructor handles the case where the first sequencer (index 0) is offline at startup.
func TestEndpointProvider_ConstructorErrorOnFirstSequencerOffline(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)

	// First sequencer is dead, second sequencer is active
	ept.rollupClients[0].ExpectSequencerActive(false, errors.New("I am offline"))
	ept.rollupClients[0].MaybeClose()
	ept.rollupClients[1].ExpectSequencerActive(true, nil)
	ept.rollupClients[1].ExpectSequencerActive(true, nil) // see comment in other tests about why we expect this twice

	endpointProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	require.Same(t, ept.ethClients[1], endpointProvider.currentEthClient)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_FailOnAllInactiveSequencers verifies that the ActiveL2RollupProvider
// fails to be created when all sequencers are inactive.
func TestRollupProvider_FailOnAllInactiveSequencers(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)

	// All sequencers are inactive
	for _, sequencer := range ept.rollupClients {
		sequencer.ExpectSequencerActive(false, nil)
		sequencer.MaybeClose()
	}

	_, err := ept.newActiveL2RollupProvider(0)
	require.Error(t, err) // Expect an error as all sequencers are inactive
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_FailOnAllInactiveSequencers verifies that the ActiveL2EndpointProvider
// fails to be created when all sequencers are inactive.
func TestEndpointProvider_FailOnAllInactiveSequencers(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)

	// All sequencers are inactive
	for _, sequencer := range ept.rollupClients {
		sequencer.ExpectSequencerActive(false, nil)
		sequencer.MaybeClose()
	}

	_, err := ept.newActiveL2EndpointProvider(0)
	require.Error(t, err) // Expect an error as all sequencers are inactive
	ept.assertAllExpectations(t)
}

// TestRollupProvider_FailOnAllErroredSequencers verifies that the ActiveL2RollupProvider
// fails to create when all sequencers return an error.
func TestRollupProvider_FailOnAllErroredSequencers(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)

	// All sequencers are inactive
	for _, sequencer := range ept.rollupClients {
		sequencer.ExpectSequencerActive(true, errors.New("a test error"))
		sequencer.MaybeClose()
	}

	_, err := ept.newActiveL2RollupProvider(0)
	require.Error(t, err) // Expect an error as all sequencers are inactive
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_FailOnAllErroredSequencers verifies that the ActiveL2EndpointProvider
// fails to create when all sequencers return an error.
func TestEndpointProvider_FailOnAllErroredSequencers(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)

	// All sequencers are inactive
	for _, sequencer := range ept.rollupClients {
		sequencer.ExpectSequencerActive(true, errors.New("a test error"))
		sequencer.MaybeClose()
	}

	_, err := ept.newActiveL2EndpointProvider(0)
	require.Error(t, err) // Expect an error as all sequencers are inactive
	ept.assertAllExpectations(t)
}

// TestRollupProvider_LongCheckDuration verifies the behavior of ActiveL2RollupProvider with a long check duration.
func TestRollupProvider_LongCheckDuration(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer := ept.rollupClients[0]

	longCheckDuration := 1 * time.Hour
	primarySequencer.ExpectSequencerActive(true, nil) // Active check on creation

	rollupProvider, err := ept.newActiveL2RollupProvider(longCheckDuration)
	require.NoError(t, err)

	// Should return the same client without extra checks
	firstSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primarySequencer, firstSequencerUsed)

	secondSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primarySequencer, secondSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_LongCheckDuration verifies the behavior of ActiveL2EndpointProvider with a long check duration.
func TestEndpointProvider_LongCheckDuration(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer := ept.rollupClients[0]

	longCheckDuration := 1 * time.Hour
	primarySequencer.ExpectSequencerActive(true, nil) // Active check on creation

	endpointProvider, err := ept.newActiveL2EndpointProvider(longCheckDuration)
	require.NoError(t, err)

	// Should return the same client without extra checks
	firstEthClient, err := endpointProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], firstEthClient)

	secondEthClient, err := endpointProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], secondEthClient)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_ErrorWhenAllSequencersInactive verifies that RollupClient() returns an error
// if all sequencers become inactive after the provider is successfully created.
func TestRollupProvider_ErrorWhenAllSequencersInactive(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	ept.rollupClients[0].ExpectSequencerActive(true, nil) // Main sequencer initially active

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	// All sequencers become inactive
	for _, sequencer := range ept.rollupClients {
		sequencer.ExpectSequencerActive(false, nil)
		sequencer.MaybeClose()
	}

	_, err = rollupProvider.RollupClient(context.Background())
	require.Error(t, err) // Expect an error as all sequencers are inactive
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_ErrorWhenAllSequencersInactive verifies that EthClient() returns an error
// if all sequencers become inactive after the provider is successfully created.
func TestEndpointProvider_ErrorWhenAllSequencersInactive(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	ept.rollupClients[0].ExpectSequencerActive(true, nil) // Main sequencer initially active
	ept.rollupClients[0].ExpectSequencerActive(true, nil) // Main sequencer initially active (double check due to embedded call of `EthClient()`)

	endpointProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	// All sequencers become inactive
	for _, sequencer := range ept.rollupClients {
		sequencer.ExpectSequencerActive(false, nil)
		sequencer.MaybeClose()
	}

	_, err = endpointProvider.EthClient(context.Background())
	require.Error(t, err) // Expect an error as all sequencers are inactive
	ept.assertAllExpectations(t)
}

// TestRollupProvider_ReturnsSameSequencerOnInactiveWithLongCheckDuration verifies that the ActiveL2RollupProvider
// still returns the same sequencer across calls even if it becomes inactive, due to a long check duration.
func TestRollupProvider_ReturnsSameSequencerOnInactiveWithLongCheckDuration(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer := ept.rollupClients[0]

	longCheckDuration := 1 * time.Hour
	primarySequencer.ExpectSequencerActive(true, nil) // Active on creation

	rollupProvider, err := ept.newActiveL2RollupProvider(longCheckDuration)
	require.NoError(t, err)

	// Primary sequencer becomes inactive, but the provider won't check immediately due to longCheckDuration
	primarySequencer.ExpectSequencerActive(false, nil)
	firstSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primarySequencer, firstSequencerUsed)

	active, err := primarySequencer.SequencerActive(context.Background())
	require.NoError(t, err)
	require.False(t, active)

	secondSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, primarySequencer, secondSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_ReturnsSameSequencerOnInactiveWithLongCheckDuration verifies that the ActiveL2EndpointProvider
// still returns the same sequencer across calls even if it becomes inactive, due to a long check duration.
func TestEndpointProvider_ReturnsSameSequencerOnInactiveWithLongCheckDuration(t *testing.T) {
	ept := setupEndpointProviderTest(t, 2)
	primarySequencer := ept.rollupClients[0]

	longCheckDuration := 1 * time.Hour
	primarySequencer.ExpectSequencerActive(true, nil) // Active on creation

	endpointProvider, err := ept.newActiveL2EndpointProvider(longCheckDuration)
	require.NoError(t, err)

	// Primary sequencer becomes inactive, but the provider won't check immediately due to longCheckDuration
	primarySequencer.ExpectSequencerActive(false, nil)
	firstEthClientUsed, err := endpointProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], firstEthClientUsed)

	active, err := primarySequencer.SequencerActive(context.Background())
	require.NoError(t, err)
	require.False(t, active)

	secondEthClientUsed, err := endpointProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], secondEthClientUsed)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_HandlesManyIndexClientMismatch verifies that the ActiveL2RollupProvider avoids
// the case where the index of the current sequencer does not match the index of the current rollup client.
func TestRollupProvider_HandlesManyIndexClientMismatch(t *testing.T) {
	ept := setupEndpointProviderTest(t, 3)
	seq0, seq1, seq2 := ept.rollupClients[0], ept.rollupClients[1], ept.rollupClients[2]

	// "start happy": primarySequencer is active on creation
	seq0.ExpectSequencerActive(true, nil) // active on creation
	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	// primarySequencer goes down
	seq0.ExpectSequencerActive(false, errors.New("I'm offline now"))
	seq0.MaybeClose()
	ept.setRollupDialOutcome(0, false) // primarySequencer fails to dial
	// secondarySequencer is inactive, but online
	seq1.ExpectSequencerActive(false, nil)
	seq1.MaybeClose()
	// tertiarySequencer can't even be dialed
	ept.setRollupDialOutcome(2, false)
	// In a prior buggy implementation, this scenario lead to an internal inconsistent state
	// where the current client didn't match the index. On a subsequent try, this led to the
	// active sequencer at 0 to be skipped entirely, while the sequencer at index 1
	// was checked twice.
	rollupClient, err := rollupProvider.RollupClient(context.Background())
	require.Error(t, err)
	require.Nil(t, rollupClient)
	// internal state would now be inconsistent in a buggy impl.

	// now seq0 is dialable and active
	ept.setRollupDialOutcome(0, true)
	seq0.ExpectSequencerActive(true, nil)
	seq0.MaybeClose()
	// now seq1 and seq2 are dialable, but inactive
	ept.setRollupDialOutcome(1, true)
	seq1.ExpectSequencerActive(false, nil)
	seq1.MaybeClose()
	ept.setRollupDialOutcome(2, true)
	seq2.ExpectSequencerActive(false, nil)
	seq2.MaybeClose()
	// this would trigger the prior bug: request the rollup client.
	rollupClient, err = rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, seq0, rollupClient)
	ept.assertAllExpectations(t)
}

// TestRollupProvider_HandlesSingleSequencer verifies that the ActiveL2RollupProvider
// can handle being passed a single sequencer endpoint without issue.
func TestRollupProvider_HandlesSingleSequencer(t *testing.T) {
	ept := setupEndpointProviderTest(t, 1)
	onlySequencer := ept.rollupClients[0]
	onlySequencer.ExpectSequencerActive(true, nil) // respond true once on creation

	rollupProvider, err := ept.newActiveL2RollupProvider(0)
	require.NoError(t, err)

	onlySequencer.ExpectSequencerActive(true, nil) // respond true again when the test calls `RollupClient()` the first time
	firstSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.NoError(t, err)
	require.Same(t, onlySequencer, firstSequencerUsed)

	onlySequencer.ExpectSequencerActive(false, nil) // become inactive after that
	onlySequencer.MaybeClose()
	secondSequencerUsed, err := rollupProvider.RollupClient(context.Background())
	require.Error(t, err)
	require.Nil(t, secondSequencerUsed)
	ept.assertAllExpectations(t)
}

// TestEndpointProvider_HandlesSingleSequencer verifies that the ActiveL2EndpointProvider
// can handle being passed a single sequencer endpoint without issue.
func TestEndpointProvider_HandlesSingleSequencer(t *testing.T) {
	ept := setupEndpointProviderTest(t, 1)
	onlySequencer := ept.rollupClients[0]
	onlySequencer.ExpectSequencerActive(true, nil) // respond true once on creation
	onlySequencer.ExpectSequencerActive(true, nil) // respond true again when the constructor calls `RollupClient()`

	endpointProvider, err := ept.newActiveL2EndpointProvider(0)
	require.NoError(t, err)

	onlySequencer.ExpectSequencerActive(true, nil) // respond true a once more on fall-through check in `EthClient()`
	firstEthClientUsed, err := endpointProvider.EthClient(context.Background())
	require.NoError(t, err)
	require.Same(t, ept.ethClients[0], firstEthClientUsed)

	onlySequencer.ExpectSequencerActive(false, nil) // become inactive after that
	onlySequencer.MaybeClose()
	secondEthClientUsed, err := endpointProvider.EthClient(context.Background())
	require.Error(t, err)
	require.Nil(t, secondEthClientUsed)
	ept.assertAllExpectations(t)
}
