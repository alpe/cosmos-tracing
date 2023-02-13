package tracing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"cosmossdk.io/core/appmodule"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmodule "github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/gogoproto/grpc"
	"github.com/cosmos/gogoproto/proto"
	"github.com/opentracing/opentracing-go"
	stdgrpc "google.golang.org/grpc"
)

// ModuleManager abstract module manager type. Should ideally exists in the SDK
type ModuleManager interface {
	SetOrderInitGenesis(moduleNames ...string)
	SetOrderExportGenesis(moduleNames ...string)
	SetOrderBeginBlockers(moduleNames ...string)
	SetOrderEndBlockers(moduleNames ...string)
	SetOrderMigrations(moduleNames ...string)
	RegisterInvariants(ir sdk.InvariantRegistry)
	RegisterServices(cfg sdkmodule.Configurator) error
	InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, genesisData map[string]json.RawMessage) (*abci.ResponseInitChain, error)
	ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) (map[string]json.RawMessage, error)
	RunMigrations(ctx sdk.Context, cfg sdkmodule.Configurator, fromVM sdkmodule.VersionMap) (sdkmodule.VersionMap, error)
	BeginBlock(ctx sdk.Context) (sdk.BeginBlock, error)
	EndBlock(ctx sdk.Context) (sdk.EndBlock, error)
	Precommit(ctx sdk.Context) error
	PrepareCheckState(ctx sdk.Context) error
	GetVersionMap() sdkmodule.VersionMap
	ModuleNames() []string
	GetModules() map[string]any
	GetConsensusVersion(module string) uint64
}

const (
	BeginBlockOperationName        = "begin_block"
	ABCIEndBlockOperationName      = "abci_end_block"
	PrepareCheckStateOperationName = "prepare_check_state"
	PrecommitOperationName         = "precommit"
	ModuleBeginBlockOperationName  = "module_begin_block"
	ModuleEndBlockOperationName    = "module_end_block"
)

var _ ModuleManager = &TraceModuleManager{}

type TraceModuleManager struct {
	*sdkmodule.Manager
	cdc codec.Codec
}

// NewTraceModuleManager constructor
func NewTraceModuleManager(other *sdkmodule.Manager, cdc codec.Codec) *TraceModuleManager {
	return &TraceModuleManager{Manager: other, cdc: cdc}
}

func (t TraceModuleManager) Precommit(rootCtx sdk.Context) (err error) {
	return traceModuleCall[appmodule.HasPrecommit](rootCtx, PrecommitOperationName, t, t.Manager.OrderPrecommiters, func(ctx sdk.Context, module appmodule.HasPrecommit) error {
		return module.Precommit(ctx)
	})
}

func (t TraceModuleManager) PrepareCheckState(rootCtx sdk.Context) error {
	return traceModuleCall[appmodule.HasPrepareCheckState](rootCtx, PrepareCheckStateOperationName, t, t.Manager.OrderPrepareCheckStaters, func(ctx sdk.Context, module appmodule.HasPrepareCheckState) error {
		return module.PrepareCheckState(ctx)
	})
}

func (t TraceModuleManager) RegisterServices(cfg sdkmodule.Configurator) error {
	return t.Manager.RegisterServices(NewTraceModuleConfigurator(t.cdc, cfg))
}

func (t TraceModuleManager) RunMigrations(rootCtx sdk.Context, cfg sdkmodule.Configurator, fromVM sdkmodule.VersionMap) (migrations sdkmodule.VersionMap, err error) {
	DoWithTracing(rootCtx, BeginBlockOperationName, nothing, func(ctx sdk.Context, span opentracing.Span) error {
		migrations, err = t.Manager.RunMigrations(ctx, cfg, fromVM)
		return err
	})
	return migrations, err
}

