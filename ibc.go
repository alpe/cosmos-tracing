package tracing

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

var _ porttypes.IBCModule = TraceIBCHandler{}

// TraceIBCHandler is a decorator to the ibc module handler that adds call tracing functionality
type TraceIBCHandler struct {
	other      porttypes.IBCModule
	moduleName string
}

// NewTraceWasmIBCHandler
// Deprecated: this is for wasm module only
func NewTraceWasmIBCHandler(other porttypes.IBCModule) porttypes.IBCModule {
	return NewTraceIBCHandler(other, wasmtypes.ModuleName)
}

// NewTraceIBCHandler constructor
func NewTraceIBCHandler(other porttypes.IBCModule, moduleName string) porttypes.IBCModule {
	if !tracerEnabled {
		return other
	}
	return &TraceIBCHandler{other: other, moduleName: moduleName}
}

func (t TraceIBCHandler) OnChanOpenInit(
	rootCtx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID string,
	channelID string,
	channelCap *capabilitytypes.Capability,
	counterparty channeltypes.Counterparty,
	version string,
) (v string, err error) {
	if !IsTraceable(rootCtx) {
		return t.other.OnChanOpenInit(rootCtx, order, connectionHops, portID, channelID, channelCap, counterparty, version)
	}
	DoWithTracing(rootCtx, "ibc_chan_open_init", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName)
		v, err = t.other.OnChanOpenInit(workCtx, order, connectionHops, portID, channelID, channelCap, counterparty, version)
		return err
	})
	return
}

func (t TraceIBCHandler) OnChanOpenTry(
	rootCtx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID, channelID string,
	channelCap *capabilitytypes.Capability,
	counterparty channeltypes.Counterparty,
	counterpartyVersion string,
) (version string, err error) {
	if !IsTraceable(rootCtx) {
		return t.other.OnChanOpenTry(rootCtx, order, connectionHops, portID, channelID, channelCap, counterparty, counterpartyVersion)
	}
	DoWithTracing(rootCtx, "ibc_chan_open_try", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName)

		version, err = t.other.OnChanOpenTry(workCtx, order, connectionHops, portID, channelID, channelCap, counterparty, counterpartyVersion)
		if err != nil {
			span.LogFields(safeLogField(logIBCVersion, version))
		}
		return err
	})
	return
}

func (t TraceIBCHandler) OnChanOpenAck(
	rootCtx sdk.Context,
	portID, channelID string,
	counterpartyChannelID string,
	counterpartyVersion string,
) (err error) {
	if !IsTraceable(rootCtx) {
		return t.other.OnChanOpenAck(rootCtx, portID, channelID, counterpartyChannelID, counterpartyVersion)
	}
	DoWithTracing(rootCtx, "ibc_chan_open_ack", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName)
		err = t.other.OnChanOpenAck(workCtx, portID, channelID, counterpartyChannelID, counterpartyVersion)
		return err
	})
	return
}

func (t TraceIBCHandler) OnChanOpenConfirm(rootCtx sdk.Context, portID, channelID string) (err error) {
	if !IsTraceable(rootCtx) {
		return t.other.OnChanOpenConfirm(rootCtx, portID, channelID)
	}
	DoWithTracing(rootCtx, "ibc_chan_open_confirm", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName)
		err = t.other.OnChanOpenConfirm(workCtx, portID, channelID)
		return err
	})
	return
}

func (t TraceIBCHandler) OnChanCloseInit(rootCtx sdk.Context, portID, channelID string) (err error) {
	if !IsTraceable(rootCtx) {
		return t.other.OnChanCloseInit(rootCtx, portID, channelID)
	}

	DoWithTracing(rootCtx, "ibc_chan_close_init", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName)
		err = t.other.OnChanCloseInit(workCtx, portID, channelID)
		return err
	})
	return
}

func (t TraceIBCHandler) OnChanCloseConfirm(rootCtx sdk.Context, portID, channelID string) (err error) {
	if !IsTraceable(rootCtx) {
		return t.other.OnChanCloseConfirm(rootCtx, portID, channelID)
	}
	DoWithTracing(rootCtx, "ibc_chan_close_confirm", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName)
		err = t.other.OnChanCloseConfirm(workCtx, portID, channelID)
		return err
	})
	return
}

var _ exported.Acknowledgement = capturedAck{}

type capturedAck struct {
	success bool
	ack     []byte
}

func newCapturedAck(orig exported.Acknowledgement) capturedAck {
	return capturedAck{
		success: orig.Success(),
		ack:     orig.Acknowledgement(),
	}
}

