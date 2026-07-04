package extendedotel

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ilhamster/traceviz/causal_tracing/concurrency"
	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	"github.com/ilhamster/traceviz/server/go/category"
	"github.com/ilhamster/traceviz/server/go/color"
	"github.com/ilhamster/traceviz/server/go/label"
	tvtrace "github.com/ilhamster/traceviz/server/go/trace"
	traceedge "github.com/ilhamster/traceviz/server/go/trace_edge"
	"github.com/ilhamster/traceviz/server/go/util"
	criticalpath "github.com/ilhamster/tracey/critical_path"
	"github.com/ilhamster/tracey/trace"
	traceparser "github.com/ilhamster/tracey/trace/parser"
)

// RenderableTrace returns this extended OTel trace as a trace-format-agnostic
// renderable trace.
func (t *Trace) RenderableTrace() rendertrace.RenderableTrace {
	return rendertrace.AdaptTypedTrace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](t)
}

// ID returns this trace's stable ID within its corpus.
func (t *Trace) ID() rendertrace.TraceID {
	return rendertrace.TraceID(t.raw.TraceID)
}

// DisplayName returns a human-facing trace name.
func (t *Trace) DisplayName() string {
	if t.raw.TraceID == "" {
		return "extended OTel trace"
	}
	return t.raw.TraceID
}

// MomentString returns a human-facing string for a render-time moment.
func (t *Trace) MomentString(moment time.Duration) string {
	return moment.String()
}

// TimeRange returns this trace's full renderable time range.
func (t *Trace) TimeRange() rendertrace.TimeRange {
	var ret rendertrace.TimeRange
	for idx, rootSpan := range t.trace.RootSpans() {
		if idx == 0 || rootSpan.Start() < ret.Start {
			ret.Start = rootSpan.Start()
		}
		if idx == 0 || rootSpan.End() > ret.End {
			ret.End = rootSpan.End()
		}
	}
	return ret
}

// Search evaluates a render search over spans and categories in the selected
// hierarchy and visible time range.
func (t *Trace) Search(
	ctx context.Context,
	hierarchy trace.HierarchyType,
	query string,
	temporalDomain rendertrace.TimeRange,
) (*rendertrace.SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ret := &rendertrace.SearchResult{
		Categories:                    map[rendertrace.CategoryID]struct{}{},
		Spans:                         map[rendertrace.SpanID]struct{}{},
		CategoriesWithDescendantMatch: map[rendertrace.CategoryID]struct{}{},
		SpansWithDescendantMatch:      map[rendertrace.SpanID]struct{}{},
	}
	if query == "" {
		return ret, nil
	}
	spanPattern, err := traceparser.ParseSpanSpecifierPatterns(hierarchy, query)
	if err != nil {
		return nil, err
	}
	spanFinder, err := traceparser.NewSpanFinder(spanPattern, t.trace)
	if err != nil {
		return nil, err
	}
	spanFinder = spanFinder.WithSpanFilter(func(span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]) (include, prune bool) {
		if !spanOverlapsTimeRange(span, temporalDomain) {
			return false, true
		}
		return true, false
	})
	for _, span := range spanFinder.FindSpans() {
		spanID := rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
			span,
			t.namer,
		)
		ret.Spans[spanID] = struct{}{}
		markSpanAncestors(ret, span, t.namer)
		markSpanCategoryAncestors(ret, span.RootSpan().ParentCategory(hierarchy), t.namer)
	}
	categoryFinder, err := traceparser.NewSpanFinder(spanPattern, t.trace)
	if err != nil {
		return nil, err
	}
	for _, category := range categoryFinder.FindCategories(trace.UseSpanIfNoCategory) {
		if !categorySubtreeOverlapsTimeRange(category, temporalDomain) {
			continue
		}
		categoryID := rendertrace.DefaultCategoryID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
			category,
			t.namer,
		)
		ret.Categories[categoryID] = struct{}{}
		markSpanCategoryAncestors(ret, category.Parent(), t.namer)
	}
	return &rendertrace.SearchResult{
		Categories:                    ret.Categories,
		Spans:                         ret.Spans,
		CategoriesWithDescendantMatch: ret.CategoriesWithDescendantMatch,
		SpansWithDescendantMatch:      ret.SpansWithDescendantMatch,
	}, nil
}

// RootCategories returns root categories visible for the current render view.
func (t *Trace) RootCategories(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.CategoryView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var ret []rendertrace.CategoryView
	for _, category := range t.trace.RootCategories(view.Request.HierarchyType) {
		if !categoryVisible(category, view, t.namer) {
			continue
		}
		ret = append(ret, categoryView{
			category:    category,
			namer:       t.namer,
			concurrency: t.concurrency,
		})
	}
	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].ID() < ret[j].ID()
	})
	return ret, nil
}

// CriticalPathRootCategories returns synthetic categories for rendering the
// current critical path as an overtime TraceViz trace.
func (t *Trace) CriticalPathRootCategories(
	ctx context.Context,
	view rendertrace.RenderView,
	path *criticalpath.Path[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) ([]rendertrace.CategoryView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == nil || len(path.CriticalPath) == 0 {
		return nil, nil
	}
	return []rendertrace.CategoryView{criticalPathCategoryView{
		path:      path,
		hierarchy: view.Request.HierarchyType,
		namer:     t.namer,
	}}, nil
}

// CriticalPathOverlay projects the current critical path onto the visible main
// trace as TraceViz trace-edge nodes.
func (t *Trace) CriticalPathOverlay(
	ctx context.Context,
	view rendertrace.RenderView,
	path *criticalpath.Path[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) (*rendertrace.CriticalPathOverlay, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == nil || len(path.CriticalPath) == 0 {
		return nil, nil
	}
	projectedElements := make([]criticalPathOverlayElement, 0, len(path.CriticalPath))
	for idx, element := range path.CriticalPath {
		if element.End() < view.TemporalDomain.Start || element.Start() > view.TemporalDomain.End {
			continue
		}
		spanID, ok := t.criticalPathDisplaySpanID(element.Span(), view)
		if !ok {
			continue
		}
		start := clampDuration(element.Start(), view.TemporalDomain)
		end := clampDuration(element.End(), view.TemporalDomain)
		projectedElements = append(projectedElements, criticalPathOverlayElement{
			spanID:   spanID,
			sequence: idx,
			start:    start,
			end:      end,
		})
	}
	if len(projectedElements) == 0 {
		return nil, nil
	}
	overlay := &rendertrace.CriticalPathOverlay{
		NodesBySpanID: map[rendertrace.SpanID][]rendertrace.CriticalPathOverlayNode{},
	}
	for idx, element := range projectedElements {
		startNodeID := fmt.Sprintf("critical-path:%d:start", element.sequence)
		endNodeID := fmt.Sprintf("critical-path:%d:end", element.sequence)
		startEndpoints := []string{endNodeID}
		endEndpoints := []string{}
		if idx+1 < len(projectedElements) {
			endEndpoints = append(
				endEndpoints,
				fmt.Sprintf("critical-path:%d:start", projectedElements[idx+1].sequence),
			)
		}
		overlay.NodesBySpanID[element.spanID] = append(
			overlay.NodesBySpanID[element.spanID],
			rendertrace.CriticalPathOverlayNode{
				ID:              startNodeID,
				Moment:          element.start,
				EndpointNodeIDs: startEndpoints,
			},
			rendertrace.CriticalPathOverlayNode{
				ID:              endNodeID,
				Moment:          element.end,
				EndpointNodeIDs: endEndpoints,
			},
		)
	}
	return overlay, nil
}

// CriticalPathVisibility identifies the spans and categories that should remain
// visible when the main trace is filtered to the current critical path.
func (t *Trace) CriticalPathVisibility(
	ctx context.Context,
	view rendertrace.RenderView,
	path *criticalpath.Path[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) (*rendertrace.CriticalPathVisibility, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	visibility := &rendertrace.CriticalPathVisibility{
		Categories: map[rendertrace.CategoryID]struct{}{},
		Spans:      map[rendertrace.SpanID]struct{}{},
	}
	if path == nil {
		return visibility, nil
	}
	for _, element := range path.CriticalPath {
		if element.End() <= view.TemporalDomain.Start || element.Start() >= view.TemporalDomain.End {
			continue
		}
		markCriticalPathSpanVisibility(visibility, element.Span(), view.Request.HierarchyType, t.namer)
	}
	return visibility, nil
}

// FocusDependencyOverlay projects dependencies between spans in the current
// focused-span stack into TraceViz trace-edge nodes.
func (t *Trace) FocusDependencyOverlay(
	ctx context.Context,
	view rendertrace.RenderView,
) (*rendertrace.TraceEdgeOverlay, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	focusedSpans := t.focusedSpans(view)
	if len(focusedSpans) < 2 {
		return nil, nil
	}
	overlay := &rendertrace.TraceEdgeOverlay{
		NodesBySpanID: map[rendertrace.SpanID][]rendertrace.TraceEdgeOverlayNode{},
	}
	sequence := 0
	originSpanIDs := make([]rendertrace.SpanID, 0, len(focusedSpans))
	for originSpanID := range focusedSpans {
		originSpanIDs = append(originSpanIDs, originSpanID)
	}
	sort.Slice(originSpanIDs, func(i, j int) bool {
		return originSpanIDs[i] < originSpanIDs[j]
	})
	for _, originSpanID := range originSpanIDs {
		originSpan := focusedSpans[originSpanID]
		for _, elementarySpan := range originSpan.ElementarySpans() {
			dependency := elementarySpan.Outgoing()
			if dependency == nil ||
				!sameElementarySpan(elementarySpan, dependency.TriggeringOrigin(), t.namer) {
				continue
			}
			for destinationIndex, destination := range dependency.Destinations() {
				if destination == nil || destination.Span() == nil {
					continue
				}
				if !durationInRange(elementarySpan.End(), view.TemporalDomain) ||
					!durationInRange(destination.Start(), view.TemporalDomain) {
					continue
				}
				destinationSpanID := rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
					destination.Span(),
					t.namer,
				)
				if _, ok := focusedSpans[destinationSpanID]; !ok {
					continue
				}
				originNodeID := fmt.Sprintf("focus-dependency:%d:origin", sequence)
				destinationNodeID := fmt.Sprintf("focus-dependency:%d:destination:%d", sequence, destinationIndex)
				overlay.NodesBySpanID[originSpanID] = append(
					overlay.NodesBySpanID[originSpanID],
					rendertrace.TraceEdgeOverlayNode{
						ID:              originNodeID,
						Moment:          elementarySpan.End(),
						EndpointNodeIDs: []string{destinationNodeID},
					},
				)
				overlay.NodesBySpanID[destinationSpanID] = append(
					overlay.NodesBySpanID[destinationSpanID],
					rendertrace.TraceEdgeOverlayNode{
						ID:     destinationNodeID,
						Moment: destination.Start(),
					},
				)
				sequence++
			}
		}
	}
	if sequence == 0 {
		return nil, nil
	}
	return overlay, nil
}

func (t *Trace) focusedSpans(
	view rendertrace.RenderView,
) map[rendertrace.SpanID]trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	focusedPayloadIDs := map[string]struct{}{}
	for _, focusSpanID := range view.Request.FocusSpanIDs {
		focusedPayloadIDs[string(focusSpanID)] = struct{}{}
	}
	ret := map[rendertrace.SpanID]trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{}
	for _, rootSpan := range t.trace.RootSpans() {
		collectFocusedSpans(rootSpan, focusedPayloadIDs, t.namer, ret)
	}
	return ret
}

func collectFocusedSpans(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	focusedPayloadIDs map[string]struct{},
	namer *Namer,
	ret map[rendertrace.SpanID]trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) {
	if payload := span.Payload(); payload != nil {
		if _, ok := focusedPayloadIDs[payload.SpanID]; ok {
			spanID := rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
				span,
				namer,
			)
			ret[spanID] = span
		}
	}
	for _, child := range span.ChildSpans() {
		collectFocusedSpans(child, focusedPayloadIDs, namer, ret)
	}
}

