package tracing

import (
	"bytes"

	"cosmossdk.io/store/gaskv"
	"cosmossdk.io/store/tracekv"
	storetypes "cosmossdk.io/store/types"
)

// TracingMultiStore Multistore that traces all operations
type TracingMultiStore struct {
	storetypes.MultiStore
	buf             bytes.Buffer
	traceWritesOnly bool
	traceGasMeter   *TraceGasMeter
}

// NewTracingMultiStore constructor
func NewTracingMultiStore(store storetypes.MultiStore, traceWritesOnly bool) *TracingMultiStore {
	return &TracingMultiStore{
		MultiStore:      store,
		traceWritesOnly: traceWritesOnly,
		traceGasMeter:   NewTraceGasMeter(storetypes.NewInfiniteGasMeter()),
	}
}

func (t *TracingMultiStore) GetStore(k storetypes.StoreKey) storetypes.Store {
	return tracekv.NewStore(t.MultiStore.GetKVStore(k), &t.buf, nil)
}

func (t *TracingMultiStore) GetKVStore(k storetypes.StoreKey) storetypes.KVStore {
	parentStore := t.MultiStore.GetKVStore(k)
	// wrap with gaskv to track gas usage
	parentStore = gaskv.NewStore(parentStore, t.traceGasMeter, storetypes.KVGasConfig())
	// wrap with trace store
	traceStore := tracekv.NewStore(parentStore, &t.buf, nil)
	if !t.traceWritesOnly {
		return traceStore
	}
	return NewTraceWritesOnlyStore(parentStore, traceStore)
}

var _ storetypes.KVStore = &TraceWritesKVStore{}

// TraceWritesKVStore decorator to log only write operations
type TraceWritesKVStore struct {
	parent storetypes.KVStore
	*tracekv.Store
}

// NewTraceWritesOnlyStore constructor
func NewTraceWritesOnlyStore(parent storetypes.KVStore, traceStore *tracekv.Store) *TraceWritesKVStore {
	return &TraceWritesKVStore{parent: parent, Store: traceStore}
}

func (t *TraceWritesKVStore) Iterator(start, end []byte) storetypes.Iterator {
	return t.parent.Iterator(start, end)
}

func (t *TraceWritesKVStore) ReverseIterator(start, end []byte) storetypes.Iterator {
	return t.parent.ReverseIterator(start, end)
}

func (t *TraceWritesKVStore) Get(key []byte) []byte {
	return t.parent.Get(key)
}

func (t *TraceWritesKVStore) Has(key []byte) bool {
	return t.parent.Has(key)
}

func (t *TraceWritesKVStore) Set(key, value []byte) {
	t.Store.Set(key, value)
}

func (t *TraceWritesKVStore) Delete(key []byte) {
	t.Store.Delete(key)
}

func (t *TracingMultiStore) getStoreDataLimited(max int) string {
	return cutLength(t.buf.String(), max)
}