func traceModuleCall[T any](rootCtx sdk.Context, traceName string, t TraceModuleManager, order []string, f func(ctx sdk.Context, module T) error) (retErr error) {
	DoWithTracing(rootCtx, "root_"+traceName, nothing, func(parentCtx sdk.Context, span opentracing.Span) error {
		for _, moduleName := range order {
			module, ok := t.Manager.Modules[moduleName].(T)
			if !ok {
				continue
			}
			DoWithTracing(parentCtx, "module"+traceName, writesOnly, func(workCtx sdk.Context, span opentracing.Span) error {
				span.SetTag(tagModule, moduleName)
				retErr = f(workCtx, module)
				return retErr
			})
		}
		return retErr
	})
	return retErr
}

func (t TraceModuleManager) BeginBlock(rootCtx sdk.Context) (sdk.BeginBlock, error) {
	if !tracerEnabled {
		return t.Manager.BeginBlock(rootCtx)
	}

	rootCtx = rootCtx.WithEventManager(sdk.NewEventManager())
	err := traceModuleCall[appmodule.HasBeginBlocker](rootCtx, BeginBlockOperationName, t, t.Manager.OrderBeginBlockers, func(ctx sdk.Context, module appmodule.HasBeginBlocker) error {
		return module.BeginBlock(ctx)
	})
	return sdk.BeginBlock{
		Events: rootCtx.EventManager().ABCIEvents(),
	}, err
}

func (t TraceModuleManager) EndBlock(rootCtx sdk.Context) (sdk.EndBlock, error) {
	if !tracerEnabled {
		return t.Manager.EndBlock(rootCtx)
	}

	rootCtx = rootCtx.WithEventManager(sdk.NewEventManager())
	var (
		validatorUpdates []abci.ValidatorUpdate
		retErr           error
	)
	DoWithTracing(rootCtx, ABCIEndBlockOperationName, nothing, func(parentCtx sdk.Context, span opentracing.Span) error {
		for _, moduleName := range t.Manager.OrderEndBlockers {
			if module, ok := t.Manager.Modules[moduleName].(appmodule.HasEndBlocker); ok {
				DoWithTracing(parentCtx, "module_end_block", writesOnly, func(workCtx sdk.Context, span opentracing.Span) error {
					span.SetTag(tagModule, moduleName)
					retErr = module.EndBlock(workCtx)
					return retErr
				})
			} else if module, ok := t.Manager.Modules[moduleName].(sdkmodule.HasABCIEndBlock); ok {
				DoWithTracing(parentCtx, "module_end_block", writesOnly, func(workCtx sdk.Context, span opentracing.Span) error {
					span.SetTag(tagModule, moduleName)

					var moduleValUpdates []abci.ValidatorUpdate
					moduleValUpdates, retErr = module.EndBlock(workCtx)
					if retErr != nil {
						return retErr
					}
					// use these validator updates if provided, the module manager assumes
					// only one module will update the validator set
					if len(moduleValUpdates) > 0 {
						span.SetTag(tagValsetUpdate, true)
						span.LogFields(safeLogField(logValsetDiff, toJson(moduleValUpdates)))

						if len(validatorUpdates) > 0 {
							retErr = errors.New("validator EndBlock updates already set by a previous module")
							return retErr
						}
						for _, updates := range moduleValUpdates {
							validatorUpdates = append(validatorUpdates, abci.ValidatorUpdate{PubKey: updates.PubKey, Power: updates.Power})
						}
					}
					return retErr
				})
			} else {
				continue
			}
		}
		return retErr
	})
	if retErr != nil {
		return sdk.EndBlock{}, retErr
	}
	return sdk.EndBlock{
		ValidatorUpdates: validatorUpdates,
		Events:           rootCtx.EventManager().ABCIEvents(),
	}, nil
}

func (t TraceModuleManager) GetVersionMap() sdkmodule.VersionMap {
	return t.Manager.GetVersionMap()
}

func (t TraceModuleManager) GetModules() map[string]any {
	return t.Manager.Modules
}

func (t TraceModuleManager) GetConsensusVersion(module string) uint64 {
	return t.GetVersionMap()[module]
}

func (t TraceModuleManager) ModuleNames() []string {
	return t.Manager.ModuleNames()
}

