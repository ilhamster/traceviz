package extendedotel

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ilhamster/traceviz/causal_tracing/concurrency"
	"github.com/ilhamster/tracey/trace"
)

const (
	// ServiceHierarchyType groups spans by service and operation.
	ServiceHierarchyType trace.HierarchyType = trace.FirstUserDefinedHierarchyType + iota
	// ProcessHierarchyType groups spans by service and process ID.
	ProcessHierarchyType
	// SpanHierarchyType places each OTel span in its own category.
	SpanHierarchyType
	// ServiceSpawnHierarchyType groups spans by the service path implied by
	// OTel CHILD_OF references.
	ServiceSpawnHierarchyType
)

const (
	// DependencyLock represents causality from a lock release to a later lock
	// acquisition.
	DependencyLock trace.DependencyType = trace.FirstUserDefinedDependencyType + iota
	// DependencyRPC represents causality from a client-side call event to a
	// server-side start event. It is intentionally not trace.Call because OTel
	// RPC spans often live under different service/process categories.
	DependencyRPC
	// DependencyRPCReturn represents causality from a server-side finish event
	// to a client-side finish event. It is intentionally not trace.Return for
	// the same category-scope reason as DependencyRPC.
	DependencyRPCReturn
	// DependencySpawn represents a Tracey-style spawn dependency encoded
	// directly in extended OTel logs.
	DependencySpawn
	// DependencySend represents a Tracey-style send dependency encoded directly
	// in extended OTel logs.
	DependencySend
	// DependencySignal represents a Tracey-style signal dependency encoded
	// directly in extended OTel logs.
	DependencySignal
)

// Trace is a Tracey wrapper for one converted extended OTel trace.
type Trace struct {
	traceID      string
	trace        trace.Trace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	namer        *Namer
	originMicros int64
	diagnostics  []Diagnostic
	spansByID    map[string]trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	concurrency  map[string]*concurrency.Profile
}

// Trace returns the converted Tracey trace.
func (t *Trace) Trace() trace.Trace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	return t.trace
}

// TraceID returns the source trace's stable corpus identifier.
func (t *Trace) TraceID() string {
	return t.traceID
}

// Namer returns the Tracey namer for this trace type.
func (t *Trace) Namer() trace.Namer[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	return t.namer
}

// OriginMicros returns the epoch-microsecond timestamp used as render-time
// origin.
func (t *Trace) OriginMicros() int64 {
	return t.originMicros
}

// Diagnostics returns non-fatal conversion diagnostics.
func (t *Trace) Diagnostics() []Diagnostic {
	return append([]Diagnostic(nil), t.diagnostics...)
}

// SpanByID returns the converted span for an OTel span ID.
func (t *Trace) SpanByID(spanID string) trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	return t.spansByID[spanID]
}

// Diagnostic describes non-fatal conversion issues.
type Diagnostic struct {
	TraceID string
	SpanID  string
	Message string
}

// CategoryPayload stores category metadata not represented by Tracey itself.
type CategoryPayload struct {
	ID            string
	Name          string
	HierarchyType trace.HierarchyType
	ServiceName   string
	ProcessID     string
	OperationName string
	SpanID        string
}

func (cp *CategoryPayload) String() string {
	if cp == nil {
		return ""
	}
	return cp.Name
}

// SpanPayload stores source span metadata and raw OTel fields.
type SpanPayload struct {
	TraceID       string
	SpanID        string
	OperationName string
	ProcessID     string
	ServiceName   string
	StartTime     int64
	Duration      int64
	Tags          []KeyValue
	Logs          []RawLog
	References    []RawReference
	Warnings      []string
}

func (sp *SpanPayload) String() string {
	if sp == nil {
		return ""
	}
	if sp.OperationName == "" {
		return sp.SpanID
	}
	return sp.OperationName
}

// DependencyPayload stores source event metadata for a Tracey dependency.
type DependencyPayload struct {
	Kind             string
	Key              string
	OriginSpanID     string
	OriginTimestamp  int64
	DestSpanID       string
	DestTimestamp    int64
	OriginEvent      string
	DestinationEvent string
}

func (dp *DependencyPayload) String() string {
	if dp == nil {
		return ""
	}
	if dp.Key == "" {
		return dp.Kind
	}
	return fmt.Sprintf("%s:%s", dp.Kind, dp.Key)
}

func valueString(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case json.Number:
		return value.String()
	default:
		return fmt.Sprint(value)
	}
}
