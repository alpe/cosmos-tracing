package tracing

import (
	"fmt"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
)

// Module init related flags
const (
	flagOpenTracingEnabled        = "twasm.open-tracing"
	flagSimulationTracingDisabled = "twasm.disable-simulation-trace"
)

var (
	tracerEnabled      bool
	disableSimulations bool
)

// AddModuleInitFlags implements servertypes.ModuleInitFlags interface.
func AddModuleInitFlags(startCmd *cobra.Command) {
	startCmd.Flags().Bool(flagOpenTracingEnabled, false, "Enable opentracing agent")
	startCmd.Flags().Bool(flagSimulationTracingDisabled, false, "Do not trace simulations")
}

// ReadTracerConfig reads the tracer flag
func ReadTracerConfig(opts servertypes.AppOptions) error {
	if v := opts.Get(flagOpenTracingEnabled); v != nil {
		var err error
		if tracerEnabled, err = cast.ToBoolE(v); err != nil {
			return err
		}
	}
	if v := opts.Get(flagSimulationTracingDisabled); v != nil {
		var err error
		if disableSimulations, err = cast.ToBoolE(v); err != nil {
			return err
		}
	}
	fmt.Printf("----> Running with tracer: %v (ignore simulations: %v)\n", tracerEnabled, disableSimulations)
	return nil
}

func hasTracerFlagSet(cmd *cobra.Command) bool {
	ok, err := cmd.Flags().GetBool(flagOpenTracingEnabled)
	return err == nil && ok
}
