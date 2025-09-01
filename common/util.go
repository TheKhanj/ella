package common

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func WaitFor[T comparable](ch <-chan T, val T) bool {
	for curr := range ch {
		if curr == val {
			return true
		}
	}

	return false
}

func NewSignalCtx(
	ctx context.Context,
) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	signal.Notify(stop, syscall.SIGINT)

	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-stop:
			cancel()
		}
	}()

	return ctx
}
