package p2p

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/snappy"
	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"golang.org/x/time/rate"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// StreamCtxFn provides a new context to use when handling stream requests
type StreamCtxFn func() context.Context

// Note: the mocknet in testing does not support read/write stream timeouts, the timeouts are only applied if available.
// Rate-limits always apply, and are making sure the request/response throughput is not too fast, instead of too slow.
const (
	// timeout for opening a req-resp stream to another peer. This may involve some protocol negotiation.
	streamTimeout = time.Second * 5
	// timeout for writing the request as client. Can be as long as serverReadRequestTimeout
	clientWriteRequestTimeout = time.Second * 10
	// timeout for reading a response of a serving peer as client. Can be as long as serverWriteChunkTimeout
	clientReadResponsetimeout = time.Second * 10
	// timeout for reading the request content, deny the request if it cannot be fully read in time
	serverReadRequestTimeout = time.Second * 10
	// timeout for writing a single response message chunk
	// (if a future response consists of multiple chunks, reset the writing timeout per chunk)
	serverWriteChunkTimeout = time.Second * 10
	// after the rate-limit reservation hits the max throttle delay, give up on serving a request and just close the stream
	maxThrottleDelay = time.Second * 20
	// Do not serve more than 20 requests per second
	globalServerBlocksRateLimit rate.Limit = 20
	// Allows a burst of 2x our rate limit
	globalServerBlocksBurst = 40
	// Do not serve more than 4 requests per second to the same peer, so we can serve other peers at the same time
	peerServerBlocksRateLimit rate.Limit = 4
	// Allow a peer to request 30s of blocks at once
	peerServerBlocksBurst = 15
	// If the client hits a request error, it counts as a lot of rate-limit tokens for syncing from that peer:
	// we rather sync from other servers. We'll try again later,
	// and eventually kick the peer based on degraded scoring if it's really not serving us well.
	// TODO: Use a backoff rather than this mechanism.
	clientErrRateCost = peerServerBlocksBurst
)

const (
	ResultCodeSuccess     byte = 0
	ResultCodeNotFoundErr byte = 1
	ResultCodeInvalidErr  byte = 2
	ResultCodeUnknownErr  byte = 3
)

var resultCodeString = []string{
	"success",
	"not found",
	"invalid request",
	"unknown error",
}

func PayloadByNumberProtocolID(l2ChainID *big.Int) protocol.ID {
	return protocol.ID(fmt.Sprintf("/opstack/req/payload_by_number/%d/0", l2ChainID))
}

type requestHandlerFn func(ctx context.Context, log log.Logger, stream network.Stream)

func MakeStreamHandler(resourcesCtx context.Context, log log.Logger, fn requestHandlerFn) network.StreamHandler {
	return func(stream network.Stream) {
		log := log.New("peer", stream.Conn().ID(), "remote", stream.Conn().RemoteMultiaddr())
		defer func() {
			if err := recover(); err != nil {
				log.Error("p2p server request handling panic", "err", err, "protocol", stream.Protocol())
			}
		}()
		defer stream.Close()
		fn(resourcesCtx, log, stream)
	}
}

type newStreamFn func(ctx context.Context, peerId peer.ID, protocolId ...protocol.ID) (network.Stream, error)

type receivePayloadFn func(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope) error

type rangeRequest struct {
	start uint64
	end   eth.L2BlockRef
	id    uint64
}

type syncResult struct {
	payload *eth.ExecutionPayloadEnvelope
	peer    peer.ID
}

type peerRequest struct {
	num        uint64
	rangeReqId uint64
}

type inFlightCheck struct {
	num    uint64
	result chan bool
}

type requestIdMap struct {
	requests map[uint64]bool
	mu       sync.Mutex
}

func newRequestIdMap() *requestIdMap {
	return &requestIdMap{
		requests: make(map[uint64]bool),
	}
}

func (r *requestIdMap) set(key uint64, value bool) {
	r.mu.Lock()
	r.requests[key] = value
	r.mu.Unlock()
}

func (r *requestIdMap) get(key uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests[key]
}

func (r *requestIdMap) delete(key uint64) {
	r.mu.Lock()
	delete(r.requests, key)
	r.mu.Unlock()
}

type SyncClientMetrics interface {
	ClientPayloadByNumberEvent(num uint64, resultCode byte, duration time.Duration)
	PayloadsQuarantineSize(n int)
}

type SyncPeerScorer interface {
	onValidResponse(id peer.ID)
	onResponseError(id peer.ID)
	onRejectedPayload(id peer.ID)
}