func sameElementarySpan(
	left trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	right trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	namer *Namer,
) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftSpanID := rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
		left.Span(),
		namer,
	)
	rightSpanID := rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
		right.Span(),
		namer,
	)
	return leftSpanID == rightSpanID &&
		left.Start() == right.Start() &&
		left.End() == right.End()
}

type criticalPathOverlayElement struct {
	spanID   rendertrace.SpanID
	sequence int
	start    time.Duration
	end      time.Duration
}

func (t *Trace) criticalPathDisplaySpanID(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	view rendertrace.RenderView,
) (rendertrace.SpanID, bool) {
	rootSpan := span.RootSpan()
	if rootSpan == nil {
		return "", false
	}
	for category := rootSpan.ParentCategory(view.Request.HierarchyType); category != nil; category = category.Parent() {
		if (categoryView{category: category, namer: t.namer}).categoryIsOpen(view) {
			continue
		}
		if syntheticID, ok := syntheticCategorySpanID(category); ok {
			return syntheticID, true
		}
		return "", false
	}
	return spanView{span: span, namer: t.namer}.ID(), true
}

func syntheticCategorySpanID(
	category trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) (rendertrace.SpanID, bool) {
	payload := category.Payload()
	if payload == nil ||
		payload.ID == "" {
		return "", false
	}
	return rendertrace.SpanID("synthetic-category:" + payload.ID), true
}

func markCriticalPathSpanVisibility(
	visibility *rendertrace.CriticalPathVisibility,
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	hierarchy trace.HierarchyType,
	namer *Namer,
) {
	for cursor := span; cursor != nil; cursor = cursor.ParentSpan() {
		spanID := rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
			cursor,
			namer,
		)
		visibility.Spans[spanID] = struct{}{}
	}
	rootSpan := span.RootSpan()
	if rootSpan == nil {
		return
	}
	for category := rootSpan.ParentCategory(hierarchy); category != nil; category = category.Parent() {
		categoryID := rendertrace.DefaultCategoryID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
			category,
			namer,
		)
		visibility.Categories[categoryID] = struct{}{}
	}
}

type categoryView struct {
	category    trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	namer       *Namer
	concurrency map[string]*concurrency.Profile
}

func (cv categoryView) ID() rendertrace.CategoryID {
	return rendertrace.DefaultCategoryID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
		cv.category,
		cv.namer,
	)
}

func (cv categoryView) ChildCategories(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.CategoryView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var ret []rendertrace.CategoryView
	if !cv.categoryIsOpen(view) {
		return ret, nil
	}
	for _, child := range cv.category.ChildCategories() {
		if !categoryVisible(child, view, cv.namer) {
			continue
		}
		ret = append(ret, categoryView{
			category:    child,
			namer:       cv.namer,
			concurrency: cv.concurrency,
		})
	}
	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].ID() < ret[j].ID()
	})
	return ret, nil
}

func (cv categoryView) RootSpans(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.SpanView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var ret []rendertrace.SpanView
	if !cv.categoryIsOpen(view) {
		if synthetic := cv.syntheticCategorySpan(view); synthetic != nil {
			ret = append(ret, synthetic)
		}
		return ret, nil
	}
	for _, rootSpan := range cv.category.RootSpans() {
		if !spanVisible(rootSpan, view, cv.namer) {
			continue
		}
		ret = append(ret, spanView{
			span:  rootSpan,
			namer: cv.namer,
		})
	}
	sort.SliceStable(ret, func(i, j int) bool {
		left := ret[i].TimeRange()
		right := ret[j].TimeRange()
		if left.Start != right.Start {
			return left.Start < right.Start
		}
		return ret[i].ID() < ret[j].ID()
	})
	return ret, nil
}

func (cv categoryView) syntheticCategorySpan(view rendertrace.RenderView) rendertrace.SpanView {
	if len(view.Request.FocusSpanIDs) > 0 {
		return nil
	}
	payload := cv.category.Payload()
	if payload == nil ||
		payload.ID == "" {
		return nil
	}
	concurrencyMap := cv.concurrency[payload.ID]
	if concurrencyMap == nil || len(concurrencyMap.Segments) == 0 || concurrencyMap.End <= concurrencyMap.Start {
		return nil
	}
	return syntheticServiceSpanView{
		categoryPayload: payload,
		concurrencyMap:  concurrencyMap,
		namer:           cv.namer,
	}
}

func (cv categoryView) categoryIsOpen(view rendertrace.RenderView) bool {
	return cv.categoryExpansionState(view) != "collapsed"
}

func (cv categoryView) categoryExpansionState(view rendertrace.RenderView) string {
	if len(cv.category.ChildCategories()) == 0 {
		return "leaf"
	}
	if view.Request.ExpandMatches && categoryContainsSearchMatch(cv.ID(), view) {
		return "force_expanded"
	}
	if len(view.Request.FocusSpanIDs) > 0 {
		return "force_expanded"
	}
	if _, ok := view.Request.ExplicitExpanded[cv.ID()]; ok {
		return "expanded"
	}
	return "collapsed"
}

func categoryExpansionGlyph(state string) string {
	switch state {
	case "collapsed":
		return "▶ "
	case "expanded":
		return "▼ "
	case "force_expanded":
		return "◆ "
	default:
		return ""
	}
}

