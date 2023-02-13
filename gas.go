package tracing

import (
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type GasTrace struct {
	Gas        storetypes.Gas
	Descriptor string
	Refund     bool
	Offset     storetypes.Gas
}

// NewGasTrace constructor
func NewGasTrace(amount storetypes.Gas, descriptor string, refund bool, consumed storetypes.Gas) GasTrace {
	return GasTrace{Gas: amount, Descriptor: descriptor, Refund: refund, Offset: consumed}
}

func (g GasTrace) String() string {
	return fmt.Sprintf("%d, %s, refund: %v", g.Gas, g.Descriptor, g.Refund)
}

var _ sdk.GasMeter = &TraceGasMeter{}

// TraceGasMeter is a decorator to the sdk GasMeter that catuptures all gas usage and refunds
type TraceGasMeter struct {
	o      sdk.GasMeter
	traces []GasTrace
}

// NewTraceGasMeter constructor
func NewTraceGasMeter(o sdk.GasMeter) *TraceGasMeter {
	return &TraceGasMeter{o: o, traces: make([]GasTrace, 0)}
}

func (t TraceGasMeter) GasConsumed() storetypes.Gas {
	return t.o.GasConsumed()
}

func (t TraceGasMeter) GasConsumedToLimit() storetypes.Gas {
	return t.o.GasConsumedToLimit()
}

func (t TraceGasMeter) Limit() storetypes.Gas {
	return t.o.Limit()
}

func (t *TraceGasMeter) ConsumeGas(amount storetypes.Gas, descriptor string) {
	t.traces = append(t.traces, NewGasTrace(amount, descriptor, false, t.o.GasConsumed()))
	t.o.ConsumeGas(amount, descriptor)
}

func (t *TraceGasMeter) RefundGas(amount storetypes.Gas, descriptor string) {
	t.traces = append(t.traces, NewGasTrace(amount, descriptor, true, t.o.GasConsumed()))
	t.o.RefundGas(amount, descriptor)
}

func (t TraceGasMeter) IsPastLimit() bool {
	return t.o.IsPastLimit()
}

func (t TraceGasMeter) IsOutOfGas() bool {
	return t.o.IsOutOfGas()
}

func (t TraceGasMeter) String() string {
	return t.o.String()
}

func (t TraceGasMeter) GasRemaining() storetypes.Gas {
	return t.o.GasRemaining()
}