// SyncClient implements a reverse chain sync with a minimal interface:
// signal the desired range, and receive blocks within this range back.
// Through parent-hash verification, received blocks are all ensured to be part of the canonical chain at one point,
// but it is up to the user to organize and process the results further.
//
// For the sync-client to retrieve any data, peers must be added with AddPeer(id), and removed upon disconnect with RemovePeer(id).
// The client is started with Start(), and may be started before or after changing any peers.
//
// ### Stages
//
// The sync mechanism is implemented as following:
// - User sends range request: blocks on sync main loop (with ctx timeout)
// - Main loop processes range request (from high to low), dividing block requests by number between parallel peers.
//   - The high part of the range has a known block-hash, and is marked as trusted.
//   - Once there are no more peers available for buffering requests, we stop the range request processing.
//   - Every request buffered for a peer is tracked as in-flight, by block number.
//   - In-flight requests are not repeated
//   - Requests for data that's already in the quarantine are not repeated
//   - Data already in the quarantine that is trusted is attempted to be promoted.
//
// - Peers each have their own routine for processing requests.
//   - They fetch the requested block by number, parse and validate it, and then send it back to the main loop
//   - If peers fail to fetch or process it, or fail to send it back to the main loop within timeout,
//     then the doRequest returns an error. It then marks the in-flight request as completed.
//
// - Main loop receives results synchronously with the range requests
//   - The result is removed from in-flight tracker
//   - The result is added to the quarantine
//   - If we trust the hash, we try to promote the result.
//
// ### Concepts
//
// The main concepts are:
// - Quarantine: an LRU that stores the latest fetched block data, by hash as well as an extra index by number.
//
//   - Quarantine eviction: upon regular LRU eviction, or explicit removal (when we learn data is not canonical),
//     the sync result is removed from quarantine without being forwarded to the receiver.
//     The peer that provided the data may be down-scored for providing un-utilized data if the data
//     is not trusted during eviction.
//
// - Trusted data: data becomes trusted through 2 ways:
//   - The hash / parent-hash of the sync target is marked as trusted.
//   - The parent-hash of any promoted data is marked as trusted.
//
// - The trusted-data is maintained in LRU: we only care about the recent accessed blocks.
//
//   - Result promotion: content from the quarantine is "promoted" when we find the blockhash is trusted.
//     The data is removed from the quarantine, and forwarded to the receiver.
//
// ### Usage
//
// The user is expected to request the range of blocks between its existing chain head,
// and a trusted future block-hash as reference to sync towards.
// Upon receiving results from the sync-client, the user should adjust down its sync-target
// based on the received results, to avoid duplicating work when req-requesting an updated range.
// Range requests should still be repeated eventually however, as the sync client will give up on syncing a large range
// when it's too busy syncing.
//
// The rationale for this approach is that this sync mechanism is primarily intended
// for quickly filling gaps between an existing chain and a gossip chain, and not for very long block ranges.
// Syncing in the execution-layer (through snap-sync) is more appropriate for long ranges.
// If the user does sync a long range of blocks through this mechanism,
// it does end up traversing through the chain, but receives the blocks in reverse order.
// It is up to the user to persist the blocks for later processing, or drop & resync them if persistence is limited.
type SyncClient struct {
	log log.Logger

	cfg *rollup.Config

	metrics   SyncClientMetrics
	appScorer SyncPeerScorer

	newStreamFn     newStreamFn
	payloadByNumber protocol.ID

	peersLock sync.Mutex
	// syncing worker per peer
	peers map[peer.ID]context.CancelFunc

	// trusted blocks are, or have been, canonical at one point.
	// Everything that's trusted is acceptable to pass to the sync receiver,
	// but we target to just sync the blocks of the latest canonical view of the chain.
	trusted *simplelru.LRU[common.Hash, struct{}]

	// quarantine is a LRU of untrusted results: blocks that could not be verified yet
	quarantine *simplelru.LRU[common.Hash, syncResult]
	// quarantineByNum indexes the quarantine contents by number.
	// No duplicates here, only the latest quarantine write is indexed.
	// This map is cleared upon evictions of items from the quarantine LRU
	quarantineByNum map[uint64]common.Hash

	// inFlight requests are not repeated
	inFlight       *requestIdMap
	inFlightChecks chan inFlightCheck

	rangeRequests       chan rangeRequest
	activeRangeRequests *requestIdMap
	rangeReqId          uint64
	peerRequests        chan peerRequest

	results chan syncResult

	receivePayload receivePayloadFn

	// Global rate limiter for all peers.
	globalRL *rate.Limiter

	// resource context: all peers and mainLoop tasks inherit this, and start shutting down once resCancel() is called.
	resCtx    context.Context
	resCancel context.CancelFunc

	// wait group: wait for the resources to close. Adding to this is only safe if the peersLock is held.
	wg sync.WaitGroup

	// Don't allow anything to be added to the wait-group while, or after, we are shutting down.
	// This is protected by peersLock.
	closingPeers bool

	extra               ExtraHostFeatures
	syncOnlyReqToStatic bool
}

