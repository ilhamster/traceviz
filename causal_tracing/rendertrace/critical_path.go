package rendertrace

import (
	"context"
	"fmt"
	"strings"

	criticalpath "github.com/ilhamster/tracey/critical_path"
	"github.com/ilhamster/tracey/trace"
	traceparser "github.com/ilhamster/tracey/trace/parser"
)

// findCriticalPath computes the configured critical path for a render request.
func findCriticalPath[T any, CP, SP, DP fmt.Stringer](
	ctx context.Context,
	tr trace.Trace[T, CP, SP, DP],
	req RenderRequest,
) (*criticalpath.Path[T, CP, SP, DP], error) {
	strategy, err := criticalPathStrategy(req.ResolvedCriticalPathStrategy())
	if err != nil {
		return nil, err
	}
	if usesDefaultCriticalPathEndpoints(req) {
		origin, destination, err := defaultTemporalCriticalPathEndpointSpans(ctx, tr)
		if err != nil {
			return nil, err
		}
		return criticalpath.FindBetweenElementarySpans(
			tr,
			origin,
			destination,
			strategy,
		)
	}
	origin, err := criticalPathEndpoint(
		ctx,
		tr,
		req.HierarchyType,
		req.ResolvedCriticalPathStart(),
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("critical path start endpoint: %w", err)
	}
	destination, err := criticalPathEndpoint(
		ctx,
		tr,
		req.HierarchyType,
		req.ResolvedCriticalPathEnd(),
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("critical path end endpoint: %w", err)
	}
	return criticalpath.FindBetweenEndpoints(
		tr,
		origin,
		destination,
		strategy,
	)
}

func usesDefaultCriticalPathEndpoints(req RenderRequest) bool {
	return strings.TrimSpace(req.ResolvedCriticalPathStart()) == DefaultCriticalPathStart &&
		strings.TrimSpace(req.ResolvedCriticalPathEnd()) == DefaultCriticalPathEnd
}

func criticalPathStrategy(strategyName string) (criticalpath.Strategy, error) {
	typeData, err := criticalpath.CommonStrategies.ByName(strategyName)
	if err != nil {
		return 0, err
	}
	return typeData.Type, nil
}

func criticalPathEndpoint[T any, CP, SP, DP fmt.Stringer](
	ctx context.Context,
	tr trace.Trace[T, CP, SP, DP],
	hierarchyType trace.HierarchyType,
	specifier string,
	isStart bool,
) (*criticalpath.Endpoint[T, CP, SP, DP], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalizedSpecifier := normalizeCriticalPathEndpointSpecifier(specifier, isStart)
	positionPattern, err := traceparser.ParsePositionSpecifiers(hierarchyType, normalizedSpecifier)
	if err != nil {
		return nil, err
	}
	positionFinder, err := traceparser.NewPositionFinder(positionPattern, tr)
	if err != nil {
		return nil, err
	}
	positions := positionFinder.FindPositions()
	if len(positions) == 0 {
		return nil, fmt.Errorf("no positions match %q", specifier)
	}
	if len(positions) > 1 {
		return nil, fmt.Errorf("%d positions match %q; add earliest or latest to disambiguate", len(positions), specifier)
	}
	return criticalpath.EndpointFromElementarySpanPosition(positions[0]), nil
}

func normalizeCriticalPathEndpointSpecifier(specifier string, isStart bool) string {
	trimmed := strings.TrimSpace(specifier)
	if trimmed == "" {
		if isStart {
			trimmed = DefaultCriticalPathStart
		} else {
			trimmed = DefaultCriticalPathEnd
		}
	}
	switch strings.ToLower(trimmed) {
	case "** earliest":
		return "** @ 0% earliest"
	case "** latest":
		return "** @ 100% latest"
	default:
		return trimmed
	}
}

func defaultTemporalCriticalPathEndpointSpans[T any, CP, SP, DP fmt.Stringer](
	ctx context.Context,
	tr trace.Trace[T, CP, SP, DP],
) (
	trace.ElementarySpan[T, CP, SP, DP],
	trace.ElementarySpan[T, CP, SP, DP],
	error,
) {
	comparator := tr.Comparator()
	var origin trace.ElementarySpan[T, CP, SP, DP]
	var destination trace.ElementarySpan[T, CP, SP, DP]
	for _, rootSpan := range tr.RootSpans() {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		visitElementarySpans(rootSpan, func(elementarySpan trace.ElementarySpan[T, CP, SP, DP]) {
			if elementarySpan == nil || comparator.LessOrEqual(elementarySpan.End(), elementarySpan.Start()) {
				return
			}
			if origin == nil ||
				comparator.Less(elementarySpan.End(), origin.End()) ||
				(comparator.Equal(elementarySpan.End(), origin.End()) &&
					comparator.Less(elementarySpan.Start(), origin.Start())) {
				origin = elementarySpan
			}
			if destination == nil ||
				comparator.Less(destination.Start(), elementarySpan.Start()) ||
				(comparator.Equal(destination.Start(), elementarySpan.Start()) &&
					comparator.Less(destination.End(), elementarySpan.End())) {
				destination = elementarySpan
			}
		})
	}
	if origin == nil || destination == nil {
		return nil, nil, fmt.Errorf("cannot compute critical path for trace with no non-zero elementary spans")
	}
	if origin == destination || !comparator.LessOrEqual(origin.End(), destination.Start()) {
		return nil, nil, fmt.Errorf(
			"cannot compute temporal critical path: default origin %s (%v-%v) does not precede destination %s (%v-%v)",
			tr.DefaultNamer().SpanName(origin.Span()),
			origin.Start(),
			origin.End(),
			tr.DefaultNamer().SpanName(destination.Span()),
			destination.Start(),
			destination.End(),
		)
	}
	return origin, destination, nil
}

func visitElementarySpans[T any, CP, SP, DP fmt.Stringer](
	span trace.Span[T, CP, SP, DP],
	visit func(trace.ElementarySpan[T, CP, SP, DP]),
) {
	for _, elementarySpan := range span.ElementarySpans() {
		visit(elementarySpan)
	}
	for _, childSpan := range span.ChildSpans() {
		visitElementarySpans(childSpan, visit)
	}
}
