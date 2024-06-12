package tracing

import (
	"testing"
	"time"

	"cosmossdk.io/x/evidence"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"github.com/cometbft/cometbft/libs/rand"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
	"github.com/cosmos/cosmos-sdk/types/module"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracingGRPCServer_RegisterService(t *testing.T) {
	tracerEnabled = true
	ctx, marshaler, storeSvc := createMinTestInput(t)

	ac := authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	keeper := evidencekeeper.NewKeeper(marshaler, storeSvc, nil, nil, ac, nil)
	keeper.SetRouter(evidencetypes.NewRouter())
	anyModule := evidence.NewAppModule(*keeper)
	evidencetypes.RegisterInterfaces(marshaler.InterfaceRegistry())
	mm := NewTraceModuleManager(module.NewManager(anyModule), marshaler)
	msgServiceRouter := baseapp.NewMsgServiceRouter()
	msgServiceRouter.SetInterfaceRegistry(marshaler.InterfaceRegistry())
	queryRouter := baseapp.NewGRPCQueryRouter()
	queryRouter.SetInterfaceRegistry(marshaler.InterfaceRegistry())
	err := mm.RegisterServices(module.NewConfigurator(marshaler, msgServiceRouter, queryRouter))
	require.NoError(t, err)

	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	// when
	msgSend, err := evidencetypes.NewMsgSubmitEvidence(rand.Bytes(address.Len), &evidencetypes.Equivocation{
		Height:           1,
		Time:             time.Now(),
		Power:            1,
		ConsensusAddress: "anyValidAddress",
	})
	require.NoError(t, err)
	h := msgServiceRouter.Handler(msgSend)
	_, err = h(ctx, msgSend)

	// then
	require.ErrorIs(t, err, evidencetypes.ErrNoEvidenceHandlerExists) // sdk 47
	// require.Error(t, err) // sdk 45
	spans := tracer.FinishedSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "service", spans[0].OperationName)
	assert.Contains(t, spans[0].Tags(), tagErrored)
}