func NewSyncClient(log log.Logger, cfg *rollup.Config, host HostNewStream, rcv receivePayloadFn, metrics SyncClientMetrics, appScorer SyncPeerScorer) *SyncClient {
	ctx, cancel := context.WithCancel(context.Background())

	c := &SyncClient{
		log:                 log,
		cfg:                 cfg,
		metrics:             metrics,
		appScorer:           appScorer,
		newStreamFn:         host.NewStream,
		payloadByNumber:     PayloadByNumberProtocolID(cfg.L2ChainID),
		peers:               make(map[peer.ID]context.CancelFunc),
		quarantineByNum:     make(map[uint64]common.Hash),
		rangeRequests:       make(chan rangeRequest), // blocking
		activeRangeRequests: newRequestIdMap(),
		peerRequests:        make(chan peerRequest, 128),
		results:             make(chan syncResult, 128),
		inFlight:            newRequestIdMap(),
		inFlightChecks:      make(chan inFlightCheck, 128),
		globalRL:            rate.NewLimiter(globalServerBlocksRateLimit, globalServerBlocksBurst),
		resCtx:              ctx,
		resCancel:           cancel,
		receivePayload:      rcv,
	}
	if extra, ok := host.(ExtraHostFeatures); ok && extra.SyncOnlyReqToStatic() {
		c.extra = extra
		c.syncOnlyReqToStatic = true
	}

	// never errors with positive LRU cache size
	// TODO: if we had an LRU based on on total payloads size, instead of payload count,
	//  we can safely buffer more data in the happy case.
	q, _ := simplelru.NewLRU[common.Hash, syncResult](100, c.onQuarantineEvict)
	c.quarantine = q
	trusted, _ := simplelru.NewLRU[common.Hash, struct{}](10000, nil)
	c.trusted = trusted
	return c
}

func (s *SyncClient) Start() {
	s.peersLock.Lock()
	s.wg.Add(1)
	s.peersLock.Unlock()
	go s.mainLoop()
}

func (s *SyncClient) AddPeer(id peer.ID) {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()
	if s.closingPeers {
		return
	}
	if _, ok := s.peers[id]; ok {
		s.log.Warn("cannot register peer for sync duties, peer was already registered", "peer", id)
		return
	}
	s.wg.Add(1)
	// add new peer routine
	ctx, cancel := context.WithCancel(s.resCtx)
	s.peers[id] = cancel
	go s.peerLoop(ctx, id)
}

func (s *SyncClient) RemovePeer(id peer.ID) {
	s.peersLock.Lock()
	defer s.peersLock.Unlock()
	if s.closingPeers {
		return
	}
	cancel, ok := s.peers[id]
	if !ok {
		s.log.Warn("cannot remove peer from sync duties, peer was not registered", "peer", id)
		return
	}
	cancel() // once loop exits
	delete(s.peers, id)
}

// Close will shut down the sync client and all attached work, and block until shutdown is complete.
// This will block if the Start() has not created the main background loop.
func (s *SyncClient) Close() error {
	s.peersLock.Lock()
	s.closingPeers = true
	s.peersLock.Unlock()
	s.resCancel()
	s.wg.Wait()
	return nil
}

func (s *SyncClient) RequestL2Range(ctx context.Context, start, end eth.L2BlockRef) (uint64, error) {
	if end == (eth.L2BlockRef{}) {
		s.log.Debug("P2P sync client received range signal, but cannot sync open-ended chain: need sync target to verify blocks through parent-hashes", "start", start)
		return 0, nil
	}
	// Create shared rangeReqId so associated peerRequests can all be cancelled by setting a single flag
	rangeReqId := atomic.AddUint64(&s.rangeReqId, 1)
	// need to flag request as active before adding request to s.rangeRequests to avoid race
	s.activeRangeRequests.set(rangeReqId, true)

	// synchronize requests with the main loop for state access
	select {
	case s.rangeRequests <- rangeRequest{start: start.Number, end: end, id: rangeReqId}:
		return rangeReqId, nil
	case <-ctx.Done():
		s.activeRangeRequests.delete(rangeReqId)
		return rangeReqId, fmt.Errorf("too busy with P2P results/requests: %w", ctx.Err())
	}
}

