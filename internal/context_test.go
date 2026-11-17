package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
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
	src := &testMockSource{people: []Person{{CompareValue: "a@example.com", Attributes: map[string]string{"email": "a@example.com"}}}}

	tests := []struct {
		name            string
		dryRun          bool
		cancelOnApply   bool
		wantApplyCalled bool
		wantErr         string
	}{
		{name: "canceled before applying", wantApplyCalled: false, wantErr: "before applying changes"},
		{name: "canceled before dry run output", dryRun: true, wantApplyCalled: false, wantErr: "before applying changes"},
		{name: "canceled while applying", cancelOnApply: true, wantApplyCalled: true, wantErr: "changes may be incomplete"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			dst := &testMockDestination{}
			if tt.cancelOnApply {
				dst.onApply = func() { cancel(cause) }
			} else {
				cancel(cause)
			}
			cfg := Config{
				Runtime:      RuntimeConfig{DryRunMode: tt.dryRun},
				AttributeMap: []AttributeMap{{Source: "email", Destination: "email"}},
			}

			err := RunSyncSet(ctx, log.New(&bytes.Buffer{}, "", 0), src, dst, cfg)
			require.ErrorIs(t, err, cause)
			require.ErrorContains(t, err, tt.wantErr)
			require.Equal(t, tt.wantApplyCalled, dst.applyCalled)
		})
	}
}

func TestSyncError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	err := fmt.Errorf("outer: %w", SyncError{Message: fmt.Errorf("middle: %w", inner), SendAlert: true})

	require.ErrorIs(t, err, inner)

	syncErr, ok := errors.AsType[SyncError](err)
	require.True(t, ok)
	require.True(t, syncErr.SendAlert)
}

func TestUntilDone(t *testing.T) {
	items := []string{"a", "b", "c", "d"}

	t.Run("yields everything when ctx is not done", func(t *testing.T) {
		var got []string
		for _, v := range UntilDone(t.Context(), slices.All(items)) {
			got = append(got, v)
		}
		require.Equal(t, items, got)
	})

	t.Run("yields nothing when ctx is already done", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		for range UntilDone(ctx, slices.All(items)) {
			t.Fatal("should not yield")
		}
	})

	t.Run("stops after ctx is canceled mid-loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var got []string
		for _, v := range UntilDone(ctx, slices.All(items)) {
			got = append(got, v)
			if v == "b" {
				cancel()
			}
		}
		require.Equal(t, []string{"a", "b"}, got)
	})

	t.Run("caller break stops iteration", func(t *testing.T) {
		var got []string
		for _, v := range UntilDone(t.Context(), slices.All(items)) {
			got = append(got, v)
			if v == "c" {
				break
			}
		}
		require.Equal(t, []string{"a", "b", "c"}, got)
	})
}
