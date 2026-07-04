package extendedotel

import (
	"time"

	"github.com/ilhamster/traceviz/causal_tracing/concurrency"
	"github.com/ilhamster/tracey/trace"
)

func (cs *conversionState) buildConcurrencyMaps() map[string]*concurrency.Profile {
	return concurrencyProfiles(
		cs.trace,
		ServiceHierarchyType,
		ProcessHierarchyType,
		ServiceSpawnHierarchyType,
	)
}

func concurrencyProfiles(
	tr trace.Trace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	hierarchyTypes ...trace.HierarchyType,
) map[string]*concurrency.Profile {
	return concurrency.ProfilesByTraceCategories(
		tr,
		time.Duration(0),
		hierarchyTypes...,
	)
}