const (
	maxRequestScheduling = time.Second * 3
	maxResultProcessing  = time.Second * 3
)

func (s *SyncClient) mainLoop() {
	defer s.wg.Done()
	for {
		select {
		case req := <-s.rangeRequests:
			ctx, cancel := context.WithTimeout(s.resCtx, maxRequestScheduling)
			s.onRangeRequest(ctx, req)
			cancel()
		case res := <-s.results:
			ctx, cancel := context.WithTimeout(s.resCtx, maxResultProcessing)
			s.onResult(ctx, res)
			cancel()
		case check := <-s.inFlightChecks:
			s.log.Info("Checking in flight", "num", check.num)
			check.result <- s.inFlight.get(check.num)
		case <-s.resCtx.Done():
			s.log.Info("stopped P2P req-resp L2 block sync client")
			return
		}
	}
}

func (s *SyncClient) isInFlight(ctx context.Context, num uint64) (bool, error) {
	check := inFlightCheck{num: num, result: make(chan bool, 1)}
	select {
	case s.inFlightChecks <- check:
	case <-ctx.Done():
		return false, errors.New("context cancelled when publishing in flight check")
	}
	select {
	case res := <-check.result:
		return res, nil
	case <-ctx.Done():
		return false, errors.New("context cancelled while waiting for in flight check response")
	}
}

// onRangeRequest is exclusively called by the main loop, and has thus direct access to the request bookkeeping state.
// This function transforms requested block ranges into work for each peer.
func (s *SyncClient) onRangeRequest(ctx context.Context, req rangeRequest) {
	log := s.log.New("target", req.start, "end", req.end)
	log.Info("processing L2 range request", "rangeReqId", req.id)

	// add req head to trusted set of blocks
	s.trusted.Add(req.end.Hash, struct{}{})
	s.trusted.Add(req.end.ParentHash, struct{}{})

	// Now try to fetch lower numbers than current end, to traverse back towards the updated start.
	for i := uint64(0); ; i++ {
		num := req.end.Number - 1 - i
		if num <= req.start {
			return
		}
		// check if we have something in quarantine already
		if h, ok := s.quarantineByNum[num]; ok {
			if s.trusted.Contains(h) { // if we trust it, try to promote it.
				s.tryPromote(h)
			}
			// Don't fetch things that we have a candidate for already.
			// We'll evict it from quarantine by finding a conflict, or if we sync enough other blocks
			continue
		}

		if s.inFlight.get(num) {
			log.Debug("request still in-flight, not rescheduling sync request", "num", num)
			continue // request still in flight
		}
		pr := peerRequest{num: num, rangeReqId: req.id}

		log.Debug("Scheduling P2P block request", "num", num, "rangeReqId", req.id)
		// schedule number
		select {
		case s.peerRequests <- pr:
			s.inFlight.set(num, true)
		case <-ctx.Done():
			log.Info("did not schedule full P2P sync range", "current", num, "err", ctx.Err())
			return
		default: // peers may all be busy processing requests already
			log.Info("no peers ready to handle block requests for more P2P requests for L2 block history", "current", num)
			return
		}
	}
}

func (s *SyncClient) onQuarantineEvict(key common.Hash, value syncResult) {
	delete(s.quarantineByNum, uint64(value.payload.ExecutionPayload.BlockNumber))
	s.metrics.PayloadsQuarantineSize(s.quarantine.Len())
	if !s.trusted.Contains(key) {
		s.log.Debug("evicting untrusted payload from quarantine", "id", value.payload.ExecutionPayload.ID(), "peer", value.peer)
		// Down-score peer for having provided us a bad block that never turned out to be canonical
		s.appScorer.onRejectedPayload(value.peer)
	} else {
		s.log.Debug("evicting trusted payload from quarantine", "id", value.payload.ExecutionPayload.ID(), "peer", value.peer)
	}
}