func (cv categoryView) RenderTraceVizCategory(
	ctx context.Context,
	view rendertrace.RenderView,
	parent rendertrace.TraceVizCategoryParent,
) (*tvtrace.Category[time.Duration], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	payload := cv.category.Payload()
	if payload == nil {
		return nil, fmt.Errorf("extended OTel category has nil payload")
	}
	palette := paletteForTheme(view.Request.Theme)
	categoryColor := serviceColor(payload.ServiceName, view)
	primaryColor := categoryColor
	if categoryMatchesSearch(cv.ID(), view) {
		primaryColor = palette.searchMatch
	}
	secondaryColor := categoryColor
	if categoryContainsSearchMatch(cv.ID(), view) {
		secondaryColor = palette.searchMatch
	}
	expansionState := cv.categoryExpansionState(view)
	return parent.Category(
		category.New(payload.ID, payload.Name, payload.Name),
		label.Format("$(category_label)"),
		color.Primary(primaryColor),
		color.Secondary(secondaryColor),
		color.Stroke(palette.stroke),
		util.StringProperty("category_id", string(cv.ID())),
		util.StringProperty("category_name", payload.Name),
		util.StringProperty("category_label", categoryExpansionGlyph(expansionState)+payload.Name),
		util.StringProperty("category_expansion_state", expansionState),
		util.StringProperty("service_name", payload.ServiceName),
	), nil
}

type spanView struct {
	span  trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	namer *Namer
}

