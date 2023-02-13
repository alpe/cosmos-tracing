package tracing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
)

func TestIsTraceable(t *testing.T) {
	t.Cleanup(func() {
		disableSimulations = false
	})
	specs := map[string]struct {
		sim, check, disabled bool
		exp                  bool
	}{
		"deliver": {
			exp: true,
		},
		"check": {
			check: true,
			exp:   false,
		},
		"sim": {
			sim:   true,
			check: true,
			exp:   true,
		},
		"sim - disabled": {
			disabled: true,
			sim:      true,
			check:    true,
			exp:      false,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			disableSimulations = spec.disabled
			rootCtx := sdk.NewContext(nil, cmtproto.Header{}, spec.check, nil)
			ctx := WithSimulation(rootCtx, spec.sim)
			got := IsTraceable(ctx)
			assert.Equal(t, spec.exp, got)
		})
	}
}

func TestBlockTimeClockContext(t *testing.T) {
	systemTime := time.Now().UTC()
	sdkCtx := sdk.Context{}.WithBlockTime(systemTime).WithContext(context.Background())
	// initial context
	startCtx, startTime := WithBlockTimeClock(sdkCtx)
	require.Equal(t, systemTime, startTime)
	// other context has clock already
	time.Sleep(time.Millisecond)
	gotCtx, gotTime := WithBlockTimeClock(startCtx)
	assert.Equal(t, startCtx, gotCtx)
	assert.Greater(t, gotTime, startTime.Add(time.Millisecond))
}

func TestBlockTimeClock(t *testing.T) {
	blockTime := time.Unix(0, 1257894000000000000).UTC()
	currentTime := time.Now().UTC()
	clock := NewBlockTimeClock(currentTime, blockTime)

	specs := map[string]struct {
		delta time.Duration
	}{
		"ns": {
			delta: time.Nanosecond,
		},
		"ms": {
			delta: time.Millisecond,
		},
		"s": {
			delta: time.Second,
		},
		"random": {
			delta: time.Duration(time.Now().UnixNano() % 10000000000000000),
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			gotTime := clock.Now(currentTime.Add(spec.delta))
			assert.Equal(t, blockTime.Add(spec.delta), gotTime)
		})
	}
}