func (s *SyncClient) tryPromote(h common.Hash) {
	parentRes, ok := s.quarantine.Get(h)
	if ok {
		// Simply reschedule the result, to get it (and possibly its parents) out of quarantine without recursion.
		// s.results is buffered, but skip the promotion if the channel is full as it would cause a deadlock.
		select {
		case s.results <- parentRes:
		default:
			s.log.Debug("failed to signal block for promotion: sync client is too busy", "h", h)
		}
	} else {
		s.log.Debug("cannot find block in quarantine, nothing to promote", "h", h)
	}
}

func (s *SyncClient) promote(ctx context.Context, res syncResult) {
	s.log.Debug("promoting p2p sync result", "payload", res.payload.ExecutionPayload.ID(), "peer", res.peer)

	if err := s.receivePayload(ctx, res.peer, res.payload); err != nil {
		s.log.Warn("failed to promote payload, receiver error", "err", err)
		return
	}
	s.trusted.Add(res.payload.ExecutionPayload.BlockHash, struct{}{})
	if s.quarantine.Remove(res.payload.ExecutionPayload.BlockHash) {
		s.log.Debug("promoted previously p2p-synced block from quarantine to main", "id", res.payload.ExecutionPayload.ID())
	} else {
		s.log.Debug("promoted new p2p-synced block to main", "id", res.payload.ExecutionPayload.ID())
	}

	// Mark parent block as trusted, so that we can promote it once we receive it / find it
	s.trusted.Add(res.payload.ExecutionPayload.ParentHash, struct{}{})

	// Try to promote the parent block too, if any: previous unverifiable data may now be canonical
	s.tryPromote(res.payload.ExecutionPayload.ParentHash)

	// In case we don't have the parent, and what we have in quarantine is wrong,
	// clear what we buffered in favor of fetching something else.
	if h, ok := s.quarantineByNum[uint64(res.payload.ExecutionPayload.BlockNumber)-1]; ok {
		s.quarantine.Remove(h)
	}
}

// onResult is exclusively called by the main loop, and has thus direct access to the request bookkeeping state.
// This function verifies if the result is canonical, and either promotes the result or moves the result into quarantine.
func (s *SyncClient) onResult(ctx context.Context, res syncResult) {
	payload := res.payload.ExecutionPayload
	s.log.Debug("processing p2p sync result", "payload", payload.ID(), "peer", res.peer)
	// Clean up the in-flight request, we have a result now.
	s.inFlight.delete(uint64(payload.BlockNumber))
	// Always put it in quarantine first. If promotion fails because the receiver is too busy, this functions as cache.
	s.quarantine.Add(payload.BlockHash, res)
	s.quarantineByNum[uint64(payload.BlockNumber)] = payload.BlockHash
	s.metrics.PayloadsQuarantineSize(s.quarantine.Len())
	// If we know this block is canonical, then promote it
	if s.trusted.Contains(payload.BlockHash) {
		s.promote(ctx, res)
	}
}

// peerLoop for syncing from a single peer
func (s *SyncClient) peerLoop(ctx context.Context, id peer.ID) {
	defer func() {
		s.peersLock.Lock()
		delete(s.peers, id) // clean up
		s.log.Debug("stopped syncing loop of peer", "id", id)
		s.wg.Done()
		s.peersLock.Unlock()
	}()

	log := s.log.New("peer", id)
	log.Info("Starting P2P sync client event loop")

	// Implement the same rate limits as the server does per-peer,
	// so we don't be too aggressive to the server.
	rl := rate.NewLimiter(peerServerBlocksRateLimit, peerServerBlocksBurst)

	// if onlyReqToStatic is on, ensure that only static peers are dealing with the request
	peerRequests := s.peerRequests
	if s.syncOnlyReqToStatic && !s.extra.IsStatic(id) {
		// for non-static peers, set peerRequests to nil
		// this will effectively make the peer loop not perform outgoing sync-requests.
		// while sync-requests will block, the loop may still process other events (if added in the future).
		peerRequests = nil
	}

	for {
		// wait for a global allocation to be available
		if err := s.globalRL.Wait(ctx); err != nil {
			return
		}
		// wait for peer to be available for more work
		if err := rl.Wait(ctx); err != nil {
			return
		}

		// once the peer is available, wait for a sync request.
		select {
		case pr := <-peerRequests:
			if !s.activeRangeRequests.get(pr.rangeReqId) {
				log.Debug("dropping cancelled p2p sync request", "num", pr.num)
				s.inFlight.delete(pr.num)
				continue
			}

			// We already established the peer is available w.r.t. rate-limiting,
			// and this is the only loop over this peer, so we can request now.
			start := time.Now()

			resultCode := ResultCodeSuccess
			err := panicGuard(s.doRequest)(ctx, id, pr.num)
			if err != nil {
				s.inFlight.delete(pr.num)
				log.Warn("failed p2p sync request", "num", pr.num, "err", err)
				resultCode = ResultCodeNotFoundErr
				sendResponseError := true

				if re, ok := err.(requestResultErr); ok {
					resultCode = re.ResultCode()
					if resultCode == ResultCodeNotFoundErr {
						log.Warn("cancelling p2p sync range request", "rangeReqId", pr.rangeReqId)
						s.activeRangeRequests.delete(pr.rangeReqId)
						sendResponseError = false // don't penalize peer for this error
					}
				}

				if sendResponseError {
					s.appScorer.onResponseError(id)
				}

				// If we hit an error, then count it as many requests.
				// We'd like to avoid making more requests for a while, so back off.
				if err := rl.WaitN(ctx, clientErrRateCost); err != nil {
					return
				}
			} else {
				log.Debug("completed p2p sync request", "num", pr.num)
				s.appScorer.onValidResponse(id)
			}

			took := time.Since(start)
			s.metrics.ClientPayloadByNumberEvent(pr.num, resultCode, took)
		case <-ctx.Done():
			return
		}
	}
}