type criticalPathCategoryView struct {
	path      *criticalpath.Path[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	hierarchy trace.HierarchyType
	namer     *Namer
}

func (cv criticalPathCategoryView) ID() rendertrace.CategoryID {
	return "critical-path:temporal"
}

func (cv criticalPathCategoryView) ChildCategories(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.CategoryView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (cv criticalPathCategoryView) RootSpans(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.SpanView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	nodes := buildCriticalPathFrameTree(cv.path, cv.hierarchy, cv.namer)
	ret := make([]rendertrace.SpanView, 0, len(nodes))
	for _, node := range nodes {
		if !criticalPathNodeVisible(node, view.TemporalDomain) {
			continue
		}
		ret = append(ret, criticalPathFrameSpanView{
			node:  node,
			namer: cv.namer,
		})
	}
	return ret, nil
}

func (cv criticalPathCategoryView) RenderTraceVizCategory(
	ctx context.Context,
	view rendertrace.RenderView,
	parent rendertrace.TraceVizCategoryParent,
) (*tvtrace.Category[time.Duration], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	palette := paletteForTheme(view.Request.Theme)
	return parent.Category(
		category.New(string(cv.ID()), "Temporal critical path", "Temporal critical path"),
		label.Format("$(category_label)"),
		color.Primary(palette.criticalPathCategory),
		color.Secondary(palette.criticalPathCategory),
		color.Stroke(palette.stroke),
		util.StringProperty("category_id", string(cv.ID())),
		util.StringProperty("category_name", "Temporal critical path"),
		util.StringProperty("category_label", "Temporal critical path"),
		util.StringProperty("category_expansion_state", "leaf"),
		util.StringProperty("critical_path_strategy", view.Request.ResolvedCriticalPathStrategy()),
	), nil
}

type criticalPathFrameKind string

const (
	criticalPathCategoryFrameKind criticalPathFrameKind = "critical_path_category_frame"
	criticalPathLeafFrameKind     criticalPathFrameKind = "critical_path_leaf"
)

type criticalPathFrame struct {
	key             string
	kind            criticalPathFrameKind
	span            trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	category        trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	serviceName     string
	name            string
	payload         *SpanPayload
	categoryPayload *CategoryPayload
}

type criticalPathFrameNode struct {
	frame         criticalPathFrame
	start         time.Duration
	end           time.Duration
	firstSequence int
	lastSequence  int
	parent        *criticalPathFrameNode
	children      []*criticalPathFrameNode
	gaps          []criticalPathNoLongerRunningGap
}

type criticalPathNoLongerRunningGap struct {
	start time.Duration
	end   time.Duration
}

type criticalPathFrameSpanView struct {
	node  *criticalPathFrameNode
	namer *Namer
}

func (sv criticalPathFrameSpanView) ID() rendertrace.SpanID {
	return sv.node.id()
}

func (sv criticalPathFrameSpanView) TimeRange() rendertrace.TimeRange {
	return rendertrace.TimeRange{Start: sv.node.start, End: sv.node.end}
}

func (sv criticalPathFrameSpanView) ChildSpans(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.SpanView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ret := make([]rendertrace.SpanView, 0, len(sv.node.children))
	for _, child := range sv.node.children {
		if !criticalPathNodeVisible(child, view.TemporalDomain) {
			continue
		}
		ret = append(ret, criticalPathFrameSpanView{
			node:  child,
			namer: sv.namer,
		})
	}
	return ret, nil
}

func (sv criticalPathFrameSpanView) RenderTraceVizSpan(
	ctx context.Context,
	view rendertrace.RenderView,
	parent rendertrace.TraceVizSpanParent,
) (*tvtrace.Span[time.Duration], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sv.node.frame.kind == criticalPathLeafFrameKind {
		return sv.renderLeafTraceVizSpan(view, parent)
	}
	categoryPayload := sv.node.frame.categoryPayload
	if categoryPayload == nil {
		categoryPayload = &CategoryPayload{}
	}
	name := categoryPayload.Name
	if name == "" {
		name = sv.node.frame.name
	}
	if name == "" {
		name = "category"
	}
	serviceName := categoryPayload.ServiceName
	palette := paletteForTheme(view.Request.Theme)
	serviceColor := serviceColor(serviceName, view)
	start, end := sv.clippedRange(view.TemporalDomain)
	return parent.Span(
		start,
		end,
		label.Format("$(span_name)"),
		color.Primary(serviceColor),
		color.Secondary(serviceColor),
		color.Stroke(palette.stroke),
		util.StringProperty("span_kind", string(criticalPathCategoryFrameKind)),
		util.StringProperty("span_name", name),
		util.StringProperty("category_name", name),
		util.StringProperty("category_id", categoryPayload.ID),
		util.StringProperty("service_name", serviceName),
		util.StringProperty("critical_path_strategy", view.Request.ResolvedCriticalPathStrategy()),
		util.IntegerProperty("critical_path_first_sequence", int64(sv.node.firstSequence)),
		util.IntegerProperty("critical_path_last_sequence", int64(sv.node.lastSequence)),
	), nil
}

func (sv criticalPathFrameSpanView) payload() *SpanPayload {
	payload := sv.node.frame.payload
	if payload == nil {
		return &SpanPayload{}
	}
	return payload
}

func (sv criticalPathFrameSpanView) spanID() string {
	payload := sv.payload()
	if payload.SpanID != "" {
		return payload.SpanID
	}
	if sv.node.frame.span != nil {
		return sv.namer.SpanName(sv.node.frame.span)
	}
	return string(sv.ID())
}

func (sv criticalPathFrameSpanView) renderLeafTraceVizSpan(
	view rendertrace.RenderView,
	parent rendertrace.TraceVizSpanParent,
) (*tvtrace.Span[time.Duration], error) {
	payload := sv.payload()
	labelText := payload.OperationName
	if labelText == "" {
		labelText = sv.spanID()
	}
	start, end := sv.clippedRange(view.TemporalDomain)
	palette := paletteForTheme(view.Request.Theme)
	renderedSpan := parent.Span(
		start,
		end,
		label.Format("$(span_name)"),
		color.Primary(palette.realSpan),
		color.Secondary(palette.realSpan),
		color.Stroke(palette.stroke),
		util.StringProperty("span_kind", string(criticalPathLeafFrameKind)),
		util.StringProperty("span_id", payload.SpanID),
		util.StringProperty("span_name", labelText),
		util.StringProperty("service_name", payload.ServiceName),
		util.StringProperty("operation_name", payload.OperationName),
		util.StringProperty("critical_path_strategy", view.Request.ResolvedCriticalPathStrategy()),
		util.IntegerProperty("critical_path_first_sequence", int64(sv.node.firstSequence)),
		util.IntegerProperty("critical_path_last_sequence", int64(sv.node.lastSequence)),
	)
	if sv.node.frame.span == nil {
		return renderedSpan, nil
	}
	clipDomain := rendertrace.TimeRange{Start: start, End: end}
	for _, suspend := range renderableSuspendIntervalsInDomain(sv.node.frame.span, view, clipDomain) {
		suspendTooltipLines := append(
			[]string{labelText, "Kind: critical_path_suspend"},
			spanIdentityTooltipLines(labelText, payload)[1:]...,
		)
		suspendTooltipLines = append(suspendTooltipLines, durationTooltipLine(suspend.Start, suspend.End))
		renderedSpan.Subspan(
			suspend.Start,
			suspend.End,
			color.Primary(palette.suspendOverlay),
			color.Secondary(palette.suspendOverlay),
			color.Stroke(palette.stroke),
			util.StringProperty("subspan_kind", "critical_path_suspend"),
			util.StringProperty("span_id", payload.SpanID),
			util.StringProperty("span_name", labelText),
			util.StringProperty("service_name", payload.ServiceName),
			util.StringProperty("operation_name", payload.OperationName),
			util.StringProperty("tooltip", tooltipText(suspendTooltipLines...)),
		)
	}
	for _, gap := range sv.node.gaps {
		gapStart := maxDuration(gap.start, start)
		gapEnd := minDuration(gap.end, end)
		if gapEnd <= gapStart {
			continue
		}
		gapTooltip := tooltipText(
			labelText,
			"Kind: critical_path_gap",
			fmt.Sprintf("Span %s no longer running", labelText),
			durationTooltipLine(gapStart, gapEnd),
		)
		renderedSpan.Subspan(
			gapStart,
			gapEnd,
			color.Primary(palette.criticalPathGapOverlay),
			color.Secondary(palette.criticalPathGapOverlay),
			color.Stroke(palette.stroke),
			util.StringProperty("subspan_kind", "critical_path_no_longer_running"),
			util.StringProperty("span_id", payload.SpanID),
			util.StringProperty("span_name", labelText),
			util.StringProperty("service_name", payload.ServiceName),
			util.StringProperty("operation_name", payload.OperationName),
			util.StringProperty("tooltip", gapTooltip),
		)
	}
	return renderedSpan, nil
}

func (sv criticalPathFrameSpanView) clippedRange(domain rendertrace.TimeRange) (time.Duration, time.Duration) {
	return maxDuration(sv.node.start, domain.Start), minDuration(sv.node.end, domain.End)
}

func (node *criticalPathFrameNode) id() rendertrace.SpanID {
	return rendertrace.SpanID(fmt.Sprintf(
		"critical-path:%s:%d:%s",
		node.frame.kind,
		node.firstSequence,
		node.frame.key,
	))
}

func buildCriticalPathFrameTree(
	path *criticalpath.Path[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	hierarchy trace.HierarchyType,
	namer *Namer,
) []*criticalPathFrameNode {
	var roots []*criticalPathFrameNode
	var active []*criticalPathFrameNode
	for idx, element := range path.CriticalPath {
		start := element.Start()
		end := element.End()
		if end <= start {
			continue
		}
		displayEnd := end
		nextElementStart := time.Duration(0)
		hasGapToNext := false
		if idx+1 < len(path.CriticalPath) {
			nextElementStart = path.CriticalPath[idx+1].Start()
			hasGapToNext = nextElementStart > end
			if hasGapToNext {
				displayEnd = nextElementStart
			}
		}
		frames := criticalPathFrames(element, hierarchy, namer)
		active = resizeCriticalPathActiveFrames(active, len(frames))
		for depth, frame := range frames {
			var parent *criticalPathFrameNode
			if depth > 0 {
				parent = active[depth-1]
			}
			node := active[depth]
			if node == nil ||
				node.parent != parent ||
				node.frame.key != frame.key ||
				node.frame.kind != frame.kind ||
				node.end != start {
				node = &criticalPathFrameNode{
					frame:         frame,
					start:         start,
					end:           displayEnd,
					firstSequence: idx,
					lastSequence:  idx,
					parent:        parent,
				}
				if parent == nil {
					roots = append(roots, node)
				} else {
					parent.children = append(parent.children, node)
				}
			} else {
				node.end = displayEnd
				node.lastSequence = idx
			}
			active[depth] = node
		}
		if hasGapToNext && !sameCriticalPathSpan(element.Span(), path.CriticalPath[idx+1].Span(), namer) {
			leafNode := active[len(active)-1]
			leafNode.gaps = append(leafNode.gaps, criticalPathNoLongerRunningGap{
				start: end,
				end:   nextElementStart,
			})
		}
	}
	return roots
}

func resizeCriticalPathActiveFrames(active []*criticalPathFrameNode, length int) []*criticalPathFrameNode {
	if len(active) >= length {
		return active[:length]
	}
	for len(active) < length {
		active = append(active, nil)
	}
	return active
}

func criticalPathFrames(
	element criticalpath.PathElement[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	hierarchy trace.HierarchyType,
	namer *Namer,
) []criticalPathFrame {
	span := element.Span()
	if span == nil {
		return nil
	}
	var ret []criticalPathFrame
	rootSpan := span.RootSpan()
	if rootSpan != nil {
		var categories []trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
		for category := rootSpan.ParentCategory(hierarchy); category != nil; category = category.Parent() {
			categories = append(categories, category)
		}
		for idx := len(categories) - 1; idx >= 0; idx-- {
			category := categories[idx]
			payload := category.Payload()
			if payload == nil {
				payload = &CategoryPayload{}
			}
			name := payload.Name
			if name == "" {
				name = namer.CategoryName(category)
			}
			ret = append(ret, criticalPathFrame{
				key:             "category:" + string(rendertrace.DefaultCategoryID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](category, namer)),
				kind:            criticalPathCategoryFrameKind,
				category:        category,
				serviceName:     payload.ServiceName,
				name:            name,
				categoryPayload: payload,
			})
		}
	}

	var spans []trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	for cursor := span; cursor != nil; cursor = cursor.ParentSpan() {
		spans = append(spans, cursor)
	}
	for idx := len(spans) - 1; idx >= 0; idx-- {
		stackSpan := spans[idx]
		payload := stackSpan.Payload()
		if payload == nil {
			payload = &SpanPayload{}
		}
		serviceName := payload.ServiceName
		if serviceName == "" {
			serviceName = "unknown service"
		}
		spanID := payload.SpanID
		if spanID == "" {
			spanID = namer.SpanName(stackSpan)
		}
		ret = append(ret, criticalPathFrame{
			key:         "span:" + string(rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](stackSpan, namer)),
			kind:        criticalPathLeafFrameKind,
			span:        stackSpan,
			serviceName: serviceName,
			name:        spanID,
			payload:     payload,
		})
	}
	return ret
}

func criticalPathNodeVisible(node *criticalPathFrameNode, domain rendertrace.TimeRange) bool {
	return node.end > domain.Start && node.start < domain.End
}

func sameCriticalPathSpan(
	left trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	right trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	namer *Namer,
) bool {
	if left == nil || right == nil {
		return left == right
	}
	leftPayload := left.Payload()
	rightPayload := right.Payload()
	if leftPayload != nil && rightPayload != nil && leftPayload.SpanID != "" && rightPayload.SpanID != "" {
		return leftPayload.SpanID == rightPayload.SpanID
	}
	return namer.SpanName(left) == namer.SpanName(right)
}

type renderedCausalEventKind string

const (
	// renderedIncomingDependencyEvent marks a dependency destination endpoint:
	// the point at which this span can proceed because another span resolved a
	// causal condition.
	renderedIncomingDependencyEvent renderedCausalEventKind = "incoming_dependency"
	// renderedOutgoingDependencyEvent marks a dependency origin endpoint: the
	// point at which this span resolves a causal condition for another span.
	renderedOutgoingDependencyEvent renderedCausalEventKind = "outgoing_dependency"
	// renderedMarkEvent marks a Tracey label-only event within a span.
	renderedMarkEvent renderedCausalEventKind = "mark"
)

type renderedCausalEventStyle struct {
	displayName string
	lightColor  string
	darkColor   string
}

var renderedCausalEventStyles = map[renderedCausalEventKind]renderedCausalEventStyle{
	renderedIncomingDependencyEvent: {
		displayName: "Incoming dependency",
		lightColor:  "#2166ac",
		darkColor:   "#38bdf8",
	},
	renderedOutgoingDependencyEvent: {
		displayName: "Outgoing dependency",
		lightColor:  "#b2182b",
		darkColor:   "#fb7185",
	},
	renderedMarkEvent: {
		displayName: "Mark",
		lightColor:  "#4d9221",
		darkColor:   "#a3e635",
	},
}

func (style renderedCausalEventStyle) color(theme rendertrace.Theme) string {
	if theme == rendertrace.ThemeDark {
		return style.darkColor
	}
	return style.lightColor
}

type renderPalette struct {
	realSpan                   string
	searchMatch                string
	stroke                     string
	criticalPathCategory       string
	criticalPathEdge           string
	focusDependencyEdge        string
	suspendOverlay             string
	criticalPathGapOverlay     string
	serviceSaturation          int
	serviceLightness           int
	heatmapBaseSaturation      int
	heatmapAddedSaturation     int
	heatmapBaseLightness       int
	heatmapSubtractedLightness int
}

func paletteForTheme(theme rendertrace.Theme) renderPalette {
	if theme == rendertrace.ThemeDark {
		return renderPalette{
			realSpan:                   "#93c5fd",
			searchMatch:                "#fb923c",
			stroke:                     "#111827",
			criticalPathCategory:       "#cbd5e1",
			criticalPathEdge:           "#c084fc",
			focusDependencyEdge:        "#2dd4bf",
			suspendOverlay:             "rgba(15, 23, 42, 0.56)",
			criticalPathGapOverlay:     "rgba(2, 6, 23, 0.68)",
			serviceSaturation:          58,
			serviceLightness:           72,
			heatmapBaseSaturation:      42,
			heatmapAddedSaturation:     30,
			heatmapBaseLightness:       36,
			heatmapSubtractedLightness: -28,
		}
	}
	return renderPalette{
		realSpan:                   "#9ecae1",
		searchMatch:                "#f97316",
		stroke:                     "#202124",
		criticalPathCategory:       "#e8eaed",
		criticalPathEdge:           "#3730a3",
		focusDependencyEdge:        "#0f766e",
		suspendOverlay:             "rgba(128, 128, 128, 0.48)",
		criticalPathGapOverlay:     "rgba(70, 70, 70, 0.58)",
		serviceSaturation:          32,
		serviceLightness:           85,
		heatmapBaseSaturation:      18,
		heatmapAddedSaturation:     22,
		heatmapBaseLightness:       96,
		heatmapSubtractedLightness: 18,
	}
}

type renderedCausalEvent struct {
	kind           renderedCausalEventKind
	moment         time.Duration
	sequence       int
	displayName    string
	label          string
	dependencyType string
	dependencyKey  string
	otherSpanID    string
	detail         string
}

type renderedCausalEventChip struct {
	Start time.Duration
	End   time.Duration
	Event renderedCausalEvent
}

type syntheticServiceSpanView struct {
	categoryPayload *CategoryPayload
	concurrencyMap  *concurrency.Profile
	namer           *Namer
}

func (sv syntheticServiceSpanView) ID() rendertrace.SpanID {
	return rendertrace.SpanID("synthetic-category:" + sv.categoryPayload.ID)
}

func (sv syntheticServiceSpanView) TimeRange() rendertrace.TimeRange {
	return rendertrace.TimeRange{
		Start: time.Duration(sv.concurrencyMap.Start),
		End:   time.Duration(sv.concurrencyMap.End),
	}
}

func (sv syntheticServiceSpanView) ChildSpans(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.SpanView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (sv syntheticServiceSpanView) RenderTraceVizSpan(
	ctx context.Context,
	view rendertrace.RenderView,
	parent rendertrace.TraceVizSpanParent,
) (*tvtrace.Span[time.Duration], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	spanRange := sv.TimeRange()
	serviceName := sv.categoryPayload.ServiceName
	if serviceName == "" {
		serviceName = sv.categoryPayload.Name
	}
	palette := paletteForTheme(view.Request.Theme)
	serviceColor := serviceColor(serviceName, view)
	syntheticTooltip := tooltipText(
		serviceName,
		"Kind: synthetic_service",
		tooltipLine("Service", serviceName),
		fmt.Sprintf("Peak concurrency: %d", sv.concurrencyMap.Peak),
		durationTooltipLine(spanRange.Start, spanRange.End),
	)
	renderedSpan := parent.Span(
		spanRange.Start,
		spanRange.End,
		label.Format(""),
		color.Primary(serviceColor),
		color.Secondary(serviceColor),
		color.Stroke(palette.stroke),
		util.StringProperty("span_kind", "synthetic_service"),
		util.StringProperty("span_name", serviceName),
		util.StringProperty("service_name", serviceName),
		util.StringProperty("category_id", sv.categoryPayload.ID),
		util.IntegerProperty("concurrency_peak", int64(sv.concurrencyMap.Peak)),
		util.StringProperty("tooltip", syntheticTooltip),
	)
	addTraceOverlayNodes(renderedSpan, view, sv.ID())
	for _, bucket := range renderableConcurrencyBuckets(sv.concurrencyMap, serviceName, spanRange, view) {
		bucketTooltip := tooltipText(
			serviceName,
			"Kind: concurrency_heatmap",
			tooltipLine("Service", serviceName),
			fmt.Sprintf("Avg concurrency: %.2f", bucket.Avg),
			fmt.Sprintf("Peak concurrency: %d", bucket.Peak),
			durationTooltipLine(bucket.Start, bucket.End),
		)
		renderedSpan.Subspan(
			bucket.Start,
			bucket.End,
			label.Format(""),
			color.Primary(bucket.Color),
			color.Secondary(bucket.Color),
			color.Stroke(bucket.Color),
			util.StringProperty("subspan_kind", "concurrency_heatmap"),
			util.StringProperty("span_kind", "synthetic_service"),
			util.StringProperty("span_name", serviceName),
			util.StringProperty("service_name", serviceName),
			util.StringProperty("category_id", sv.categoryPayload.ID),
			util.DoubleProperty("concurrency_avg", bucket.Avg),
			util.IntegerProperty("concurrency_peak", int64(bucket.Peak)),
			util.StringProperty("tooltip", bucketTooltip),
		)
	}
	return renderedSpan, nil
}

type renderableConcurrencyBucket struct {
	Start time.Duration
	End   time.Duration
	Color string
	Avg   float64
	Peak  int
}

func renderableConcurrencyBuckets(
	profile *concurrency.Profile,
	serviceName string,
	spanRange rendertrace.TimeRange,
	view rendertrace.RenderView,
) []renderableConcurrencyBucket {
	if profile == nil {
		return nil
	}
	var ret []renderableConcurrencyBucket
	for _, bucket := range profile.Buckets(concurrency.BucketOptions{
		Domain: concurrency.Range{
			Start: float64(view.TemporalDomain.Start),
			End:   float64(view.TemporalDomain.End),
		},
		Clip: concurrency.Range{
			Start: float64(spanRange.Start),
			End:   float64(spanRange.End),
		},
		WidthPx:               view.Request.TraceViewRangePx,
		MinimumFeatureWidthPx: view.Request.MinimumFeatureWidthPx,
	}) {
		intensity := 0.0
		if profile.Peak > 0 {
			intensity = bucket.Avg / float64(profile.Peak)
		}
		next := renderableConcurrencyBucket{
			Start: time.Duration(bucket.Start),
			End:   time.Duration(bucket.End),
			Color: serviceHeatmapColor(serviceName, intensity, view),
			Avg:   bucket.Avg,
			Peak:  bucket.Peak,
		}
		if len(ret) > 0 {
			last := &ret[len(ret)-1]
			if last.Color == next.Color && last.End == next.Start {
				mergeConcurrencyBucket(last, next)
				continue
			}
		}
		ret = append(ret, next)
	}
	return ret
}

func mergeConcurrencyBucket(into *renderableConcurrencyBucket, next renderableConcurrencyBucket) {
	intoDuration := float64(into.End - into.Start)
	nextDuration := float64(next.End - next.Start)
	totalDuration := intoDuration + nextDuration
	if totalDuration > 0 {
		into.Avg = (into.Avg*intoDuration + next.Avg*nextDuration) / totalDuration
	}
	into.End = next.End
	if next.Peak > into.Peak {
		into.Peak = next.Peak
	}
}

func (sv spanView) ID() rendertrace.SpanID {
	return rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
		sv.span,
		sv.namer,
	)
}

func (sv spanView) TimeRange() rendertrace.TimeRange {
	return rendertrace.TimeRange{
		Start: sv.span.Start(),
		End:   sv.span.End(),
	}
}

func (sv spanView) ChildSpans(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.SpanView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var ret []rendertrace.SpanView
	for _, child := range sv.span.ChildSpans() {
		if !spanVisible(child, view, sv.namer) {
			continue
		}
		ret = append(ret, spanView{
			span:  child,
			namer: sv.namer,
		})
	}
	sort.SliceStable(ret, func(i, j int) bool {
		left := ret[i].TimeRange()
		right := ret[j].TimeRange()
		if left.Start != right.Start {
			return left.Start < right.Start
		}
		return ret[i].ID() < ret[j].ID()
	})
	return ret, nil
}

func categorySubtreeContainsFocusedSpan(
	category trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	view rendertrace.RenderView,
) bool {
	if len(view.Request.FocusSpanIDs) == 0 {
		return true
	}
	for _, rootSpan := range category.RootSpans() {
		if spanSubtreeContainsFocusedSpan(rootSpan, view) {
			return true
		}
	}
	for _, child := range category.ChildCategories() {
		if categorySubtreeContainsFocusedSpan(child, view) {
			return true
		}
	}
	return false
}

func categoryMatchesSearch(categoryID rendertrace.CategoryID, view rendertrace.RenderView) bool {
	if view.SearchResult == nil {
		return false
	}
	_, ok := view.SearchResult.Categories[categoryID]
	return ok
}

func categoryContainsSearchMatch(categoryID rendertrace.CategoryID, view rendertrace.RenderView) bool {
	if view.SearchResult == nil {
		return false
	}
	_, ok := view.SearchResult.CategoriesWithDescendantMatch[categoryID]
	return ok
}

func spanMatchesSearch(spanID rendertrace.SpanID, view rendertrace.RenderView) bool {
	if view.SearchResult == nil {
		return false
	}
	_, ok := view.SearchResult.Spans[spanID]
	return ok
}

func spanContainsSearchMatch(spanID rendertrace.SpanID, view rendertrace.RenderView) bool {
	if view.SearchResult == nil {
		return false
	}
	_, ok := view.SearchResult.SpansWithDescendantMatch[spanID]
	return ok
}

func spanPrimaryColor(spanID rendertrace.SpanID, view rendertrace.RenderView) string {
	palette := paletteForTheme(view.Request.Theme)
	if spanMatchesSearch(spanID, view) {
		return palette.searchMatch
	}
	return palette.realSpan
}

func markSpanCategoryAncestors(
	searchResult *rendertrace.SearchResult,
	category trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	namer *Namer,
) {
	for cursor := category; cursor != nil; cursor = cursor.Parent() {
		categoryID := rendertrace.DefaultCategoryID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
			cursor,
			namer,
		)
		searchResult.CategoriesWithDescendantMatch[categoryID] = struct{}{}
	}
}

func markSpanAncestors(
	searchResult *rendertrace.SearchResult,
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	namer *Namer,
) {
	for cursor := span.ParentSpan(); cursor != nil; cursor = cursor.ParentSpan() {
		spanID := rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
			cursor,
			namer,
		)
		searchResult.SpansWithDescendantMatch[spanID] = struct{}{}
	}
}

