package extendedotel

import (
	"fmt"
	"time"

	"github.com/ilhamster/tracey/trace"
)

// Namer names extended OTel Tracey elements.
type Namer struct{}

// CategoryName returns the matchable category name.
func (*Namer) CategoryName(
	category trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) string {
	return category.Payload().Name
}

// CategoryUniqueID returns the category's stable unique ID among siblings.
func (*Namer) CategoryUniqueID(
	category trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) string {
	return category.Payload().ID
}

// SpanName returns the matchable span name.
func (*Namer) SpanName(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) string {
	return span.Payload().OperationName
}

// SpanUniqueID returns the span's stable unique ID among siblings.
func (*Namer) SpanUniqueID(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) string {
	return span.Payload().SpanID
}

// HierarchyTypes returns the hierarchies supported by extended OTel traces.
func (*Namer) HierarchyTypes() *trace.HierarchyTypes {
	return trace.NewHierarchyTypes().
		With(ServiceHierarchyType, "service", "Service and operation hierarchy").
		With(ProcessHierarchyType, "process", "Service and process hierarchy").
		With(ServiceSpawnHierarchyType, "service_spawn", "Service spawning hierarchy")
}

// DependencyTypes returns dependency names supported by extended OTel traces.
func (*Namer) DependencyTypes() *trace.DependencyTypes {
	return trace.NewDependencyTypes().
		With(trace.Call, "call", "Tracey call dependency").
		With(trace.Return, "return", "Tracey return dependency").
		With(DependencyLock, "lock", "Lock release to acquire dependency").
		With(DependencyRPC, "rpc_call", "Client RPC call to server start dependency").
		With(DependencyRPCReturn, "rpc_return", "Server finish to client RPC finish dependency").
		With(DependencySpawn, "spawn", "Tracey-style spawn dependency").
		With(DependencySend, "send", "Tracey-style send dependency").
		With(DependencySignal, "signal", "Tracey-style signal dependency")
}

// MomentString formats a duration-offset moment.
func (*Namer) MomentString(t time.Duration) string {
	return fmt.Sprintf("%v", t)
}
