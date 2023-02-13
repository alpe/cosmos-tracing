package tracing

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cometbft/cometbft/libs/rand"
	"github.com/cosmos/cosmos-sdk/types/address"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"

	"github.com/opentracing/opentracing-go/mocktracer"

	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnRecvPacket(t *testing.T) {
	tracerEnabled = true
	t.Cleanup(func() { tracerEnabled = false })
	myPkg := channeltypes.Packet{}
	relayerAddr := sdk.AccAddress(rand.Bytes(address.Len))
	specs := map[string]struct {
		srcPkg     channeltypes.Packet
		srcRelayer sdk.AccAddress
		retAck     exported.Acknowledgement
	}{
		"success ack": {
			srcPkg:     myPkg,
			srcRelayer: relayerAddr,
			retAck:     channeltypes.NewResultAcknowledgement(rand.Bytes(128)),
		},
		"relayer nil": {
			srcPkg: myPkg,
			retAck: channeltypes.NewResultAcknowledgement(rand.Bytes(128)),
		},
		"error ack": {
			srcPkg:     myPkg,
			srcRelayer: relayerAddr,
			retAck:     channeltypes.NewErrorAcknowledgement(types.ErrEmpty),
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			tracer := mocktracer.New()
			opentracing.SetGlobalTracer(tracer)
			ctx, _, _ := createMinTestInput(t)

			var capPkg channeltypes.Packet
			var capRelayer sdk.AccAddress
			mock := &MockIBCModule{
				OnRecvPacketFn: func(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) exported.Acknowledgement {
					capPkg = packet
					capRelayer = relayer
					return spec.retAck
				},
			}
			h := NewTraceIBCHandler(mock, "foo")

			// when
			gotAck := h.OnRecvPacket(ctx, spec.srcPkg, spec.srcRelayer)
			// then
			assert.Equal(t, spec.srcRelayer, capRelayer)
			assert.Equal(t, spec.srcPkg, capPkg)
			assert.Equal(t, spec.retAck.Acknowledgement(), gotAck.Acknowledgement())
			assert.Equal(t, spec.retAck.Success(), gotAck.Success())

			// and traced
			spans := tracer.FinishedSpans()
			require.Len(t, spans, 1)
			require.Len(t, spans[0].Tags(), 6)
		})
	}
}

func TestOnAcknowledgementPacket(t *testing.T) {
	tracerEnabled = true
	t.Cleanup(func() { tracerEnabled = false })

	myPkg := channeltypes.Packet{}
	relayerAddr := sdk.AccAddress(rand.Bytes(address.Len))
	specs := map[string]struct {
		srcPkg     channeltypes.Packet
		srcAck     []byte
		srcRelayer sdk.AccAddress
		retErr     error
		expErr     bool
	}{
		"all good": {
			srcPkg:     myPkg,
			srcAck:     []byte("my ack"),
			srcRelayer: relayerAddr,
		},
		"empty relayer": {
			srcPkg: myPkg,
			srcAck: []byte("my ack"),
		},
		"empty ack": {
			srcPkg:     myPkg,
			srcRelayer: relayerAddr,
		},
		"error": {
			srcPkg:     myPkg,
			srcAck:     []byte("my ack"),
			srcRelayer: relayerAddr,
			retErr:     errors.New("testing"),
			expErr:     true,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			tracer := mocktracer.New()
			opentracing.SetGlobalTracer(tracer)
			ctx, _, _ := createMinTestInput(t)

			var capPkg channeltypes.Packet
			var capAck []byte
			var capRelayer sdk.AccAddress
			mock := &MockIBCModule{
				OnAcknowledgementPacketFn: func(ctx sdk.Context, packet channeltypes.Packet, acknowledgement []byte, relayer sdk.AccAddress) error {
					capPkg = packet
					capAck = acknowledgement
					capRelayer = relayer
					return spec.retErr
				},
			}
			h := NewTraceIBCHandler(mock, "foo")
			gotErr := h.OnAcknowledgementPacket(ctx, spec.srcPkg, spec.srcAck, spec.srcRelayer)
			if spec.expErr {
				require.Error(t, gotErr)
				return
			}
			require.NoError(t, gotErr)
			assert.Equal(t, spec.srcRelayer, capRelayer)
			assert.Equal(t, spec.srcPkg, capPkg)
			assert.Equal(t, spec.srcAck, capAck)
			// and traced
			spans := tracer.FinishedSpans()
			require.Len(t, spans, 1)
			require.Len(t, spans[0].Tags(), 6)
		})
	}
}

func TestCaptureAck(t *testing.T) {
	specs := map[string]struct {
		src channeltypes.Acknowledgement
	}{
		"ok": {
			src: channeltypes.NewResultAcknowledgement([]byte("foo")),
		},
		"error": {
			src: channeltypes.NewErrorAcknowledgement(errors.New("bar")),
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			m := newCapturedAck(spec.src)
			assert.Equal(t, spec.src.Acknowledgement(), m.Acknowledgement())
			assert.Equal(t, spec.src.Success(), m.Success())
			assert.Equal(t, spec.src.Success(), !IsAckError(m.Acknowledgement()))
		})
	}
}

// IsAckError checks an IBC acknowledgement to see if it's an error.
// This is a replacement for ack.Success() which is currently not working on some circumstances
func IsAckError(acknowledgement []byte) bool {
	var ackErr channeltypes.Acknowledgement_Error
	if err := json.Unmarshal(acknowledgement, &ackErr); err == nil && len(ackErr.Error) > 0 {
		return true
	}
	return false
}
