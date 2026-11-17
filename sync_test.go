package personnel_sync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithReportingMargin(t *testing.T) {
	t.Run("no deadline", func(t *testing.T) {
		ctx, cancel := withReportingMargin(t.Context())
		defer cancel()
		_, ok := ctx.Deadline()
		require.False(t, ok)
	})

	t.Run("long deadline uses max margin", func(t *testing.T) {
		parentDeadline := time.Now().Add(15 * time.Minute)
		parent, cancelParent := context.WithDeadline(t.Context(), parentDeadline)
		defer cancelParent()

		ctx, cancel := withReportingMargin(parent)
		defer cancel()
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.WithinDuration(t, parentDeadline.Add(-maxReportingMargin), deadline, 10*time.Millisecond)
	})

	t.Run("short deadline uses a tenth of remaining time", func(t *testing.T) {
		parentDeadline := time.Now().Add(10 * time.Second)
		parent, cancelParent := context.WithDeadline(t.Context(), parentDeadline)
		defer cancelParent()

		ctx, cancel := withReportingMargin(parent)
		defer cancel()
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.WithinDuration(t, parentDeadline.Add(-time.Second), deadline, 10*time.Millisecond)
	})

	t.Run("cause explains the early stop", func(t *testing.T) {
		parent, cancelParent := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancelParent()

		ctx, cancel := withReportingMargin(parent)
		defer cancel()
		<-ctx.Done()
		require.ErrorIs(t, context.Cause(ctx), errReportingMargin)
		require.NoError(t, parent.Err(), "parent should still have time left for reporting")
	})
}
