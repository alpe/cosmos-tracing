package tracing

import (
	"encoding/json"
	"fmt"
	"strings"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmvmtypes "github.com/CosmWasm/wasmvm/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/opentracing/opentracing-go"
)

const (
	tagSender = "sender"
	// todo: do we need this with sender, now?
	tagContract          = "contract"
	tagSenderContract    = "sender_contract"
	tagCodeID            = "code_id"
	tagWasmMsgCategory   = "wasm_message_category"
	tagWasmMsgType       = "wasm_message_type"
	tagWasmQueryCategory = "wasm_query_category"
	tagWasmQueryType     = "wasm_query_type"
	tagIBCDestPort       = "ibc_dest_port"
	tagIBCSrcPort        = "ibc_src_port"
	tagIBCSrcChannel     = "ibc_src_channel"
	tagIBCDestChannel    = "ibc_dest_channel"

	logRawSDKMsg = "raw_sdk_message"
	// message send from a contract: wasmvm mesasge
	logRawWasmMsg        = "raw_wasm_message"
	logRawSubMsg         = "raw_sub_message"
	logDecodedWasmMsg    = "decoded_wasm_msg"
	logRawContractMsg    = "raw_contract_msg"
	logRawResponseMsg    = "raw_response_msg"
	logRawResponseData   = "raw_response_data_json"
	logRawResponseEvents = "raw_response_events"

	// query object
	logRawWasmQuery = "raw_wasm_query"
	// json payload
	logDecodedWasmQuery   = "decoded_wasm_query"
	logRawWasmQueryResult = "raw_wasm_query_result"
	logRawQueryResult     = "raw_query_result"
	logRawQueryData       = "raw_query_data"
	logRawEvents          = "raw_events"
	logIBCVersion         = "ibc_negotiated_version"

	logRawIBCACK        = "ibc_ack"
	logRawIBCACKType    = "ibc_ack_type"
	logRawIBCACKSuccess = "ibc_ack_success"
	logRawIBCRelayer    = "ibc_relayer"
	logIBCPacketMemo    = "ibc_packet_memo"
	logIBCPacketDescr   = "ibc_packet_description"
	logRawABCIVersion   = "ibc_version_negotiated"
)

var _ wasmkeeper.Messenger = TraceMessageHandler{}

// TraceMessageHandler is a decorator to another message handler adds call tracing functionality
type TraceMessageHandler struct {
	other wasmkeeper.Messenger
	cdc   codec.Codec
}

// TraceMessageHandlerDecorator wasm keeper option to decorate the messenger for tracing
func TraceMessageHandlerDecorator(cdc codec.Codec) func(other wasmkeeper.Messenger) wasmkeeper.Messenger {
	return func(other wasmkeeper.Messenger) wasmkeeper.Messenger {
		if !tracerEnabled {
			return other
		}
		return &TraceMessageHandler{other: other, cdc: cdc}
	}
}

func (h TraceMessageHandler) DispatchMsg(rootCtx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg wasmvmtypes.CosmosMsg) (events []sdk.Event, data [][]byte, err error) {
	if !IsTraceable(rootCtx) {
		return h.other.DispatchMsg(rootCtx, contractAddr, contractIBCPortID, msg)
	}

	DoWithTracing(rootCtx, "messenger", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagSenderContract, contractAddr.String())
		addTagsFromWasmContractMsg(span, msg)
		events, data, err = h.other.DispatchMsg(workCtx, contractAddr, contractIBCPortID, msg)
		addTagsFromWasmEvents(span, events)
		return err
	})
	return
}

var _ wasmkeeper.WasmVMQueryHandler = TraceQueryPlugin{}

// TraceQueryPlugin is a decorator to a WASMVMQueryHandler that adds tracing functionality
type TraceQueryPlugin struct {
	other wasmkeeper.WasmVMQueryHandler
}

