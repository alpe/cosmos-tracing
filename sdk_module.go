package tracing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
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
	RegisterServices(cfg module.Configurator)
	InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, genesisData map[string]json.RawMessage) abci.ResponseInitChain
	ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) map[string]json.RawMessage
	RunMigrations(ctx sdk.Context, cfg module.Configurator, fromVM module.VersionMap) (module.VersionMap, error)
	BeginBlock(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock
	EndBlock(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock
	GetVersionMap() module.VersionMap
	ModuleNames() []string
	GetModules() map[string]any
	GetConsensusVersion(module string) uint64
	ExportGenesisForModules(ctx sdk.Context, cdc codec.JSONCodec, modulesToExport []string) map[string]json.RawMessage
}

const (
	ABCIBeginBlockOperationName   = "abci_begin_block"
	ABCIEndBlockOperationName     = "abci_end_block"
	ModuleBeginBlockOperationName = "module_begin_block"
	ModuleEndBlockOperationName   = "module_end_block"
)

var _ ModuleManager = &TraceModuleManager{}

type TraceModuleManager struct {
	other *module.Manager
	cdc   codec.Codec
}

func NewTraceModuleManager(other *module.Manager, cdc codec.Codec) *TraceModuleManager {
	return &TraceModuleManager{other: other, cdc: cdc}
}

func (t TraceModuleManager) SetOrderInitGenesis(moduleNames ...string) {
	t.other.SetOrderInitGenesis(moduleNames...)
}

func (t TraceModuleManager) SetOrderExportGenesis(moduleNames ...string) {
	t.other.SetOrderExportGenesis(moduleNames...)
}

func (t TraceModuleManager) SetOrderBeginBlockers(moduleNames ...string) {
	t.other.SetOrderBeginBlockers(moduleNames...)
}

func (t TraceModuleManager) SetOrderEndBlockers(moduleNames ...string) {
	t.other.SetOrderEndBlockers(moduleNames...)
}

func (t TraceModuleManager) SetOrderMigrations(moduleNames ...string) {
	t.other.SetOrderMigrations(moduleNames...)
}

func (t TraceModuleManager) RegisterInvariants(ir sdk.InvariantRegistry) {
	t.other.RegisterInvariants(ir)
}

func (t TraceModuleManager) RegisterServices(cfg module.Configurator) {
	t.other.RegisterServices(NewTraceModuleConfigurator(t.cdc, cfg))
}

func (t TraceModuleManager) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, genesisData map[string]json.RawMessage) abci.ResponseInitChain {
	return t.other.InitGenesis(ctx, cdc, genesisData)
}

func (t TraceModuleManager) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) map[string]json.RawMessage {
	return t.other.ExportGenesis(ctx, cdc)
}

func (t TraceModuleManager) RunMigrations(rootCtx sdk.Context, cfg module.Configurator, fromVM module.VersionMap) (migrations module.VersionMap, err error) {
	DoWithTracing(rootCtx, ABCIBeginBlockOperationName, writesOnly, func(ctx sdk.Context, span opentracing.Span) error {
		migrations, err = t.other.RunMigrations(ctx, cfg, fromVM)
		return err
	})
	return migrations, err
}

func (t TraceModuleManager) BeginBlock(rootCtx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	if !tracerEnabled {
		return t.other.BeginBlock(rootCtx, req)
	}

	rootCtx = rootCtx.WithEventManager(sdk.NewEventManager())

	DoWithTracing(rootCtx, ABCIBeginBlockOperationName, nothing, func(parentCtx sdk.Context, span opentracing.Span) error {
		// run by order defined as in the sdk
		for _, moduleName := range t.other.OrderBeginBlockers {
			module, ok := t.other.Modules[moduleName].(module.BeginBlockAppModule)
			if !ok {
				continue
			}
			DoWithTracing(parentCtx, ModuleBeginBlockOperationName, writesOnly, func(workCtx sdk.Context, span opentracing.Span) error {
				span.SetTag(tagModule, moduleName)
				module.BeginBlock(workCtx, req)
				return nil
			})
		}
		return nil
	})

	return abci.ResponseBeginBlock{
		Events: rootCtx.EventManager().ABCIEvents(),
	}
}

func (t TraceModuleManager) EndBlock(rootCtx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	if !tracerEnabled {
		return t.other.EndBlock(rootCtx, req)
	}

	rootCtx = rootCtx.WithEventManager(sdk.NewEventManager())
	validatorUpdates := []abci.ValidatorUpdate{} // nolint
	DoWithTracing(rootCtx, ABCIEndBlockOperationName, nothing, func(parentCtx sdk.Context, span opentracing.Span) error {
		for _, moduleName := range t.other.OrderEndBlockers {
			module, ok := t.other.Modules[moduleName].(module.EndBlockAppModule)
			if !ok {
				continue
			}
			DoWithTracing(parentCtx, ModuleEndBlockOperationName, writesOnly, func(workCtx sdk.Context, span opentracing.Span) error {
				span.SetTag(tagModule, moduleName)

				moduleValUpdates := module.EndBlock(workCtx, req)
				span.LogFields(safeLogField(logValsetDiff, toJson(moduleValUpdates)))
				// use these validator updates if provided, the module manager assumes
				// only one module will update the validator set
				if len(moduleValUpdates) > 0 {
					if len(validatorUpdates) > 0 {
						panic("validator EndBlock updates already set by a previous module")
					}

					validatorUpdates = moduleValUpdates
					span.SetTag(tagValsetUpdate, true)
				}
				return nil
			})
		}
		return nil
	})

	return abci.ResponseEndBlock{
		ValidatorUpdates: validatorUpdates,
		Events:           rootCtx.EventManager().ABCIEvents(),
	}
}

func (t TraceModuleManager) GetVersionMap() module.VersionMap {
	return t.other.GetVersionMap()
}

func (t TraceModuleManager) GetModules() map[string]any {
	return t.other.Modules
}

func (t TraceModuleManager) GetConsensusVersion(module string) uint64 {
	return t.GetVersionMap()[module]
}

func (t TraceModuleManager) ExportGenesisForModules(ctx sdk.Context, cdc codec.JSONCodec, modulesToExport []string) map[string]json.RawMessage {
	return t.other.ExportGenesisForModules(ctx, cdc, modulesToExport)
}

func (t TraceModuleManager) ModuleNames() []string {
	return t.other.ModuleNames()
}

var _ module.Configurator = &TraceModuleConfigurator{}

type TraceModuleConfigurator struct {
	nested module.Configurator
	cdc    codec.Codec
}

func NewTraceModuleConfigurator(cdc codec.Codec, nested module.Configurator) *TraceModuleConfigurator {
	return &TraceModuleConfigurator{nested: nested, cdc: cdc}
}

func (t *TraceModuleConfigurator) MsgServer() grpc.Server {
	return NewTracingGRPCServer(t.nested.MsgServer(), t.cdc)
}

func (t *TraceModuleConfigurator) QueryServer() grpc.Server {
	if os.Getenv("no_tracing_query_service") != "" {
		return t.nested.QueryServer()
	}
	return NewTracingGRPCServer(t.nested.QueryServer(), t.cdc)
}

func (t *TraceModuleConfigurator) RegisterMigration(moduleName string, forVersion uint64, handler module.MigrationHandler) error {
	return t.nested.RegisterMigration(moduleName, forVersion, handler)
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
				result, err = nestedHandler(sdk.WrapSDKContext(workCtx), req2)
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
