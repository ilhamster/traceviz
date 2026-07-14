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
	// CausalityEntryIDProperty is the TraceViz property containing the stable
	// identifier for a single focused-span causality table row.
	CausalityEntryIDProperty = "causality_entry_id"
	// CausalityEntryIDsProperty is the TraceViz property containing every
	// focused-span causality entry represented by a rendered feature.
	CausalityEntryIDsProperty = "causality_entry_ids"

	// SpanCausalityEventEntry records an instantaneous Tracey dependency
	// endpoint or mark.
	SpanCausalityEventEntry SpanCausalityEntryKind = "event"
	// SpanCausalitySuspendEntry records an interval during which the span is
	// suspended.
	SpanCausalitySuspendEntry SpanCausalityEntryKind = "suspend"
)

// SpanCausalityEntry describes one internal causality item in a focused span.
type SpanCausalityEntry struct {
	// ID correlates this unfiltered entry with any downsampled rendered feature
	// that represents it.
	ID             string
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
			ID:       suspendCausalityEntryID(spanID, suspend),
			Kind:     SpanCausalitySuspendEntry,
			Type:     "suspend",
			Time:     suspend.Start,
			Duration: suspend.End - suspend.Start,
		})
	}
	for _, event := range spanCausalEvents(span, t.namer) {
		ret = append(ret, SpanCausalityEntry{
			ID:             eventCausalityEntryID(spanID, event),
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

func eventCausalityEntryID(spanID string, event renderedCausalEvent) string {
	return fmt.Sprintf("%s:event:%d:%d", spanID, event.moment, event.sequence)
}

func suspendCausalityEntryID(spanID string, suspend rendertrace.TimeRange) string {
	return fmt.Sprintf("%s:suspend:%d:%d", spanID, suspend.Start, suspend.End)
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