func categorySubtreeOverlapsTimeRange(
	category trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	timeRange rendertrace.TimeRange,
) bool {
	for _, rootSpan := range category.RootSpans() {
		if spanSubtreeOverlapsTimeRange(rootSpan, timeRange) {
			return true
		}
	}
	for _, child := range category.ChildCategories() {
		if categorySubtreeOverlapsTimeRange(child, timeRange) {
			return true
		}
	}
	return false
}

func spanSubtreeOverlapsTimeRange(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	timeRange rendertrace.TimeRange,
) bool {
	if spanOverlapsTimeRange(span, timeRange) {
		return true
	}
	for _, child := range span.ChildSpans() {
		if spanSubtreeOverlapsTimeRange(child, timeRange) {
			return true
		}
	}
	return false
}

func spanOverlapsTimeRange(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	timeRange rendertrace.TimeRange,
) bool {
	return span.End() > timeRange.Start && span.Start() < timeRange.End
}

func categoryVisible(
	category trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	view rendertrace.RenderView,
	namer *Namer,
) bool {
	if !categorySubtreeContainsFocusedSpan(category, view) {
		return false
	}
	if view.Request.HideEmptyCategories && !categorySubtreeOverlapsTimeRange(category, view.TemporalDomain) {
		return false
	}
	categoryID := rendertrace.DefaultCategoryID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
		category,
		namer,
	)
	if view.Request.DisplayMode == rendertrace.DisplayOnlyMatches {
		if !categoryMatchesSearch(categoryID, view) && !categoryContainsSearchMatch(categoryID, view) {
			return false
		}
	}
	if view.Request.ShowOnlyCriticalPath && len(view.Request.FocusSpanIDs) == 0 {
		if !view.CriticalPathVisibility.ContainsCategory(categoryID) {
			return false
		}
	}
	return true
}

