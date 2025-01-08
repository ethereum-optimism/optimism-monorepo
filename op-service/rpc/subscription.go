package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

func SubscribeRPC[T any](ctx context.Context, logger log.Logger, feed *event.FeedOf[T]) (*gethrpc.Subscription, error) {
	notifier, supported := gethrpc.NotifierFromContext(ctx)
	if !supported {
		return &gethrpc.Subscription{}, gethrpc.ErrNotificationsUnsupported
	}
	logger.Info("Opening subscription via RPC")

	rpcSub := notifier.CreateSubscription()
	ch := make(chan T, 10)
	feedSub := feed.Subscribe(ch)

	go func() {
		defer logger.Info("Closing RPC subscription")
		defer feedSub.Unsubscribe()

		for {
			select {
			case v := <-ch:
				if err := notifier.Notify(rpcSub.ID, v); err != nil {
					logger.Warn("Failed to notify RPC subscription", "err", err)
					return
				}
			case err, ok := <-rpcSub.Err():
				if !ok {
					logger.Debug("Exiting subscription")
					return
				}
				logger.Warn("RPC subscription failed", "err", err)
				return
			}
		}
	}()

	return rpcSub, nil
}
