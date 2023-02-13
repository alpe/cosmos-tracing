package tracing

import (
	"bytes"
	"time"

	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/opentracing/opentracing-go"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"
)

type storeLogSetting int

const (
	all storeLogSetting = iota
	writesOnly
	nothing
)

// Exec callback to be executed within a tracing contexts. returned error is for tracking only and not causing any side effects
type Exec func(workCtx sdk.Context, span opentracing.Span) error

// DoWithTracing execute callback in tracing context
func DoWithTracing(ctx sdk.Context, operationName string, logStore storeLogSetting, cb Exec) {
	DoWithTracingAsync(ctx, operationName, logStore, cb)()
}

func DoWithTracingAsync(ctx sdk.Context, operationName string, logStore storeLogSetting, cb Exec) func() {
	var now time.Time
	ctx, now = WithBlockTimeClock(ctx)
	span, goCtx := opentracing.StartSpanFromContext(ctx.Context(), operationName, opentracing.StartTime(now))

	span.SetTag(tagBlockHeight, ctx.BlockHeight())

	ms := NewTracingMultiStore(ctx.MultiStore(), logStore == writesOnly)
	if logStore != nothing {
		ctx = ctx.WithMultiStore(ms)
	}
	em := sdk.NewEventManager()
	var buf bytes.Buffer

	logger := NewMultiLogger(ctx.Logger(), log.NewLogger(&buf, log.LevelOption(zerolog.DebugLevel)))
	gm := NewTraceGasMeter(ctx.GasMeter())
	if err := cb(ctx.WithContext(goCtx).WithEventManager(em).WithLogger(logger).WithGasMeter(gm), span); err != nil {
		span.LogFields(otlog.Error(err))
		span.SetTag(tagErrored, "true")
	}

	if logStore != nothing {
		span.LogFields(safeLogField(logRawStoreIO, ms.getStoreDataLimited(MaxStoreTraced)))
	}
	span.LogFields(safeLogField(logRawEvents, toJson(em.Events())))
	span.LogFields(safeLogField(logRawLoggerOut, cutLength(buf.String(), MaxSDKLogTraced)))

	gasUsage := struct {
		Application []GasTrace
		Storage     []GasTrace
	}{gm.traces, ms.traceGasMeter.traces}
	span.LogFields(safeLogField(logGasUsage, toJson(gasUsage)))

	ctx.EventManager().EmitEvents(em.Events())
	return func() {
		_, now := WithBlockTimeClock(ctx)
		span.FinishWithOptions(opentracing.FinishOptions{
			FinishTime: now,
		})
	}
}

type multiLogger struct {
	loggers []log.Logger
}

func NewMultiLogger(loggers ...log.Logger) log.Logger {
	return &multiLogger{loggers: loggers}
}

func (m multiLogger) Info(msg string, keyVals ...any) {
	for _, l := range m.loggers {
		l.Info(msg, keyVals)
	}
}

func (m multiLogger) Warn(msg string, keyVals ...any) {
	for _, l := range m.loggers {
		l.Warn(msg, keyVals)
	}
}

func (m multiLogger) Error(msg string, keyVals ...any) {
	for _, l := range m.loggers {
		l.Error(msg, keyVals)
	}
}

func (m multiLogger) Debug(msg string, keyVals ...any) {
	for _, l := range m.loggers {
		l.Debug(msg, keyVals)
	}
}

func (m multiLogger) With(keyVals ...any) log.Logger {
	nl := make([]log.Logger, len(m.loggers))
	for i, l := range m.loggers {
		nl[i] = l.With(keyVals...)
	}
	return NewMultiLogger(nl...)
}

func (m multiLogger) Impl() any {
	return m.loggers[0].Impl()
}

func safeLogField(key string, descr string) otlog.Field {
	return otlog.String(key, cutLength(descr, DefaultMaxLength))
}
