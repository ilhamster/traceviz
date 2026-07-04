package rendertrace

import (
	"context"
	"fmt"
	"strings"

	criticalpath "github.com/ilhamster/tracey/critical_path"
	"github.com/ilhamster/tracey/trace"
	traceparser "github.com/ilhamster/tracey/trace/parser"
)

// ValidateCriticalPath confirms that the configured strategy and endpoints can
// resolve to a critical path on the provided trace.
func ValidateCriticalPath[T any, CP, SP, DP fmt.Stringer](
	ctx context.Context,
	tr trace.Trace[T, CP, SP, DP],
	req RenderRequest,
) error {
	_, err := findCriticalPath(ctx, tr, req)
	return err
}

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
	var defaultOrigin trace.ElementarySpan[T, CP, SP, DP]
	var defaultDestination trace.ElementarySpan[T, CP, SP, DP]
	if strings.TrimSpace(req.ResolvedCriticalPathStart()) == DefaultCriticalPathStart ||
		strings.TrimSpace(req.ResolvedCriticalPathEnd()) == DefaultCriticalPathEnd {
		var err error
		defaultOrigin, defaultDestination, err = defaultTemporalCriticalPathEndpointSpans(ctx, tr)
		if err != nil {
			return nil, err
		}
	}

	var origin *criticalpath.Endpoint[T, CP, SP, DP]
	if strings.TrimSpace(req.ResolvedCriticalPathStart()) == DefaultCriticalPathStart {
		origin = criticalpath.EndpointFromElementarySpan(defaultOrigin, true)
	} else {
		var err error
		origin, err = criticalPathEndpoint(
			ctx,
			tr,
			req.HierarchyType,
			req.ResolvedCriticalPathStart(),
		)
		if err != nil {
			return nil, fmt.Errorf("critical path start endpoint: %w", err)
		}
	}

	var destination *criticalpath.Endpoint[T, CP, SP, DP]
	if strings.TrimSpace(req.ResolvedCriticalPathEnd()) == DefaultCriticalPathEnd {
		destination = criticalpath.EndpointFromElementarySpan(defaultDestination, false)
	} else {
		var err error
		destination, err = criticalPathEndpoint(
			ctx,
			tr,
			req.HierarchyType,
			req.ResolvedCriticalPathEnd(),
		)
		if err != nil {
			return nil, fmt.Errorf("critical path end endpoint: %w", err)
		}
	}
	if strategy == criticalpath.PreferTemporalMostWork {
		if err := validateTemporalCriticalPathEndpoints(
			tr,
			origin,
			destination,
			req.ResolvedCriticalPathStart(),
			req.ResolvedCriticalPathEnd(),
		); err != nil {
			return nil, err
		}
	}
	return criticalpath.FindBetweenEndpoints(
		tr,
		origin,
		destination,
		strategy,
	)
}

func validateTemporalCriticalPathEndpoints[T any, CP, SP, DP fmt.Stringer](
	tr trace.Trace[T, CP, SP, DP],
	origin *criticalpath.Endpoint[T, CP, SP, DP],
	destination *criticalpath.Endpoint[T, CP, SP, DP],
	originSpecifier string,
	destinationSpecifier string,
) error {
	comparator := tr.Comparator()
	originElementarySpan := origin.ElementarySpan(comparator)
	if originElementarySpan == nil {
		return fmt.Errorf("critical path origin %q resolves to %s @%v, but that span is not running there",
			originSpecifier, tr.DefaultNamer().SpanName(origin.Span), origin.At)
	}
	destinationElementarySpan := destination.ElementarySpan(comparator)
	if destinationElementarySpan == nil {
		return fmt.Errorf("critical path destination %q resolves to %s @%v, but that span is not running there",
			destinationSpecifier, tr.DefaultNamer().SpanName(destination.Span), destination.At)
	}
	if originElementarySpan == destinationElementarySpan {
		return fmt.Errorf(
			"critical path endpoints resolve to the same elementary span %s (%v-%v): origin %q at %v, destination %q at %v; temporal critical paths require distinct ordered elementary spans",
			tr.DefaultNamer().SpanName(originElementarySpan.Span()),
			originElementarySpan.Start(),
			originElementarySpan.End(),
			originSpecifier,
			origin.At,
			destinationSpecifier,
			destination.At,
		)
	}
	if !comparator.LessOrEqual(originElementarySpan.End(), destinationElementarySpan.Start()) {
		return fmt.Errorf(
			"critical path origin %s (%v-%v from %q at %v) does not precede destination %s (%v-%v from %q at %v); temporal critical paths require the origin elementary span to end no later than the destination elementary span starts",
			tr.DefaultNamer().SpanName(originElementarySpan.Span()),
			originElementarySpan.Start(),
			originElementarySpan.End(),
			originSpecifier,
			origin.At,
			tr.DefaultNamer().SpanName(destinationElementarySpan.Span()),
			destinationElementarySpan.Start(),
			destinationElementarySpan.End(),
			destinationSpecifier,
			destination.At,
		)
	}
	return nil
}

func usesDefaultCriticalPathEndpoints(req RenderRequest) bool {
	return strings.TrimSpace(req.ResolvedCriticalPathStart()) == DefaultCriticalPathStart &&
		strings.TrimSpace(req.ResolvedCriticalPathEnd()) == DefaultCriticalPathEnd
}

func criticalPathStrategy(strategySpecifier string) (criticalpath.Strategy, error) {
	if typeData, err := criticalpath.CommonStrategies.ByName(strategySpecifier); err == nil {
		return typeData.Type, nil
	}
	typeData, err := criticalpath.CommonStrategies.ByDescription(strategySpecifier)
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
) (*criticalpath.Endpoint[T, CP, SP, DP], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalizedSpecifier := strings.TrimSpace(specifier)
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