func spanVisible(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	view rendertrace.RenderView,
	namer *Namer,
) bool {
	if !spanSubtreeContainsFocusedSpan(span, view) {
		return false
	}
	if view.Request.HideEmptyCategories && !spanSubtreeOverlapsTimeRange(span, view.TemporalDomain) {
		return false
	}
	spanID := rendertrace.DefaultSpanID[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
		span,
		namer,
	)
	if view.Request.DisplayMode == rendertrace.DisplayOnlyMatches {
		if !spanMatchesSearch(spanID, view) && !spanContainsSearchMatch(spanID, view) {
			return false
		}
	}
	if view.Request.ShowOnlyCriticalPath && len(view.Request.FocusSpanIDs) == 0 {
		if !view.CriticalPathVisibility.ContainsSpan(spanID) {
			return false
		}
	}
	return true
}

func spanSubtreeContainsFocusedSpan(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	view rendertrace.RenderView,
) bool {
	if len(view.Request.FocusSpanIDs) == 0 {
		return true
	}
	if spanMatchesFocusedID(span, view.Request.FocusSpanIDs) {
		return true
	}
	for _, child := range span.ChildSpans() {
		if spanSubtreeContainsFocusedSpan(child, view) {
			return true
		}
	}
	return false
}

func spanMatchesFocusedID(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	focusSpanIDs []rendertrace.SpanID,
) bool {
	payload := span.Payload()
	if payload == nil {
		return false
	}
	for _, focusSpanID := range focusSpanIDs {
		if payload.SpanID == string(focusSpanID) {
			return true
		}
	}
	return false
}

