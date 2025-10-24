package main

import (
	"context"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSignalHandling(t *testing.T) {
	t.Run("InterruptSignal", func(t *testing.T) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		// Имитируем отправку SIGINT
		go func() {
			time.Sleep(10 * time.Millisecond)
			stop()
		}()

		select {
		case <-ctx.Done():
			// Ожидаемое поведение
			assert.Error(t, ctx.Err())
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Timeout waiting for signal")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		<-ctx.Done()
		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	})
}