type requestResultErr byte

func (r requestResultErr) Error() string {
	var errStr string
	if ri := int(r); ri < len(resultCodeString) {
		errStr = resultCodeString[ri]
	} else {
		errStr = "invalid code"
	}
	return fmt.Sprintf("peer failed to serve request with code %d: %s", uint8(r), errStr)
}

func (r requestResultErr) ResultCode() byte {
	return byte(r)
}

func (s *SyncClient) doRequest(ctx context.Context, id peer.ID, expectedBlockNum uint64) error {
	// open stream to peer
	reqCtx, reqCancel := context.WithTimeout(ctx, streamTimeout)
	str, err := s.newStreamFn(reqCtx, id, s.payloadByNumber)
	reqCancel()
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer str.Close()
	// set write timeout (if available)
	_ = str.SetWriteDeadline(time.Now().Add(clientWriteRequestTimeout))
	if err := binary.Write(str, binary.LittleEndian, expectedBlockNum); err != nil {
		return fmt.Errorf("failed to write request (%d): %w", expectedBlockNum, err)
	}
	if err := str.CloseWrite(); err != nil {
		return fmt.Errorf("failed to close writer side while making request: %w", err)
	}

	// set read timeout (if available)
	_ = str.SetReadDeadline(time.Now().Add(clientReadResponsetimeout))

	// Limit input, as well as output.
	// Compression may otherwise continue to read ignored data for a small output,
	// or output more data than desired (zip-bomb)
	r := io.LimitReader(str, maxGossipSize)
	var result [1]byte
	if _, err := io.ReadFull(r, result[:]); err != nil {
		return fmt.Errorf("failed to read result part of response: %w", err)
	}
	if res := result[0]; res != 0 {
		return requestResultErr(res)
	}
	var versionData [4]byte
	if _, err := io.ReadFull(r, versionData[:]); err != nil {
		return fmt.Errorf("failed to read version part of response: %w", err)
	}

	// payload is SSZ encoded with Snappy framed compression
	r = snappy.NewReader(r)
	r = io.LimitReader(r, maxGossipSize)

	// We cannot stream straight into the SSZ decoder, since we need the scope of the SSZ payload.
	// The server does not prepend it, nor would we trust a claimed length anyway, so we buffer the data we get.
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	version := binary.LittleEndian.Uint32(versionData[:])
	isCanyon := s.cfg.IsCanyon(s.cfg.TimestampForBlock(expectedBlockNum))
	isIsthmus := s.cfg.IsIsthmus(s.cfg.TimestampForBlock(expectedBlockNum))
	envelope, err := readExecutionPayload(version, data, isCanyon, isIsthmus)
	if err != nil {
		return err
	}
	if err := str.CloseRead(); err != nil {
		return fmt.Errorf("failed to close reading side")
	}
	if err := verifyBlock(envelope, expectedBlockNum); err != nil {
		return fmt.Errorf("received execution payload is invalid: %w", err)
	}
	select {
	case s.results <- syncResult{payload: envelope, peer: id}:
	case <-ctx.Done():
		return fmt.Errorf("failed to process response, sync client is too busy: %w", err)
	}
	return nil
}

