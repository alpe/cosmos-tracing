package tracing

import (
	"errors"
	"testing"
	"time"

	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/opentracing/opentracing-go"
)

func TestDoWithTracing(t *testing.T) {
	specs := map[string]struct {
		cb     func(workCtx types.Context, span opentracing.Span) error
		expect func(t *testing.T, cap *mocktracer.MockTracer)
	}{
		"with error logged": {
			cb: func(workCtx types.Context, span opentracing.Span) error {
				return errors.New("my-error")
			},
			expect: func(t *testing.T, cap *mocktracer.MockTracer) {
				spans := cap.FinishedSpans()
				require.Len(t, spans, 1)
				assert.Equal(t, "true", spans[0].Tags()[tagErrored])

				logs := spans[0].Logs()
				require.GreaterOrEqual(t, len(logs), 1)
				assert.Equal(t, "error.object", logs[0].Fields[0].Key)
				assert.Equal(t, "my-error", logs[0].Fields[0].ValueString)
			},
		},
		"with other calls": {
			cb: func(parentCtx types.Context, span opentracing.Span) error {
				span.SetTag("outer", "true")
				DoWithTracing(parentCtx, "inner", all, func(workCtx types.Context, span opentracing.Span) error {
					span.SetTag("inner", "sure")
					return nil
				})
				return nil
			},
			expect: func(t *testing.T, cap *mocktracer.MockTracer) {
				spans := cap.FinishedSpans()
				require.Len(t, spans, 2)
				assert.Equal(t, "inner", spans[0].OperationName)
				assert.Equal(t, "my-op", spans[1].OperationName)

				assert.Equal(t, "sure", spans[0].Tags()["inner"])
				assert.Equal(t, "true", spans[1].Tags()["outer"])
			},
		},
		"with clock shift": {
			cb: func(workCtx types.Context, span opentracing.Span) error {
				return nil
			},
			expect: func(t *testing.T, cap *mocktracer.MockTracer) {
				spans := cap.FinishedSpans()
				require.Len(t, spans, 1)
				blockTime := time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC)
				require.Equal(t, blockTime, spans[0].StartTime)
				assert.Less(t, spans[0].FinishTime, blockTime.Add(time.Second))
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			tracer := mocktracer.New()
			opentracing.SetGlobalTracer(tracer)
			ctx, _, _ := createMinTestInput(t)
			DoWithTracing(ctx, "my-op", all, spec.cb)
			spec.expect(t, tracer)
		})
	}
}