func tooltipText(lines ...string) string {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func tooltipLine(label string, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", label, value)
}

func durationTooltipLine(start, end time.Duration) string {
	return fmt.Sprintf("Duration: %s", end-start)
}

func spanIdentityTooltipLines(labelText string, payload *SpanPayload) []string {
	lines := []string{
		labelText,
		tooltipLine("Span ID", payload.SpanID),
		tooltipLine("Service", payload.ServiceName),
		tooltipLine("Process", payload.ProcessID),
	}
	if payload.OperationName != "" && payload.OperationName != labelText {
		lines = append(lines, tooltipLine("Operation", payload.OperationName))
	}
	return lines
}

func (sv spanView) RenderTraceVizSpan(
	ctx context.Context,
	view rendertrace.RenderView,
	parent rendertrace.TraceVizSpanParent,
) (*tvtrace.Span[time.Duration], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	payload := sv.span.Payload()
	if payload == nil {
		return nil, fmt.Errorf("extended OTel span has nil payload")
	}
	labelText := payload.OperationName
	if labelText == "" {
		labelText = payload.SpanID
	}
	spanSuspends := spanSuspendCount(sv.span)
	spanCausalEvents := spanCausalEventCount(sv.span)
	spanTooltipLines := append(
		spanIdentityTooltipLines(labelText, payload),
		fmt.Sprintf("Suspends: %d", spanSuspends),
		fmt.Sprintf("Causal events: %d", spanCausalEvents),
		durationTooltipLine(sv.span.Start(), sv.span.End()),
	)
	palette := paletteForTheme(view.Request.Theme)
	renderedSpan := parent.Span(
		sv.span.Start(),
		sv.span.End(),
		label.Format("$(span_name)"),
		color.Primary(spanPrimaryColor(sv.ID(), view)),
		color.Secondary(spanPrimaryColor(sv.ID(), view)),
		color.Stroke(palette.stroke),
		util.StringProperty("span_id", payload.SpanID),
		util.StringProperty("span_name", labelText),
		util.StringProperty("service_name", payload.ServiceName),
		util.StringProperty("process_id", payload.ProcessID),
		util.StringProperty("operation_name", payload.OperationName),
		util.IntegerProperty("suspend_count", int64(spanSuspends)),
		util.IntegerProperty("causal_event_count", int64(spanCausalEvents)),
		util.StringProperty("tooltip", tooltipText(spanTooltipLines...)),
	)
	addTraceOverlayNodes(renderedSpan, view, sv.ID())
	for _, suspend := range renderableSuspendIntervals(sv.span, view) {
		suspendTooltipLines := append(
			[]string{labelText, "Kind: suspend"},
			spanIdentityTooltipLines(labelText, payload)[1:]...,
		)
		suspendTooltipLines = append(suspendTooltipLines, durationTooltipLine(suspend.Start, suspend.End))
		renderedSpan.Subspan(
			suspend.Start,
			suspend.End,
			color.Primary(palette.suspendOverlay),
			color.Secondary(palette.suspendOverlay),
			color.Stroke(palette.stroke),
			util.StringProperty("subspan_kind", "suspend"),
			util.StringProperty("span_id", payload.SpanID),
			util.StringProperty("span_name", labelText),
			util.StringProperty("service_name", payload.ServiceName),
			util.StringProperty("process_id", payload.ProcessID),
			util.StringProperty("operation_name", payload.OperationName),
			util.StringProperty("tooltip", tooltipText(suspendTooltipLines...)),
		)
	}
	for _, eventChip := range renderableCausalEventChips(sv.span, view, sv.namer) {
		eventStyle := renderedCausalEventStyles[eventChip.Event.kind]
		eventColor := eventStyle.color(view.Request.Theme)
		eventTooltipLines := append(
			[]string{
				labelText,
				"Kind: causal_event",
				tooltipLine("Event", eventChip.Event.displayName),
				tooltipLine("Event type", string(eventChip.Event.kind)),
				tooltipLine("Event time", sv.namer.MomentString(eventChip.Event.moment)),
				tooltipLine("Label", eventChip.Event.label),
				tooltipLine("Dependency", eventChip.Event.dependencyType),
				tooltipLine("Dependency key", eventChip.Event.dependencyKey),
				eventChip.Event.detail,
			},
			spanIdentityTooltipLines(labelText, payload)[1:]...,
		)
		renderedSpan.Subspan(
			eventChip.Start,
			eventChip.End,
			color.Primary(eventColor),
			color.Secondary(eventColor),
			color.Stroke(palette.stroke),
			util.StringProperty("subspan_kind", "causal_event"),
			util.StringProperty("event_type", string(eventChip.Event.kind)),
			util.StringProperty("event_display_name", eventChip.Event.displayName),
			util.StringProperty("event_time", sv.namer.MomentString(eventChip.Event.moment)),
			util.StringProperty("event_label", eventChip.Event.label),
			util.StringProperty("event_dependency_type", eventChip.Event.dependencyType),
			util.StringProperty("event_dependency_key", eventChip.Event.dependencyKey),
			util.StringProperty("event_other_span_id", eventChip.Event.otherSpanID),
			util.StringProperty("event_detail", eventChip.Event.detail),
			util.StringProperty("span_id", payload.SpanID),
			util.StringProperty("span_name", labelText),
			util.StringProperty("service_name", payload.ServiceName),
			util.StringProperty("process_id", payload.ProcessID),
			util.StringProperty("operation_name", payload.OperationName),
			util.StringProperty("tooltip", tooltipText(eventTooltipLines...)),
		)
	}
	return renderedSpan, nil
}

func addTraceOverlayNodes(
	renderedSpan *tvtrace.Span[time.Duration],
	view rendertrace.RenderView,
	spanID rendertrace.SpanID,
) {
	palette := paletteForTheme(view.Request.Theme)
	for _, node := range view.CriticalPathOverlay.NodesForSpan(spanID) {
		traceedge.New(
			view.TemporalAxis,
			renderedSpan,
			node.Moment,
			node.ID,
			node.EndpointNodeIDs...,
		).With(
			color.Stroke(palette.criticalPathEdge),
			color.Secondary(palette.criticalPathEdge),
			util.StringProperty("trace_edge_kind", "critical_path"),
			util.StringProperty("critical_path_strategy", view.Request.ResolvedCriticalPathStrategy()),
		)
	}
	for _, node := range view.FocusDependencyOverlay.NodesForSpan(spanID) {
		traceedge.New(
			view.TemporalAxis,
			renderedSpan,
			node.Moment,
			node.ID,
			node.EndpointNodeIDs...,
		).With(
			color.Stroke(palette.focusDependencyEdge),
			color.Secondary(palette.focusDependencyEdge),
			util.StringProperty("trace_edge_kind", "focus_dependency"),
		)
	}
}

func spanSuspendCount(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) int {
	return len(suspendIntervals(span, rendertrace.TimeRange{
		Start: span.Start(),
		End:   span.End(),
	}))
}

func spanCausalEventCount(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) int {
	return len(spanCausalEvents(span, nil))
}

func renderableCausalEventChips(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	view rendertrace.RenderView,
	namer *Namer,
) []renderedCausalEventChip {
	events := spanCausalEvents(span, namer)
	if len(events) == 0 {
		return nil
	}
	domain := view.TemporalDomain
	if domain.End <= domain.Start || view.Request.TraceViewRangePx <= 0 {
		return nil
	}
	minFeatureWidthPx := view.Request.MinimumFeatureWidthPx
	if minFeatureWidthPx <= 0 {
		minFeatureWidthPx = 1
	}
	bucketCount := int(math.Ceil(float64(view.Request.TraceViewRangePx) / minFeatureWidthPx))
	if bucketCount <= 0 {
		return nil
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].moment != events[j].moment {
			return events[i].moment < events[j].moment
		}
		if events[i].kind != events[j].kind {
			return eventRenderPriority(events[i].kind) < eventRenderPriority(events[j].kind)
		}
		return events[i].sequence < events[j].sequence
	})

	selectedByBucket := map[int]renderedCausalEvent{}
	for _, event := range events {
		if event.moment < domain.Start || event.moment > domain.End {
			continue
		}
		bucket := bucketIndexFloor(event.moment, domain, bucketCount)
		if _, ok := selectedByBucket[bucket]; ok {
			continue
		}
		selectedByBucket[bucket] = event
	}
	if len(selectedByBucket) == 0 {
		return nil
	}

	domainDuration := domain.End - domain.Start
	var ret []renderedCausalEventChip
	for idx := 0; idx < bucketCount; idx++ {
		event, ok := selectedByBucket[idx]
		if !ok {
			continue
		}
		start := domain.Start + time.Duration(float64(domainDuration)*float64(idx)/float64(bucketCount))
		end := domain.Start + time.Duration(float64(domainDuration)*float64(idx+1)/float64(bucketCount))
		if idx == bucketCount-1 {
			end = domain.End
		}
		start = maxDuration(start, maxDuration(span.Start(), domain.Start))
		end = minDuration(end, minDuration(span.End(), domain.End))
		if end > start {
			ret = append(ret, renderedCausalEventChip{
				Start: start,
				End:   end,
				Event: event,
			})
		}
	}
	return ret
}

func spanCausalEvents(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	namer *Namer,
) []renderedCausalEvent {
	var ret []renderedCausalEvent
	for _, elementarySpan := range span.ElementarySpans() {
		if incoming := elementarySpan.Incoming(); incoming != nil {
			ret = append(ret, dependencyEndpointEvent(
				elementarySpan,
				incoming,
				renderedIncomingDependencyEvent,
				elementarySpan.Start(),
				len(ret),
				namer,
			))
		}
		for _, mark := range elementarySpan.Marks() {
			style := renderedCausalEventStyles[renderedMarkEvent]
			ret = append(ret, renderedCausalEvent{
				kind:        renderedMarkEvent,
				moment:      mark.Moment(),
				sequence:    len(ret),
				displayName: style.displayName,
				label:       mark.Label(),
			})
		}
		if outgoing := elementarySpan.Outgoing(); outgoing != nil {
			ret = append(ret, dependencyEndpointEvent(
				elementarySpan,
				outgoing,
				renderedOutgoingDependencyEvent,
				elementarySpan.End(),
				len(ret),
				namer,
			))
		}
	}
	return ret
}

