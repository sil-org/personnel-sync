package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSleep(t *testing.T) {
	t.Run("full duration", func(t *testing.T) {
		start := time.Now()
		require.NoError(t, Sleep(t.Context(), 20*time.Millisecond))
		require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
	})

	t.Run("zero or negative duration", func(t *testing.T) {
		require.NoError(t, Sleep(t.Context(), 0))
		require.NoError(t, Sleep(t.Context(), -time.Second))
	})

	t.Run("already canceled", func(t *testing.T) {
		cause := errors.New("test cause")
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(cause)
		require.ErrorIs(t, Sleep(ctx, 0), cause)
	})

	t.Run("canceled while sleeping", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		start := time.Now()
		require.ErrorIs(t, Sleep(ctx, time.Hour), context.DeadlineExceeded)
		require.Less(t, time.Since(start), 5*time.Second)
	})
}

func TestBatchTimer_WaitOnBatchCanceled(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	b := NewBatchTimer(1, 3600)
	// The first batch doesn't wait (Init computes endTime before setting SecondsPerBatch), so wait on the second
	b.WaitOnBatch(t.Context())

	start := time.Now()
	b.WaitOnBatch(ctx)
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 15*time.Millisecond, "WaitOnBatch should have waited until ctx was done")
	require.Less(t, elapsed, 5*time.Second, "WaitOnBatch should return when ctx is done")
}

func TestRunSyncSet_Interrupted(t *testing.T) {
	cause := errors.New("test cause")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)

	src := &testMockSource{people: []Person{{CompareValue: "a@example.com", Attributes: map[string]string{"email": "a@example.com"}}}}
	dst := &testMockDestination{}
	cfg := Config{AttributeMap: []AttributeMap{{Source: "email", Destination: "email"}}}

	err := RunSyncSet(ctx, log.New(&bytes.Buffer{}, "", 0), src, dst, cfg)
	require.ErrorIs(t, err, cause)
	require.ErrorContains(t, err, "changes may be incomplete")
}

func TestSyncError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	err := fmt.Errorf("outer: %w", SyncError{Message: fmt.Errorf("middle: %w", inner), SendAlert: true})

	require.ErrorIs(t, err, inner)

	syncErr, ok := errors.AsType[SyncError](err)
	require.True(t, ok)
	require.True(t, syncErr.SendAlert)
}
