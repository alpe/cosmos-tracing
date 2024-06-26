package tracing

import (
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics"
)

func RunWithTracer(startCmd *cobra.Command, appName string) {
	otherRunE := startCmd.RunE
	var tracer io.Closer
	startCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if hasTracerFlagSet(cmd) {
			tracer = StartTracer(appName)
		}
		return otherRunE(cmd, args)
	}
	otherPostRun := startCmd.PostRun
	startCmd.PostRun = func(cmd *cobra.Command, args []string) {
		if tracer != nil {
			tracer.Close()
		}
		if otherPostRun != nil {
			otherPostRun(cmd, args)
		}
	}
	AddModuleInitFlags(startCmd)
}

// todo: make this configurable
func StartTracer(appName string) io.Closer {
	// Sample configuration for testing. Use constant sampling to sample every trace
	// and enable LogSpan to log every span via configured Logger.
	cfg := config.Configuration{
		ServiceName: appName,
		Sampler: &config.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans:  true,
			QueueSize: 100,
		},
	}

	// Example logger and metrics factory. Use github.com/uber/jaeger-client-go/log
	// and github.com/uber/jaeger-lib/metrics respectively to bind to real logging and metrics
	// frameworks.
	// jLogger := jaegerlog.StdLogger
	jMetricsFactory := metrics.NullFactory
	// Initialize tracer with a logger and a metrics factory
	tracer, closer, err := cfg.NewTracer(
		// jaegercfg.Logger(jLogger),
		config.Metrics(jMetricsFactory),
	)
	if err != nil {
		panic(err.Error())
	}
	// Set the singleton opentracing.Tracer with the Jaeger tracer.
	opentracing.SetGlobalTracer(tracer)
	return closer
}
