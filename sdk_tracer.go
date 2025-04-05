package tracing

import (
	"fmt"
	"strconv"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	cmttypes "github.com/cometbft/cometbft/libs/bytes"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypesv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/gogo/protobuf/proto"
	"github.com/opentracing/opentracing-go"
)

const (
	tagModule         = "module"
	tagErrored        = "errored"
	tagSDKMsgType     = "sdk_message_type"
	tagSDKGRPCService = "sdk_grpc_service"
	tagBlockHeight    = "height"
	tagTXHash         = "tx"
	tagSimulation     = "simulation"
	tagQueryPath      = "query_path"
	tagValsetUpdate   = "valset_update"

	logRawStoreIO   = "raw_store_io"
	logValsetDiff   = "valset_diff"
	logRawLoggerOut = "logger_out"
	logGasUsage     = "gas_usage"
)

// NewTraceAnteHandler decorates the ante handler with tracing functionality
func NewTraceAnteHandler(other sdk.AnteHandler, cdc codec.Codec) sdk.AnteHandler {
	if !tracerEnabled {
		return other
	}
	return func(rootCtx sdk.Context, tx sdk.Tx, simulate bool) (nextCtx sdk.Context, err error) {
		if !isTraceable(rootCtx, simulate) {
			return other(rootCtx, tx, simulate)
		}
		ctx := WithSimulation(rootCtx, simulate)
		DoWithTracing(ctx, "ante_handler", writesOnly, func(workCtx sdk.Context, span opentracing.Span) error {
			msgs := make([]string, len(tx.GetMsgs()))
			senders := make([]string, 0, len(tx.GetMsgs()))
			for i, msg := range tx.GetMsgs() {
				msgs[i] = fmt.Sprintf("%T", msg)
				pm, ok := msg.(proto.Message)
				if !ok {
					continue
				}
				jsonMsg, err := cdc.MarshalJSON(pm)
				if err != nil {
					jsonMsg = []byte(err.Error())
				}
				span.LogFields(safeLogField(logRawSDKMsg, cutLength(string(jsonMsg), MaxSDKMsgTraced)))
				senders = deduplicateStrings(append(senders, findSigners(msg)...))
			}
			span.SetTag(tagTXHash, cmttypes.HexBytes(tmhash.Sum(rootCtx.TxBytes())).String())
			span.SetTag(tagSDKMsgType, deduplicateStrings(msgs))
			span.SetTag(tagSender, senders)
			span.SetTag(tagSimulation, strconv.FormatBool(simulate))

			nextCtx, err = other(workCtx, tx, simulate)
			return err
		})
		return
	}
}

func addrsToString[T []byte | sdk.AccAddress](addrs []T) []string {
	r := make([]string, len(addrs))
	for i, a := range addrs {
		r[i] = sdk.AccAddress(a).String()
	}
	return r
}

func deduplicateStrings(msgs []string) []string {
	r := make([]string, 0, len(msgs))
	idx := make(map[string]struct{}, len(msgs))
	for _, v := range msgs {
		if _, exists := idx[v]; exists {
			continue
		}
		r = append(r, v)
		idx[v] = struct{}{}
	}
	return r
}

var _ govtypesv1beta1.Router = TraceGovRouter{}

// TraceGovRouter is a decorator to the sdk gov router that adds call tracing functionality
type TraceGovRouter struct {
	other govtypesv1beta1.Router
}

func NewTraceGovRouter(other govtypesv1beta1.Router) govtypesv1beta1.Router {
	if !tracerEnabled {
		return other
	}
	return &TraceGovRouter{other: other}
}

func (t TraceGovRouter) AddRoute(r string, h govtypesv1beta1.Handler) (rtr govtypesv1beta1.Router) {
	return NewTraceGovRouter(t.other.AddRoute(r, h))
}

func (t TraceGovRouter) HasRoute(r string) bool {
	return t.other.HasRoute(r)
}

func (t TraceGovRouter) Seal() {
	t.other.Seal()
}

func (t TraceGovRouter) GetRoute(path string) (h govtypesv1beta1.Handler) {
	realHandler := t.other.GetRoute(path)
	if realHandler == nil {
		return nil
	}
	return func(rootCtx sdk.Context, content govtypesv1beta1.Content) (err error) {
		if !IsTraceable(rootCtx) {
			return realHandler(rootCtx, content)
		}
		DoWithTracing(rootCtx, "gov_router", all, func(workCtx sdk.Context, span opentracing.Span) error {
			span.SetTag(tagModule, content.ProposalRoute()).
				SetTag(tagSDKMsgType, fmt.Sprintf("%T", content))
			err = realHandler(workCtx, content)
			return err
		})
		return
	}
}

// MessageRouter ADR 031 request type routing
type MessageRouter interface {
	Handler(msg sdk.Msg) baseapp.MsgServiceHandler
}

var _ MessageRouter = TraceMessageRouter{}

type TraceMessageRouter struct {
	other MessageRouter
}

func NewTraceMessageRouter(other MessageRouter) MessageRouter {
	if !tracerEnabled {
		return other
	}
	return &TraceMessageRouter{other: other}
}

type routeable interface {
	Route() string
}

func (t TraceMessageRouter) Handler(msg sdk.Msg) baseapp.MsgServiceHandler {
	realHandler := t.other.Handler(msg)
	return func(rootCtx sdk.Context, msg sdk.Msg) (result *sdk.Result, err error) {
		if !IsTraceable(rootCtx) {
			return realHandler(rootCtx, msg)
		}
		DoWithTracing(rootCtx, "new_msg_router", all, func(workCtx sdk.Context, span opentracing.Span) error {
			moduleName := "-"
			if m, ok := msg.(routeable); ok {
				moduleName = m.Route()
			}
			span.SetTag(tagModule, moduleName).
				SetTag(tagSDKMsgType, fmt.Sprintf("%T", msg)).
				SetTag(tagSender, findSigners(msg))

			tryLogWasmMsg(span, msg)

			result, err = realHandler(workCtx, msg)
			if err != nil {
				return err
			}
			addTagsFromWasmEvents(span, result.GetEvents())
			logResultData(span, result.Data)
			return nil
		})
		return
	}
}

func findSigners(msg sdk.Msg) []string {
	m, ok := msg.(interface{ GetSigners() ([][]byte, error) })
	if !ok {
		return nil
	}
	signers, err := m.GetSigners()
	if err != nil {
		return nil
	}
	return addrsToString(signers)
}

func logResultData(span opentracing.Span, data []byte) {
	span.LogFields(safeLogField("raw_wasm_message_result", toJSON(data)))
}

func tryLogWasmMsg(span opentracing.Span, msg sdk.Msg) {
	switch m := msg.(type) {
	case *wasmtypes.MsgInstantiateContract:
		span.LogFields(safeLogField(logDecodedWasmMsg, string(m.Msg)))
	case *wasmtypes.MsgInstantiateContract2:
		span.LogFields(safeLogField(logDecodedWasmMsg, string(m.Msg)))
	case *wasmtypes.MsgExecuteContract:
		span.LogFields(safeLogField(logDecodedWasmMsg, string(m.Msg)))
	case *wasmtypes.MsgMigrateContract:
		span.LogFields(safeLogField(logDecodedWasmMsg, string(m.Msg)))
	}
}