// TraceQueryDecorator wasm keeper option to decorate the query handler for tracing
func TraceQueryDecorator(other wasmkeeper.WasmVMQueryHandler) wasmkeeper.WasmVMQueryHandler {
	if !tracerEnabled {
		return other
	}
	return &TraceQueryPlugin{other: other}
}

func (t TraceQueryPlugin) HandleQuery(rootCtx sdk.Context, caller sdk.AccAddress, request wasmvmtypes.QueryRequest) (result []byte, err error) {
	if !IsTraceable(rootCtx) { // track only internal queries
		return t.other.HandleQuery(rootCtx, caller, request)
	}
	DoWithTracing(rootCtx, "wasm_query", all, func(workCtx sdk.Context, span opentracing.Span) error {
		span.SetTag(tagSenderContract, caller.String())
		addTagsFromWasmQuery(span, request)
		result, err = t.other.HandleQuery(workCtx, caller, request)
		span.LogFields(safeLogField(logRawWasmQueryResult, string(result)))
		return err
	})
	return
}

func addTagsFromWasmQuery(span opentracing.Span, req wasmvmtypes.QueryRequest) {
	span.LogFields(safeLogField(logRawWasmQuery, toJson(req)))

	switch {
	case req.Bank != nil:
		span.SetTag(tagWasmQueryCategory, fmt.Sprintf("%T", req.Bank))
		switch {
		case req.Bank.Balance != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Bank.Balance))
		case req.Bank.AllBalances != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Bank.AllBalances))
		default:
			span.SetTag(tagWasmQueryType, "unknown")
		}
	case req.Custom != nil:
		span.SetTag(tagWasmQueryCategory, "custom")
		span.LogFields(safeLogField(logDecodedWasmQuery, string(req.Custom)))
	case req.IBC != nil:
		span.SetTag(tagWasmQueryCategory, fmt.Sprintf("%T", req.IBC))
		switch {
		case req.IBC.PortID != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.IBC.PortID))
		case req.IBC.ListChannels != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.IBC.ListChannels))
		case req.IBC.Channel != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.IBC.Channel))
		default:
			span.SetTag(tagWasmQueryType, "unknown")
		}
	case req.Staking != nil:
		span.SetTag(tagWasmQueryCategory, fmt.Sprintf("%T", req.Staking))
		switch {
		case req.Staking.Validator != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Staking.Validator))
		case req.Staking.AllValidators != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Staking.AllValidators))
		case req.Staking.AllDelegations != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Staking.AllDelegations))
		case req.Staking.BondedDenom != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Staking.BondedDenom))
		case req.Staking.Delegation != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Staking.Delegation))
		default:
			span.SetTag(tagWasmQueryType, "unknown")
		}
	case req.Stargate != nil:
		span.SetTag(tagWasmQueryCategory, fmt.Sprintf("%T", req.Stargate))
		span.SetTag(tagWasmQueryType, req.Stargate.Path)
	case req.Wasm != nil:
		span.SetTag(tagWasmQueryCategory, fmt.Sprintf("%T", req.Wasm))
		switch {
		case req.Wasm.Smart != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Wasm.Smart))
			span.LogFields(safeLogField(logDecodedWasmQuery, string(req.Wasm.Smart.Msg)))
		case req.Wasm.Raw != nil:
			span.SetTag(tagWasmQueryType, fmt.Sprintf("%T", req.Wasm.Raw))
		case req.Staking != nil:
			span.SetTag(tagWasmQueryCategory, fmt.Sprintf("%T", req.Staking))
			switch {
			case req.Staking.AllValidators != nil:
			}
		default:
			span.SetTag(tagWasmQueryType, "unknown")
		}
	}
}