// NewTraceModuleConfigurator constructor
func NewTraceModuleConfigurator(cdc codec.Codec, nested sdkmodule.Configurator) sdkmodule.Configurator {
	msgServer := NewTracingGRPCServer(nested.MsgServer(), cdc)
	if os.Getenv("no_tracing_msg_service") != "" {
		msgServer = nested.MsgServer()
	}
	queryServer := NewTracingGRPCServer(nested.QueryServer(), cdc)
	if os.Getenv("no_tracing_query_service") != "" {
		queryServer = nested.QueryServer()
	}
	return sdkmodule.NewConfigurator(cdc, msgServer, queryServer)
}

var _ grpc.Server = &TraceGRPCServer{}

type TraceGRPCServer struct {
	other grpc.Server
	cdc   codec.Codec
}

func NewTracingGRPCServer(other grpc.Server, cdc codec.Codec) grpc.Server {
	if !tracerEnabled {
		return other
	}
	return &TraceGRPCServer{other: other, cdc: cdc}
}

type methodHandlerFunc = func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor stdgrpc.UnaryServerInterceptor) (interface{}, error)

func (t *TraceGRPCServer) RegisterService(sd *stdgrpc.ServiceDesc, handler interface{}) {
	traceMethods := make([]stdgrpc.MethodDesc, len(sd.Methods))
	for i, method := range sd.Methods {
		traceMethods[i] = stdgrpc.MethodDesc{
			MethodName: method.MethodName,
			Handler:    t.decorateHandler(method.Handler, fmt.Sprintf("/%s/%s", sd.ServiceName, method.MethodName)),
		}
	}
	t.other.RegisterService(&stdgrpc.ServiceDesc{
		ServiceName: sd.ServiceName,
		HandlerType: sd.HandlerType,
		Methods:     traceMethods,
		Streams:     sd.Streams,
		Metadata:    sd.Metadata,
	}, handler)
}

func (t *TraceGRPCServer) decorateHandler(methodHandler methodHandlerFunc, fqMethod string) methodHandlerFunc {
	return func(srv interface{}, goCtx1 context.Context, dec func(interface{}) error, nestedInterceptor stdgrpc.UnaryServerInterceptor) (interface{}, error) {
		if os.Getenv("no_tracing_service") != "" {
			return methodHandler(srv, goCtx1, dec, nestedInterceptor)
		}
		traceInterceptor := func(goCtx2 context.Context, req1 interface{}, info *stdgrpc.UnaryServerInfo, nestedHandler stdgrpc.UnaryHandler) (interface{}, error) {
			if nestedInterceptor != nil {
				return nestedInterceptor(goCtx2, req1, info, t.traceHandler(fqMethod, nestedHandler))
			}
			// queries do not have an interceptor so that we can call the handler directly
			return t.traceHandler(fqMethod, nestedHandler)(goCtx2, req1)
		}
		return methodHandler(srv, goCtx1, dec, traceInterceptor)
	}
}

func (t *TraceGRPCServer) traceHandler(fqMethod string, nestedHandler stdgrpc.UnaryHandler) func(goCtx3 context.Context, req2 interface{}) (result interface{}, err error) {
	return func(goCtx3 context.Context, req2 interface{}) (result interface{}, err error) {
		DoWithTracing(sdk.UnwrapSDKContext(goCtx3), "service", writesOnly,
			func(workCtx sdk.Context, span opentracing.Span) error {
				span.SetTag(tagSDKGRPCService, fqMethod)
				result, err = nestedHandler(workCtx, req2)
				if err != nil {
					return err
				}
				if os.Getenv("no_tracing_service_result") != "" {
					return nil
				}
				span.LogFields(safeLogField("raw_wasm_message_result", safeToJson(t.cdc, result)))
				return nil
			})
		return
	}
}

func safeToJson(cdc codec.Codec, obj any) string {
	var resJson string
	if p, ok := obj.(proto.Message); ok {
		if bz, err := cdc.MarshalJSON(p); err != nil {
			resJson = string(bz)
		} else {
			resJson = fmt.Sprintf("marshal error: %s", err)
		}
	} else {
		resJson = toJson(obj)
	}
	return resJson
}
