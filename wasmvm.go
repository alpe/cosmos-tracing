package tracing

import (
	"encoding/hex"
	"fmt"

	"github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	cosmwasm "github.com/CosmWasm/wasmvm"
	wasmvmtypes "github.com/CosmWasm/wasmvm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/opentracing/opentracing-go"
)

var _ wasmtypes.WasmEngine = TraceWasmVm{}

type TraceWasmVm struct {
	other wasmtypes.WasmEngine
}

// NewTraceWasmVm constructor
func NewTraceWasmVm(other wasmtypes.WasmEngine) wasmtypes.WasmEngine {
	if !tracerEnabled {
		return other
	}
	return &TraceWasmVm{other: other}
}

func (t TraceWasmVm) Create(code cosmwasm.WasmCode) (cosmwasm.Checksum, error) {
	return t.other.StoreCode(code)
}

func (t TraceWasmVm) AnalyzeCode(checksum cosmwasm.Checksum) (*wasmvmtypes.AnalysisReport, error) {
	return t.other.AnalyzeCode(checksum)
}

func (t TraceWasmVm) Instantiate(checksum cosmwasm.Checksum, env wasmvmtypes.Env, info wasmvmtypes.MessageInfo, initMsg []byte, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
	return wasmvmDoWithTracing(
		"wasmvm_instantiate",
		querier,
		ContractJsonInputMsgRecorder(initMsg),
		ContractVmResponseRecorder(),
		func() (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
			return t.other.Instantiate(checksum, env, info, initMsg, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) Execute(checksum cosmwasm.Checksum, env wasmvmtypes.Env, info wasmvmtypes.MessageInfo, executeMsg []byte, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
	return wasmvmDoWithTracing(
		"wasmvm_execute",
		querier,
		ContractJsonInputMsgRecorder(executeMsg),
		ContractVmResponseRecorder(),
		func() (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
			return t.other.Execute(checksum, env, info, executeMsg, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) Query(checksum cosmwasm.Checksum, env wasmvmtypes.Env, queryMsg []byte, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (resp []byte, gasUsed uint64, err error) {
	rootCtx := fetchCtx(querier)
	DoWithTracing(rootCtx, "wasmvm_query", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.LogFields(safeLogField(logRawQueryData, string(queryMsg)))
		resp, gasUsed, err = t.other.Query(checksum, env, queryMsg, store, goapi, querier, gasMeter, gasLimit, deserCost)
		if err == nil && resp != nil {
			span.LogFields(safeLogField(logRawQueryResult, string(resp)))
		}
		return err
	})
	return
}

// fetch current sdk context, hack
func fetchCtx(querier cosmwasm.Querier) sdk.Context {
	switch q := querier.(type) {
	case *keeper.QueryHandler:
		return q.Ctx
	case keeper.QueryHandler:
		return q.Ctx
	default:
		fmt.Printf("+++++: UNSUPPORTED QUERY HANDLER: %T\n", querier)
		return sdk.Context{}
	}
}

func (t TraceWasmVm) Migrate(checksum cosmwasm.Checksum, env wasmvmtypes.Env, migrateMsg []byte, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
	return wasmvmDoWithTracing(
		"wasmvm_migrate",
		querier,
		ContractJsonInputMsgRecorder(migrateMsg),
		ContractVmResponseRecorder(),
		func() (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
			return t.other.Migrate(checksum, env, migrateMsg, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) Sudo(checksum cosmwasm.Checksum, env wasmvmtypes.Env, sudoMsg []byte, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
	return wasmvmDoWithTracing(
		"wasmvm_sudo",
		querier,
		ContractJsonInputMsgRecorder(sudoMsg),
		ContractVmResponseRecorder(),
		func() (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
			return t.other.Sudo(checksum, env, sudoMsg, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) Reply(checksum cosmwasm.Checksum, env wasmvmtypes.Env, reply wasmvmtypes.Reply, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
	return wasmvmDoWithTracing(
		"wasmvm_reply",
		querier,
		ContractGenericInputMsgRecorder(reply),
		ContractVmResponseRecorder(),
		func() (resp *wasmvmtypes.Response, gasUsed uint64, err error) {
			return t.other.Reply(checksum, env, reply, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) GetCode(checksum cosmwasm.Checksum) (cosmwasm.WasmCode, error) {
	return t.other.GetCode(checksum)
}

func (t TraceWasmVm) Cleanup() {
	t.other.Cleanup()
}

func (t TraceWasmVm) IBCChannelOpen(checksum cosmwasm.Checksum, env wasmvmtypes.Env, channel wasmvmtypes.IBCChannelOpenMsg, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (*wasmvmtypes.IBC3ChannelOpenResponse, uint64, error) {
	return wasmvmDoWithTracing(
		"wasmvm_chan_open",
		querier,
		ContractGenericInputMsgRecorder(channel),
		ContractIBCChannelOpenResponseRecorder(),
		func() (resp *wasmvmtypes.IBC3ChannelOpenResponse, gasUsed uint64, err error) {
			return t.other.IBCChannelOpen(checksum, env, channel, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) IBCChannelConnect(checksum cosmwasm.Checksum, env wasmvmtypes.Env, channel wasmvmtypes.IBCChannelConnectMsg, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (*wasmvmtypes.IBCBasicResponse, uint64, error) {
	return wasmvmDoWithTracing(
		"wasmvm_chan_connect",
		querier,
		ContractGenericInputMsgRecorder(channel),
		ContractGenericResponseRecorder[wasmvmtypes.IBCBasicResponse](),
		func() (resp *wasmvmtypes.IBCBasicResponse, gasUsed uint64, err error) {
			return t.other.IBCChannelConnect(checksum, env, channel, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) IBCChannelClose(checksum cosmwasm.Checksum, env wasmvmtypes.Env, channel wasmvmtypes.IBCChannelCloseMsg, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (*wasmvmtypes.IBCBasicResponse, uint64, error) {
	return wasmvmDoWithTracing(
		"wasmvm_chan_close",
		querier,
		ContractGenericInputMsgRecorder(channel),
		ContractGenericResponseRecorder[wasmvmtypes.IBCBasicResponse](),
		func() (resp *wasmvmtypes.IBCBasicResponse, gasUsed uint64, err error) {
			return t.other.IBCChannelClose(checksum, env, channel, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) IBCPacketReceive(checksum cosmwasm.Checksum, env wasmvmtypes.Env, packet wasmvmtypes.IBCPacketReceiveMsg, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (*wasmvmtypes.IBCReceiveResult, uint64, error) {
	return wasmvmDoWithTracing(
		"wasmvm_pkg_recv",
		querier,
		ContractGenericInputMsgRecorder(packet),
		ContractGenericResponseRecorder[wasmvmtypes.IBCReceiveResult](),
		func() (resp *wasmvmtypes.IBCReceiveResult, gasUsed uint64, err error) {
			return t.other.IBCPacketReceive(checksum, env, packet, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) IBCPacketAck(checksum cosmwasm.Checksum, env wasmvmtypes.Env, ack wasmvmtypes.IBCPacketAckMsg, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (*wasmvmtypes.IBCBasicResponse, uint64, error) {
	return wasmvmDoWithTracing(
		"wasmvm_pkg_ack",
		querier,
		ContractGenericInputMsgRecorder(ack),
		ContractGenericResponseRecorder[wasmvmtypes.IBCBasicResponse](),
		func() (*wasmvmtypes.IBCBasicResponse, uint64, error) {
			return t.other.IBCPacketAck(checksum, env, ack, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) IBCPacketTimeout(checksum cosmwasm.Checksum, env wasmvmtypes.Env, packet wasmvmtypes.IBCPacketTimeoutMsg, store cosmwasm.KVStore, goapi cosmwasm.GoAPI, querier cosmwasm.Querier, gasMeter cosmwasm.GasMeter, gasLimit uint64, deserCost wasmvmtypes.UFraction) (*wasmvmtypes.IBCBasicResponse, uint64, error) {
	return wasmvmDoWithTracing(
		"wasmvm_pkg_timeout",
		querier,
		ContractGenericInputMsgRecorder(packet),
		ContractGenericResponseRecorder[wasmvmtypes.IBCBasicResponse](),
		func() (resp *wasmvmtypes.IBCBasicResponse, gasUsed uint64, err error) {
			return t.other.IBCPacketTimeout(checksum, env, packet, store, goapi, querier, gasMeter, gasLimit, deserCost)
		},
	)
}

func (t TraceWasmVm) Pin(checksum cosmwasm.Checksum) error {
	return t.other.Pin(checksum)
}

func (t TraceWasmVm) Unpin(checksum cosmwasm.Checksum) error {
	return t.other.Unpin(checksum)
}

func (t TraceWasmVm) GetMetrics() (*wasmvmtypes.Metrics, error) {
	return t.other.GetMetrics()
}

func (t TraceWasmVm) StoreCode(code cosmwasm.WasmCode) (cosmwasm.Checksum, error) {
	return t.other.StoreCode(code)
}

func (t TraceWasmVm) StoreCodeUnchecked(code cosmwasm.WasmCode) (cosmwasm.Checksum, error) {
	return t.other.StoreCodeUnchecked(code)
}

func ContractJsonInputMsgRecorder(msg []byte) func(span opentracing.Span) {
	return func(span opentracing.Span) {
		span.LogFields(safeLogField(logRawContractMsg, string(msg)))
	}
}

func ContractGenericInputMsgRecorder(obj interface{}) func(opentracing.Span) {
	return func(span opentracing.Span) {
		span.LogFields(safeLogField(logRawContractMsg, toJson(obj)))
	}
}

func ContractVmResponseRecorder() func(opentracing.Span, *wasmvmtypes.Response) {
	return func(span opentracing.Span, resp *wasmvmtypes.Response) {
		span.LogFields(safeLogField(logRawResponseMsg, toJson(resp)))
		for i, v := range resp.Messages {
			span.LogFields(safeLogField(fmt.Sprintf("%s_%d", logRawSubMsg, i), toJson(v)))
		}
		span.LogFields(safeLogField(logRawResponseData, hex.EncodeToString(resp.Data)))
		span.LogFields(safeLogField(logRawResponseEvents, toJson(resp.Events)))
	}
}

func ContractIBCChannelOpenResponseRecorder() func(opentracing.Span, *wasmvmtypes.IBC3ChannelOpenResponse) {
	return func(span opentracing.Span, resp *wasmvmtypes.IBC3ChannelOpenResponse) {
		span.LogFields(safeLogField(logIBCVersion, resp.Version))
	}
}

// ContractGenericResponseRecorder logs response as json which gives a much better output than log.Object
func ContractGenericResponseRecorder[T any]() func(opentracing.Span, *T) {
	return func(span opentracing.Span, resp *T) {
		span.LogFields(safeLogField(logRawResponseMsg, toJson(resp)))
	}
}

// generic helper function to execute the callback in a tracing context
// customized input/response tracers are used to log type specific data
func wasmvmDoWithTracing[T wasmvmtypes.Response | wasmvmtypes.IBCBasicResponse | wasmvmtypes.IBC3ChannelOpenResponse | wasmvmtypes.IBCReceiveResult](
	name string,
	querier cosmwasm.Querier,
	inputTracer func(opentracing.Span),
	responseTracer func(opentracing.Span, *T),
	cb func() (*T, uint64, error),
) (resp *T, gasUsed uint64, err error) {
	rootCtx := fetchCtx(querier)
	if rootCtx.IsZero() {
		fmt.Println("+++++ not tracing wasmvm call due to missing root context")
		return cb()
	}
	DoWithTracing(rootCtx, name, all, func(workCtx sdk.Context, span opentracing.Span) error {
		inputTracer(span)
		resp, gasUsed, err = cb()
		if err == nil && resp != nil {
			responseTracer(span, resp)
		}
		return err
	})
	return
}
