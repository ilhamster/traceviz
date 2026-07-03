package extendedotel

import (
	"fmt"
	"sort"
	"time"

	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
)

// SpanCausalityEntryKind identifies a focused-span table row.
type SpanCausalityEntryKind string

const (
	// SpanCausalityEventEntry records an instantaneous Tracey dependency
	// endpoint or mark.
	SpanCausalityEventEntry SpanCausalityEntryKind = "event"
	// SpanCausalitySuspendEntry records an interval during which the span is
	// suspended.
	SpanCausalitySuspendEntry SpanCausalityEntryKind = "suspend"
)

// SpanCausalityEntry describes one internal causality item in a focused span.
type SpanCausalityEntry struct {
	Kind           SpanCausalityEntryKind
	Type           string
	Time           time.Duration
	Duration       time.Duration
	Label          string
	DependencyType string
	DependencyKey  string
	OtherSpanID    string
	Detail         string
}

// SpanCausalityEntries returns unfiltered causality entries for a span.
func (t *Trace) SpanCausalityEntries(spanID string) ([]SpanCausalityEntry, error) {
	span := t.SpanByID(spanID)
	if span == nil {
		return nil, fmt.Errorf("unknown span_id %q", spanID)
	}
	var ret []SpanCausalityEntry
	for _, suspend := range suspendIntervals(span, rendertrace.TimeRange{
		Start: span.Start(),
		End:   span.End(),
	}) {
		ret = append(ret, SpanCausalityEntry{
			Kind:     SpanCausalitySuspendEntry,
			Type:     "suspend",
			Time:     suspend.Start,
			Duration: suspend.End - suspend.Start,
		})
	}
	for _, event := range spanCausalEvents(span, t.namer) {
		ret = append(ret, SpanCausalityEntry{
			Kind:           SpanCausalityEventEntry,
			Type:           string(event.kind),
			Time:           event.moment,
			Label:          event.label,
			DependencyType: event.dependencyType,
			DependencyKey:  event.dependencyKey,
			OtherSpanID:    event.otherSpanID,
			Detail:         event.detail,
		})
	}
	sort.SliceStable(ret, func(i, j int) bool {
		if ret[i].Time != ret[j].Time {
			return ret[i].Time < ret[j].Time
		}
		return spanCausalityKindPriority(ret[i].Kind) < spanCausalityKindPriority(ret[j].Kind)
	})
	return ret, nil
}

func spanCausalityKindPriority(kind SpanCausalityEntryKind) int {
	switch kind {
	case SpanCausalityEventEntry:
		return 0
	case SpanCausalitySuspendEntry:
		return 1
	default:
		return 2
	}
}
