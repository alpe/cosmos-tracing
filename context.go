package tracing

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// IsTraceable returns true when context is applicable for tracing
func IsTraceable(ctx sdk.Context) bool {
	return isTraceable(ctx, IsSimulation(ctx))
}

// IsTraceable returns true when context is applicable for tracing
func isTraceable(ctx sdk.Context, simulate bool) bool {
	return !ctx.IsCheckTx() || simulate && !disableSimulations
}

type key int

var (
	simulationKey key = 1
	clockKey      key = 2
)

// WithSimulation set simulation flag
func WithSimulation(rootCtx sdk.Context, sim bool) sdk.Context {
	return rootCtx.WithValue(simulationKey, sim)
}

// IsSimulation is simulation context
func IsSimulation(ctx sdk.Context) bool {
	v, ok := ctx.Value(simulationKey).(bool)
	return ok && v
}

type BlockTimeClock struct {
	startTime time.Time
	blockTime time.Time
}

// NewBlockTimeClock constructor
func NewBlockTimeClock(currentSystemTime time.Time, blockTime time.Time) *BlockTimeClock {
	return &BlockTimeClock{startTime: currentSystemTime.UTC(), blockTime: blockTime.UTC()}
}

// Now returns the relative block time
func (b BlockTimeClock) Now(currentSystemTime time.Time) time.Time {
	passed := currentSystemTime.UTC().Sub(b.startTime)
	return b.blockTime.Add(passed).UTC()
}

// WithBlockTimeClock return context with block time clock set and current relative block time
func WithBlockTimeClock(rootCtx sdk.Context) (sdk.Context, time.Time) {
	if c, ok := rootCtx.Value(clockKey).(*BlockTimeClock); ok {
		return rootCtx, c.Now(time.Now())
	}
	blockTime := rootCtx.BlockTime().UTC()
	clock := NewBlockTimeClock(time.Now(), blockTime)
	return rootCtx.WithValue(clockKey, clock), blockTime
}