func (c capturedAck) Success() bool {
	return c.success
}

func (c capturedAck) Acknowledgement() []byte {
	return c.ack
}

func (t TraceIBCHandler) OnRecvPacket(rootCtx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) (result exported.Acknowledgement) {
	if !IsTraceable(rootCtx) {
		return t.other.OnRecvPacket(rootCtx, packet, relayer)
	}
	if os.Getenv("no_tracing_ibcreceive") != "" {
		return t.other.OnRecvPacket(rootCtx, packet, relayer)
	}
	DoWithTracing(rootCtx, "ibc_packet_recv", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName).
			SetTag(tagIBCSrcPort, packet.SourcePort).
			SetTag(tagIBCDestPort, packet.DestinationPort).
			SetTag(tagIBCSrcChannel, packet.SourceChannel).
			SetTag(tagIBCDestChannel, packet.DestinationChannel)

		memo, packetDescr := parsePacket(packet.Data)
		span.LogFields(safeLogField(logIBCPacketMemo, memo))
		if packetDescr != "" {
			span.LogFields(safeLogField(logIBCPacketDescr, cutLength(packetDescr, MaxIBCPacketDescr)))
		}

		orig := t.other.OnRecvPacket(rootCtx, packet, relayer)
		if os.Getenv("no_tracing_ibcreceive_capture") != "" {
			result = orig
		} else {
			result = newCapturedAck(orig)
		}
		span.LogFields(safeLogField(logRawIBCACKType, fmt.Sprintf("%T", orig)))
		if os.Getenv("no_tracing_ibcreceive_log") == "" {
			span.LogFields(safeLogField(logRawIBCACK, hex.EncodeToString(result.Acknowledgement())))
			span.LogFields(log.Bool(logRawIBCACKSuccess, result.Success()))
		}
		span.LogFields(safeLogField(logRawIBCRelayer, relayer.String()))
		return nil
	})
	return
}

func (t TraceIBCHandler) OnAcknowledgementPacket(rootCtx sdk.Context, packet channeltypes.Packet, acknowledgement []byte, relayer sdk.AccAddress) (err error) {
	if !IsTraceable(rootCtx) {
		return t.other.OnAcknowledgementPacket(rootCtx, packet, acknowledgement, relayer)
	}
	DoWithTracing(rootCtx, "ibc_packet_ack", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName).
			SetTag(tagIBCSrcPort, packet.SourcePort).
			SetTag(tagIBCDestPort, packet.DestinationPort).
			SetTag(tagIBCSrcChannel, packet.SourceChannel).
			SetTag(tagIBCDestChannel, packet.DestinationChannel)

		span.LogFields(safeLogField(logRawIBCACK, hex.EncodeToString(acknowledgement)))
		span.LogFields(safeLogField(logRawIBCRelayer, relayer.String()))

		err = t.other.OnAcknowledgementPacket(workCtx, packet, acknowledgement, relayer)
		return err
	})
	return
}

func (t TraceIBCHandler) OnTimeoutPacket(rootCtx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) (err error) {
	if !IsTraceable(rootCtx) {
		return t.other.OnTimeoutPacket(rootCtx, packet, relayer)
	}
	DoWithTracing(rootCtx, "ibc_packet_timeout", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagModule, t.moduleName).
			SetTag(tagIBCSrcPort, packet.SourcePort).
			SetTag(tagIBCDestPort, packet.DestinationPort).
			SetTag(tagIBCSrcChannel, packet.SourceChannel).
			SetTag(tagIBCDestChannel, packet.DestinationChannel)

		err = t.other.OnTimeoutPacket(workCtx, packet, relayer)
		return err
	})
	return
}

func parsePacket(data []byte) (string, string) {
	if json.Valid(data) {
		var p1 transfertypes.FungibleTokenPacketData
		if err := json.Unmarshal(data, &p1); err == nil {
			return p1.Memo, p1.String()
		}
		var p2 icatypes.InterchainAccountPacketData
		if err := json.Unmarshal(data, &p2); err == nil {
			return p2.Memo, p2.String()
		}
		return "unidentified", ""
	} else { // try protobuf
		var p1 transfertypes.FungibleTokenPacketData
		if err := p1.Unmarshal(data); err == nil {
			return p1.Memo, p1.String()
		}
		var p2 icatypes.InterchainAccountPacketData
		if err := p2.Unmarshal(data); err == nil {
			return p2.Memo, p2.String()
		}
		return "unidentified", ""

	}
}