func addTagsFromWasmContractMsg(span opentracing.Span, msg wasmvmtypes.CosmosMsg) {
	span.LogFields(safeLogField(logRawWasmMsg, toJson(msg)))
	switch {
	case msg.Bank != nil:
		span.SetTag(tagWasmMsgCategory, fmt.Sprintf("%T", msg.Bank))
		switch {
		case msg.Bank.Burn != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Bank.Burn))
		case msg.Bank.Send != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Bank.Send))
		default:
			span.SetTag(tagWasmMsgType, "unknown")
		}
	case msg.Custom != nil:
		span.SetTag(tagWasmMsgCategory, "custom")
		// todo: extend for custom bindings

	case msg.Distribution != nil:
		span.SetTag(tagWasmMsgCategory, "distribution")
		switch {
		case msg.Distribution.SetWithdrawAddress != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Distribution.SetWithdrawAddress))
		case msg.Distribution.WithdrawDelegatorReward != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Distribution.WithdrawDelegatorReward))
		default:
			span.SetTag(tagWasmMsgType, "unknown")
		}
	case msg.IBC != nil:
		span.SetTag(tagWasmMsgCategory, fmt.Sprintf("%T", msg.IBC))
		switch {
		case msg.IBC.SendPacket != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.IBC.SendPacket))
		case msg.IBC.Transfer != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.IBC.Transfer))
		case msg.IBC.CloseChannel != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.IBC.CloseChannel))
		default:
			span.SetTag(tagWasmMsgType, "unknown")
		}
	case msg.Staking != nil:
		span.SetTag(tagWasmMsgCategory, fmt.Sprintf("%T", msg.Staking))
		switch {
		case msg.Staking.Delegate != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Staking.Delegate))
		case msg.Staking.Undelegate != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Staking.Undelegate))
		case msg.Staking.Redelegate != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Staking.Redelegate))
		default:
			span.SetTag(tagWasmMsgType, "unknown")
		}
	case msg.Stargate != nil:
		span.SetTag(tagWasmMsgCategory, fmt.Sprintf("%T", msg.Stargate))
		span.SetTag(tagWasmMsgType, msg.Stargate.TypeURL)
	case msg.Wasm != nil:
		span.SetTag(tagWasmMsgCategory, fmt.Sprintf("%T", msg.Wasm))
		switch {
		case msg.Wasm.Migrate != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Wasm.Migrate))
			span.LogFields(safeLogField(logDecodedWasmMsg, string(msg.Wasm.Migrate.Msg)))
		case msg.Wasm.Execute != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Wasm.Execute))
			span.LogFields(safeLogField(logDecodedWasmMsg, string(msg.Wasm.Execute.Msg)))
		case msg.Wasm.Instantiate != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Wasm.Instantiate))
			span.LogFields(safeLogField(logDecodedWasmMsg, string(msg.Wasm.Instantiate.Msg)))
		// case msg.Wasm.Instantiate2 != nil: // todo: add with wasmvm 1.2
		case msg.Wasm.UpdateAdmin != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Wasm.UpdateAdmin))
		case msg.Wasm.ClearAdmin != nil:
			span.SetTag(tagWasmMsgType, fmt.Sprintf("%T", msg.Wasm.ClearAdmin))
		default:
			span.SetTag(tagWasmMsgType, "unknown")
		}
	}
}

func addTagsFromWasmEvents(span opentracing.Span, events sdk.Events) {
	for _, e := range events {
		if e.Type == wasmtypes.WasmModuleEventType ||
			strings.HasPrefix(e.Type, wasmtypes.CustomContractEventPrefix) ||
			e.Type == sdk.EventTypeMessage {
			for _, a := range e.Attributes {
				if string(a.Key) == wasmtypes.AttributeKeyContractAddr {
					span.SetTag(tagContract, string(a.Value))
				}
				if string(a.Key) == wasmtypes.AttributeKeyCodeID {
					span.SetTag(tagCodeID, string(a.Value))
				}
			}
		}
	}
}

// use with care as this can cause trouble with proto Any types
func toJson(o any) string {
	if o == nil {
		return "<nil>"
	}
	bz, err := json.Marshal(o)
	if err != nil {
		return fmt.Sprintf("marshal error: %s", err)
	}
	return string(bz)
}