func dependencyEndpointEvent(
	elementarySpan trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	dependency trace.Dependency[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	kind renderedCausalEventKind,
	moment time.Duration,
	sequence int,
	namer *Namer,
) renderedCausalEvent {
	style := renderedCausalEventStyles[kind]
	payload := dependency.Payload()
	ret := renderedCausalEvent{
		kind:           kind,
		moment:         moment,
		sequence:       sequence,
		displayName:    style.displayName,
		dependencyType: dependencyTypeName(dependency.DependencyType(), namer),
		otherSpanID:    dependencyOtherSpanID(elementarySpan, dependency, kind),
		detail:         dependencyEndpointDetail(elementarySpan, dependency, kind),
	}
	if payload != nil {
		ret.dependencyKey = payload.Key
	}
	return ret
}

func dependencyOtherSpanID(
	elementarySpan trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	dependency trace.Dependency[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	kind renderedCausalEventKind,
) string {
	var otherEndpoint trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	switch kind {
	case renderedIncomingDependencyEvent:
		otherEndpoint = dependency.TriggeringOrigin()
	case renderedOutgoingDependencyEvent:
		otherEndpoint = earliestDestinationExcluding(elementarySpan, dependency.Destinations())
	}
	if otherEndpoint == nil || otherEndpoint.Span().Payload() == nil {
		return ""
	}
	return otherEndpoint.Span().Payload().SpanID
}

func dependencyTypeName(dependencyType trace.DependencyType, namer *Namer) string {
	if namer != nil {
		if typeData := namer.DependencyTypes().TypeData(dependencyType); typeData != nil {
			return typeData.Name
		}
	}
	return fmt.Sprintf("dependency %d", dependencyType)
}

func dependencyEndpointDetail(
	elementarySpan trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	dependency trace.Dependency[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	kind renderedCausalEventKind,
) string {
	switch kind {
	case renderedIncomingDependencyEvent:
		return dependencyOriginDetail(dependency.TriggeringOrigin())
	case renderedOutgoingDependencyEvent:
		return dependencyDestinationDetail(elementarySpan, dependency.Destinations())
	default:
		return ""
	}
}

func dependencyOriginDetail(
	origin trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) string {
	if origin == nil {
		return ""
	}
	payload := origin.Span().Payload()
	if payload == nil {
		return ""
	}
	return fmt.Sprintf("from %s", spanPayloadDisplayName(payload))
}

func dependencyDestinationDetail(
	origin trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	destinations []trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) string {
	earliestDestination := earliestDestinationExcluding(origin, destinations)
	if earliestDestination == nil {
		return ""
	}
	payload := earliestDestination.Span().Payload()
	if payload == nil {
		return ""
	}
	detail := fmt.Sprintf("to %s", spanPayloadDisplayName(payload))
	if len(destinations) > 1 {
		detail = fmt.Sprintf("%s (+%d destinations)", detail, len(destinations)-1)
	}
	return detail
}

func earliestDestinationExcluding(
	origin trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	destinations []trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	var earliestDestination trace.ElementarySpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	for _, destination := range destinations {
		if destination == nil || destination == origin {
			continue
		}
		if earliestDestination == nil || destination.Start() < earliestDestination.Start() {
			earliestDestination = destination
		}
	}
	return earliestDestination
}

func spanPayloadDisplayName(payload *SpanPayload) string {
	displayName := payload.OperationName
	if displayName == "" {
		displayName = payload.SpanID
	}
	if payload.ServiceName != "" {
		return fmt.Sprintf("%s / %s", payload.ServiceName, displayName)
	}
	return displayName
}

func eventRenderPriority(kind renderedCausalEventKind) int {
	switch kind {
	case renderedIncomingDependencyEvent:
		return 0
	case renderedMarkEvent:
		return 1
	case renderedOutgoingDependencyEvent:
		return 2
	default:
		return 3
	}
}

func renderableSuspendIntervals(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	view rendertrace.RenderView,
) []rendertrace.TimeRange {
	return renderableSuspendIntervalsInDomain(span, view, view.TemporalDomain)
}

func renderableSuspendIntervalsInDomain(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	view rendertrace.RenderView,
	domain rendertrace.TimeRange,
) []rendertrace.TimeRange {
	domain.Start = maxDuration(domain.Start, view.TemporalDomain.Start)
	domain.End = minDuration(domain.End, view.TemporalDomain.End)
	rawSuspends := suspendIntervals(span, domain)
	if len(rawSuspends) == 0 {
		return nil
	}
	if domain.End <= domain.Start || view.Request.TraceViewRangePx <= 0 {
		return nil
	}
	minFeatureWidthPx := view.Request.MinimumFeatureWidthPx
	if minFeatureWidthPx <= 0 {
		minFeatureWidthPx = 1
	}
	bucketCount := int(math.Ceil(float64(view.Request.TraceViewRangePx) / minFeatureWidthPx))
	if bucketCount <= 0 {
		return nil
	}
	bucketCoverage := make([]time.Duration, bucketCount)
	domainDuration := domain.End - domain.Start
	for _, suspend := range rawSuspends {
		startIdx := bucketIndexFloor(suspend.Start, domain, bucketCount)
		endIdx := bucketIndexCeil(suspend.End, domain, bucketCount) - 1
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx >= bucketCount {
			endIdx = bucketCount - 1
		}
		for idx := startIdx; idx <= endIdx; idx++ {
			bucketStart := domain.Start + time.Duration(float64(domainDuration)*float64(idx)/float64(bucketCount))
			bucketEnd := domain.Start + time.Duration(float64(domainDuration)*float64(idx+1)/float64(bucketCount))
			if idx == bucketCount-1 {
				bucketEnd = domain.End
			}
			overlapStart := maxDuration(suspend.Start, bucketStart)
			overlapEnd := minDuration(suspend.End, bucketEnd)
			if overlapEnd > overlapStart {
				bucketCoverage[idx] += overlapEnd - overlapStart
			}
		}
	}
	var ret []rendertrace.TimeRange
	var runStart = -1
	for idx, coverage := range bucketCoverage {
		bucketStart := domain.Start + time.Duration(float64(domainDuration)*float64(idx)/float64(bucketCount))
		bucketEnd := domain.Start + time.Duration(float64(domainDuration)*float64(idx+1)/float64(bucketCount))
		if idx == bucketCount-1 {
			bucketEnd = domain.End
		}
		isSuspended := coverage > (bucketEnd-bucketStart)/2
		if isSuspended && runStart < 0 {
			runStart = idx
		}
		if (!isSuspended || idx == bucketCount-1) && runStart >= 0 {
			runEnd := idx
			if !isSuspended {
				runEnd = idx - 1
			}
			start := domain.Start + time.Duration(float64(domainDuration)*float64(runStart)/float64(bucketCount))
			end := domain.Start + time.Duration(float64(domainDuration)*float64(runEnd+1)/float64(bucketCount))
			if runEnd == bucketCount-1 {
				end = domain.End
			}
			start = maxDuration(start, maxDuration(span.Start(), domain.Start))
			end = minDuration(end, minDuration(span.End(), domain.End))
			if end > start {
				ret = append(ret, rendertrace.TimeRange{Start: start, End: end})
			}
			runStart = -1
		}
	}
	return ret
}

func suspendIntervals(
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	domain rendertrace.TimeRange,
) []rendertrace.TimeRange {
	elementarySpans := span.ElementarySpans()
	if len(elementarySpans) == 0 {
		return nil
	}
	var ret []rendertrace.TimeRange
	cursor := span.Start()
	for _, elementarySpan := range elementarySpans {
		if elementarySpan.Start() > cursor {
			appendClippedSuspend(&ret, cursor, elementarySpan.Start(), domain)
		}
		if elementarySpan.End() > cursor {
			cursor = elementarySpan.End()
		}
	}
	if span.End() > cursor {
		appendClippedSuspend(&ret, cursor, span.End(), domain)
	}
	return ret
}

func appendClippedSuspend(ret *[]rendertrace.TimeRange, start, end time.Duration, domain rendertrace.TimeRange) {
	start = maxDuration(start, domain.Start)
	end = minDuration(end, domain.End)
	if end > start {
		*ret = append(*ret, rendertrace.TimeRange{Start: start, End: end})
	}
}

func clampDuration(moment time.Duration, domain rendertrace.TimeRange) time.Duration {
	if moment < domain.Start {
		return domain.Start
	}
	if moment > domain.End {
		return domain.End
	}
	return moment
}

func durationInRange(moment time.Duration, domain rendertrace.TimeRange) bool {
	return moment >= domain.Start && moment <= domain.End
}

func bucketIndexFloor(moment time.Duration, domain rendertrace.TimeRange, bucketCount int) int {
	if moment <= domain.Start {
		return 0
	}
	if moment >= domain.End {
		return bucketCount - 1
	}
	fraction := float64(moment-domain.Start) / float64(domain.End-domain.Start)
	return int(math.Floor(fraction * float64(bucketCount)))
}

func bucketIndexCeil(moment time.Duration, domain rendertrace.TimeRange, bucketCount int) int {
	if moment <= domain.Start {
		return 0
	}
	if moment >= domain.End {
		return bucketCount
	}
	fraction := float64(moment-domain.Start) / float64(domain.End-domain.Start)
	return int(math.Ceil(fraction * float64(bucketCount)))
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func maxDuration(left, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}

func serviceColor(serviceName string, view rendertrace.RenderView) string {
	palette := paletteForTheme(view.Request.Theme)
	return fmt.Sprintf(
		"hsl(%d, %d%%, %d%%)",
		serviceHue(serviceName),
		palette.serviceSaturation,
		palette.serviceLightness,
	)
}

func serviceHeatmapColor(serviceName string, intensity float64, view rendertrace.RenderView) string {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}
	palette := paletteForTheme(view.Request.Theme)
	saturation := palette.heatmapBaseSaturation + int(math.Round(float64(palette.heatmapAddedSaturation)*intensity))
	lightness := palette.heatmapBaseLightness - int(math.Round(float64(palette.heatmapSubtractedLightness)*intensity))
	return fmt.Sprintf("hsl(%d, %d%%, %d%%)", serviceHue(serviceName), saturation, lightness)
}

func serviceHue(serviceName string) uint32 {
	if serviceName == "" {
		serviceName = "unknown-service"
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(serviceName))
	hash := hasher.Sum32()
	return hash % 360
}
