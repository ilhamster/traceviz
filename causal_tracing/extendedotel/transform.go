package extendedotel

import (
	"fmt"
	"time"

	"github.com/ilhamster/tracey/trace"
	traceparser "github.com/ilhamster/tracey/transform/parser"
)

// TransformTemplate parses and applies a Tracey transform template to this
// trace, returning a new extended OTel wrapper over the transformed Tracey
// trace. An empty template returns the receiver.
func (t *Trace) TransformTemplate(template string) (*Trace, error) {
	if template == "" {
		return t, nil
	}
	parsedTransform, err := traceparser.ParseTransformTemplate(
		ServiceHierarchyType,
		t.namer,
		template,
	)
	if err != nil {
		return nil, err
	}
	transformed, err := parsedTransform.TransformTrace(t.trace)
	if err != nil {
		return nil, err
	}
	return t.withTrace(transformed)
}

func (t *Trace) withTrace(
	transformed trace.Trace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) (*Trace, error) {
	if transformed == nil {
		return nil, fmt.Errorf("transformed trace is nil")
	}
	return &Trace{
		raw:          t.raw,
		trace:        transformed,
		namer:        t.namer,
		originMicros: t.originMicros,
		diagnostics:  t.Diagnostics(),
		spansByID:    spansByID(transformed),
		concurrency: concurrencyProfiles(
			transformed,
			ServiceHierarchyType,
			ProcessHierarchyType,
			SpanHierarchyType,
		),
	}, nil
}

func spansByID(
	tr trace.Trace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) map[string]trace.RootSpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	ret := map[string]trace.RootSpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{}
	for _, rootSpan := range tr.RootSpans() {
		payload := rootSpan.Payload()
		if payload == nil || payload.SpanID == "" {
			continue
		}
		ret[payload.SpanID] = rootSpan
	}
	return ret
}