// panicGuard is a generic function that takes another function with generic arguments and returns an error.
// It recovers from any panic that occurs during the execution of the function.
func panicGuard[T, S, U any](fn func(T, S, U) error) func(T, S, U) error {
	return func(arg0 T, arg1 S, arg2 U) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("recovered from a panic: %v", r)
			}
		}()
		return fn(arg0, arg1, arg2)
	}
}

// readExecutionPayload will unmarshal the supplied data into an ExecutionPayloadEnvelope.
func readExecutionPayload(version uint32, data []byte, isCanyon, isIsthmus bool) (*eth.ExecutionPayloadEnvelope, error) {
	switch version {
	case 0:
		blockVersion := eth.BlockV1
		if isCanyon {
			blockVersion = eth.BlockV2
		}
		var res eth.ExecutionPayload
		if err := res.UnmarshalSSZ(blockVersion, uint32(len(data)), bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &eth.ExecutionPayloadEnvelope{ExecutionPayload: &res}, nil
	case 1:
		envelope := &eth.ExecutionPayloadEnvelope{}
		blockVersion := eth.BlockV3
		if isIsthmus {
			blockVersion = eth.BlockV4
		}
		if err := envelope.UnmarshalSSZ(blockVersion, uint32(len(data)), bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("failed to decode execution payload envelope response: %w", err)
		}
		return envelope, nil
	default:
		return nil, fmt.Errorf("unrecognized version: %d", version)
	}
}

func verifyBlock(envelope *eth.ExecutionPayloadEnvelope, expectedNum uint64) error {
	payload := envelope.ExecutionPayload

	// verify L2 block
	if expectedNum != uint64(payload.BlockNumber) {
		return fmt.Errorf("received execution payload for block %d, but expected block %d", payload.BlockNumber, expectedNum)
	}
	actual, ok := envelope.CheckBlockHash()
	if !ok { // payload itself contains bad block hash
		return fmt.Errorf("received execution payload for block %d with bad block hash %s, expected %s", expectedNum, payload.BlockHash, actual)
	}
	return nil
}

// peerStat maintains rate-limiting data of a peer that requests blocks from us.
type peerStat struct {
	// Requests tokenizes each request to sync
	Requests *rate.Limiter
}

type L2Chain interface {
	PayloadByNumber(ctx context.Context, number uint64) (*eth.ExecutionPayloadEnvelope, error)
}

type ReqRespServerMetrics interface {
	ServerPayloadByNumberEvent(num uint64, resultCode byte, duration time.Duration)
}

type ReqRespServer struct {
	cfg *rollup.Config

	l2 L2Chain

	metrics ReqRespServerMetrics

	peerRateLimits *simplelru.LRU[peer.ID, *peerStat]
	peerStatsLock  sync.Mutex

	globalRequestsRL *rate.Limiter
}

func NewReqRespServer(cfg *rollup.Config, l2 L2Chain, metrics ReqRespServerMetrics) *ReqRespServer {
	// We should never allow over 1000 different peers to churn through quickly,
	// so it's fine to prune rate-limit details past this.

	peerRateLimits, _ := simplelru.NewLRU[peer.ID, *peerStat](1000, nil)
	globalRequestsRL := rate.NewLimiter(globalServerBlocksRateLimit, globalServerBlocksBurst)

	return &ReqRespServer{
		cfg:              cfg,
		l2:               l2,
		metrics:          metrics,
		peerRateLimits:   peerRateLimits,
		globalRequestsRL: globalRequestsRL,
	}
}

// HandleSyncRequest is a stream handler function to register the L2 unsafe payloads alt-sync protocol.
// See MakeStreamHandler to transform this into a LibP2P handler function.
//
// Note that the same peer may open parallel streams.
//
// The caller must Close the stream.
func (srv *ReqRespServer) HandleSyncRequest(ctx context.Context, log log.Logger, stream network.Stream) {
	// may stay 0 if we fail to decode the request
	start := time.Now()

	// We wait as long as necessary; we throttle the peer instead of disconnecting,
	// unless the delay reaches a threshold that is unreasonable to wait for.
	ctx, cancel := context.WithTimeout(ctx, maxThrottleDelay)
	req, err := srv.handleSyncRequest(ctx, stream)
	cancel()

	resultCode := ResultCodeSuccess
	if err != nil {
		log.Warn("failed to serve p2p sync request", "req", req, "err", err)
		if errors.Is(err, ethereum.NotFound) {
			resultCode = ResultCodeNotFoundErr
		} else if errors.Is(err, errInvalidRequest) {
			resultCode = ResultCodeInvalidErr
		} else {
			resultCode = ResultCodeUnknownErr
		}
		// try to write error code, so the other peer can understand the reason for failure.
		_, _ = stream.Write([]byte{resultCode})
	} else {
		log.Debug("successfully served sync response", "req", req)
	}
	srv.metrics.ServerPayloadByNumberEvent(req, resultCode, time.Since(start))
}

var errInvalidRequest = errors.New("invalid request")

func (srv *ReqRespServer) handleSyncRequest(ctx context.Context, stream network.Stream) (uint64, error) {
	peerId := stream.Conn().RemotePeer()

	// take a token from the global rate-limiter,
	// to make sure there's not too much concurrent server work between different peers.
	if err := srv.globalRequestsRL.Wait(ctx); err != nil {
		return 0, fmt.Errorf("timed out waiting for global sync rate limit: %w", err)
	}

	// find rate limiting data of peer, or add otherwise
	srv.peerStatsLock.Lock()
	ps, _ := srv.peerRateLimits.Get(peerId)
	if ps == nil {
		ps = &peerStat{
			Requests: rate.NewLimiter(peerServerBlocksRateLimit, peerServerBlocksBurst),
		}
		srv.peerRateLimits.Add(peerId, ps)
		ps.Requests.Reserve() // count the hit, but make it delay the next request rather than immediately waiting
	} else {
		// Only wait if it's an existing peer, otherwise the instant rate-limit Wait call always errors.

		// If the requester thinks we're taking too long, then it's their problem and they can disconnect.
		// We'll disconnect ourselves only when failing to read/write,
		// if the work is invalid (range validation), or when individual sub tasks timeout.
		if err := ps.Requests.Wait(ctx); err != nil {
			return 0, fmt.Errorf("timed out waiting for global sync rate limit: %w", err)
		}
	}
	srv.peerStatsLock.Unlock()

	// Set read deadline, if available
	_ = stream.SetReadDeadline(time.Now().Add(serverReadRequestTimeout))

	// Read the request
	var req uint64
	if err := binary.Read(stream, binary.LittleEndian, &req); err != nil {
		return 0, fmt.Errorf("failed to read requested block number: %w", err)
	}
	if err := stream.CloseRead(); err != nil {
		return req, fmt.Errorf("failed to close reading-side of a P2P sync request call: %w", err)
	}

	// Check the request is within the expected range of blocks
	if req < srv.cfg.Genesis.L2.Number {
		return req, fmt.Errorf("cannot serve request for L2 block %d before genesis %d: %w", req, srv.cfg.Genesis.L2.Number, errInvalidRequest)
	}
	max, err := srv.cfg.TargetBlockNumber(uint64(time.Now().Unix()))
	if err != nil {
		return req, fmt.Errorf("cannot determine max target block number to verify request: %w", errInvalidRequest)
	}
	if req > max {
		return req, fmt.Errorf("cannot serve request for L2 block %d after max expected block (%v): %w", req, max, errInvalidRequest)
	}

	envelope, err := srv.l2.PayloadByNumber(ctx, req)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return req, fmt.Errorf("peer requested unknown block by number: %w", err)
		} else {
			return req, fmt.Errorf("failed to retrieve payload to serve to peer: %w", err)
		}
	}

	// We set write deadline, if available, to safely write without blocking on a throttling peer connection
	_ = stream.SetWriteDeadline(time.Now().Add(serverWriteChunkTimeout))

	w := snappy.NewBufferedWriter(stream)

	if srv.cfg.IsEcotone(uint64(envelope.ExecutionPayload.Timestamp)) {
		// 0 - resultCode: success = 0
		// 1:5 - version: 1 (little endian)
		tmp := [5]byte{0, 1, 0, 0, 0}
		if _, err := stream.Write(tmp[:]); err != nil {
			return req, fmt.Errorf("failed to write response header data: %w", err)
		}
		if _, err := envelope.MarshalSSZ(w); err != nil {
			return req, fmt.Errorf("failed to write payload to sync response: %w", err)
		}
	} else {
		// 0 - resultCode: success = 0
		// 1:5 - version: 0
		var tmp [5]byte
		if _, err := stream.Write(tmp[:]); err != nil {
			return req, fmt.Errorf("failed to write response header data: %w", err)
		}
		if _, err := envelope.ExecutionPayload.MarshalSSZ(w); err != nil {
			return req, fmt.Errorf("failed to write payload to sync response: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return req, fmt.Errorf("failed to finishing writing payload to sync response: %w", err)
	}

	return req, nil
}
