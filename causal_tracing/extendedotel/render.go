package extendedotel

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
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

// StackTypes returns the stack policies this trace can render. None are
// supported yet.
func (t *Trace) StackTypes() *rendertrace.StackTypes {
	return rendertrace.NewStackTypes()
}

// Search evaluates a render search. Search rendering is not implemented yet.
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
		if !categorySubtreeContainsFocusedSpan(category, view) {
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
		path:  path,
		namer: t.namer,
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
		if syntheticID, ok := syntheticServiceSpanID(category); ok {
			return syntheticID, true
		}
		return "", false
	}
	return spanView{span: span, namer: t.namer}.ID(), true
}

func syntheticServiceSpanID(
	category trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) (rendertrace.SpanID, bool) {
	payload := category.Payload()
	if payload == nil ||
		payload.HierarchyType != ServiceHierarchyType ||
		category.Parent() != nil ||
		payload.ID == "" {
		return "", false
	}
	return rendertrace.SpanID("synthetic-service:" + payload.ID), true
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
		if !categorySubtreeContainsFocusedSpan(child, view) {
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
		if synthetic := cv.syntheticServiceSpan(view); synthetic != nil {
			ret = append(ret, synthetic)
		}
		return ret, nil
	}
	for _, rootSpan := range cv.category.RootSpans() {
		if !spanSubtreeContainsFocusedSpan(rootSpan, view) {
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

func (cv categoryView) syntheticServiceSpan(view rendertrace.RenderView) rendertrace.SpanView {
	if len(view.Request.FocusSpanIDs) > 0 {
		return nil
	}
	payload := cv.category.Payload()
	if payload == nil ||
		payload.HierarchyType != ServiceHierarchyType ||
		cv.category.Parent() != nil ||
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
	categoryColor := serviceColor(payload.ServiceName)
	primaryColor := categoryColor
	if categoryMatchesSearch(cv.ID(), view) {
		primaryColor = searchMatchColor
	}
	secondaryColor := categoryColor
	if categoryContainsSearchMatch(cv.ID(), view) {
		secondaryColor = searchMatchColor
	}
	expansionState := cv.categoryExpansionState(view)
	return parent.Category(
		category.New(payload.ID, payload.Name, payload.Name),
		label.Format("$(category_label)"),
		color.Primary(primaryColor),
		color.Secondary(secondaryColor),
		color.Stroke("#202124"),
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
	path  *criticalpath.Path[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	namer *Namer
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
	ret := make([]rendertrace.SpanView, 0, len(cv.path.CriticalPath))
	for idx, element := range cv.path.CriticalPath {
		if element.End() <= view.TemporalDomain.Start || element.Start() >= view.TemporalDomain.End {
			continue
		}
		start := maxDuration(element.Start(), view.TemporalDomain.Start)
		end := minDuration(element.End(), view.TemporalDomain.End)
		if end <= start {
			continue
		}
		ret = append(ret, criticalPathFrameSpanView{
			element:  element,
			sequence: idx,
			start:    start,
			end:      end,
			namer:    cv.namer,
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
	const categoryColor = "#e8eaed"
	return parent.Category(
		category.New(string(cv.ID()), "Temporal critical path", "Temporal critical path"),
		label.Format("$(category_label)"),
		color.Primary(categoryColor),
		color.Secondary(categoryColor),
		color.Stroke("#202124"),
		util.StringProperty("category_id", string(cv.ID())),
		util.StringProperty("category_name", "Temporal critical path"),
		util.StringProperty("category_label", "Temporal critical path"),
		util.StringProperty("category_expansion_state", "leaf"),
		util.StringProperty("critical_path_strategy", view.Request.ResolvedCriticalPathStrategy()),
	), nil
}

type criticalPathFrameSpanView struct {
	element  criticalpath.PathElement[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	sequence int
	start    time.Duration
	end      time.Duration
	namer    *Namer
}

func (sv criticalPathFrameSpanView) ID() rendertrace.SpanID {
	return rendertrace.SpanID(fmt.Sprintf("critical-path:frame:%d:%s", sv.sequence, sv.spanID()))
}

func (sv criticalPathFrameSpanView) TimeRange() rendertrace.TimeRange {
	return rendertrace.TimeRange{Start: sv.start, End: sv.end}
}

func (sv criticalPathFrameSpanView) ChildSpans(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.SpanView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []rendertrace.SpanView{criticalPathLeafSpanView(sv)}, nil
}

func (sv criticalPathFrameSpanView) RenderTraceVizSpan(
	ctx context.Context,
	view rendertrace.RenderView,
	parent rendertrace.TraceVizSpanParent,
) (*tvtrace.Span[time.Duration], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	payload := sv.payload()
	serviceName := payload.ServiceName
	if serviceName == "" {
		serviceName = "unknown service"
	}
	serviceColor := serviceColor(serviceName)
	return parent.Span(
		sv.start,
		sv.end,
		label.Format("$(span_name)"),
		color.Primary(serviceColor),
		color.Secondary(serviceColor),
		color.Stroke("#202124"),
		util.StringProperty("span_kind", "critical_path_service_frame"),
		util.StringProperty("span_id", payload.SpanID),
		util.StringProperty("span_name", serviceName),
		util.StringProperty("service_name", serviceName),
		util.StringProperty("operation_name", payload.OperationName),
		util.StringProperty("critical_path_strategy", view.Request.ResolvedCriticalPathStrategy()),
		util.IntegerProperty("critical_path_sequence", int64(sv.sequence)),
	), nil
}

func (sv criticalPathFrameSpanView) payload() *SpanPayload {
	payload := sv.element.Span().Payload()
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
	return sv.namer.SpanName(sv.element.Span())
}

type criticalPathLeafSpanView criticalPathFrameSpanView

func (sv criticalPathLeafSpanView) ID() rendertrace.SpanID {
	return rendertrace.SpanID(fmt.Sprintf("critical-path:leaf:%d:%s", sv.sequence, sv.spanID()))
}

func (sv criticalPathLeafSpanView) TimeRange() rendertrace.TimeRange {
	return rendertrace.TimeRange{Start: sv.start, End: sv.end}
}

func (sv criticalPathLeafSpanView) ChildSpans(
	ctx context.Context,
	view rendertrace.RenderView,
) ([]rendertrace.SpanView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (sv criticalPathLeafSpanView) RenderTraceVizSpan(
	ctx context.Context,
	view rendertrace.RenderView,
	parent rendertrace.TraceVizSpanParent,
) (*tvtrace.Span[time.Duration], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	payload := sv.payload()
	labelText := payload.OperationName
	if labelText == "" {
		labelText = sv.spanID()
	}
	return parent.Span(
		sv.start,
		sv.end,
		label.Format("$(span_name)"),
		color.Primary(realSpanColor),
		color.Secondary(realSpanColor),
		color.Stroke("#202124"),
		util.StringProperty("span_kind", "critical_path_leaf"),
		util.StringProperty("span_id", payload.SpanID),
		util.StringProperty("span_name", labelText),
		util.StringProperty("service_name", payload.ServiceName),
		util.StringProperty("operation_name", payload.OperationName),
		util.StringProperty("critical_path_strategy", view.Request.ResolvedCriticalPathStrategy()),
		util.IntegerProperty("critical_path_sequence", int64(sv.sequence)),
	), nil
}

func (sv criticalPathLeafSpanView) payload() *SpanPayload {
	return criticalPathFrameSpanView(sv).payload()
}

func (sv criticalPathLeafSpanView) spanID() string {
	return criticalPathFrameSpanView(sv).spanID()
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
	color       string
}

var renderedCausalEventStyles = map[renderedCausalEventKind]renderedCausalEventStyle{
	renderedIncomingDependencyEvent: {
		displayName: "Incoming dependency",
		color:       "#2166ac",
	},
	renderedOutgoingDependencyEvent: {
		displayName: "Outgoing dependency",
		color:       "#b2182b",
	},
	renderedMarkEvent: {
		displayName: "Mark",
		color:       "#4d9221",
	},
}

const realSpanColor = "#9ecae1"
const searchMatchColor = "#f97316"

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
	return rendertrace.SpanID("synthetic-service:" + sv.categoryPayload.ID)
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
	serviceColor := serviceColor(serviceName)
	renderedSpan := parent.Span(
		spanRange.Start,
		spanRange.End,
		label.Format(""),
		color.Primary(serviceColor),
		color.Secondary(serviceColor),
		color.Stroke("#202124"),
		util.StringProperty("span_kind", "synthetic_service"),
		util.StringProperty("span_name", serviceName),
		util.StringProperty("service_name", serviceName),
		util.StringProperty("category_id", sv.categoryPayload.ID),
		util.IntegerProperty("concurrency_peak", int64(sv.concurrencyMap.Peak)),
	)
	addCriticalPathOverlayNodes(renderedSpan, view, sv.ID())
	for _, bucket := range sv.concurrencyMap.Buckets(concurrency.BucketOptions{
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
		if sv.concurrencyMap.Peak > 0 {
			intensity = bucket.Avg / float64(sv.concurrencyMap.Peak)
		}
		bucketColor := serviceHeatmapColor(serviceName, intensity)
		renderedSpan.Subspan(
			time.Duration(bucket.Start),
			time.Duration(bucket.End),
			color.Primary(bucketColor),
			color.Secondary(bucketColor),
			color.Stroke("#202124"),
			util.StringProperty("subspan_kind", "concurrency_heatmap"),
			util.StringProperty("span_kind", "synthetic_service"),
			util.StringProperty("span_name", serviceName),
			util.StringProperty("service_name", serviceName),
			util.StringProperty("category_id", sv.categoryPayload.ID),
			util.DoubleProperty("concurrency_avg", bucket.Avg),
			util.IntegerProperty("concurrency_peak", int64(bucket.Peak)),
		)
	}
	return renderedSpan, nil
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
		if !spanSubtreeContainsFocusedSpan(child, view) {
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

func spanPrimaryColor(spanID rendertrace.SpanID, view rendertrace.RenderView) string {
	if spanMatchesSearch(spanID, view) {
		return searchMatchColor
	}
	return realSpanColor
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
	renderedSpan := parent.Span(
		sv.span.Start(),
		sv.span.End(),
		label.Format("$(span_name)"),
		color.Primary(spanPrimaryColor(sv.ID(), view)),
		color.Secondary(spanPrimaryColor(sv.ID(), view)),
		color.Stroke("#202124"),
		util.StringProperty("span_id", payload.SpanID),
		util.StringProperty("span_name", labelText),
		util.StringProperty("service_name", payload.ServiceName),
		util.StringProperty("process_id", payload.ProcessID),
		util.StringProperty("operation_name", payload.OperationName),
		util.IntegerProperty("suspend_count", int64(spanSuspendCount(sv.span))),
		util.IntegerProperty("causal_event_count", int64(spanCausalEventCount(sv.span))),
	)
	addCriticalPathOverlayNodes(renderedSpan, view, sv.ID())
	for _, suspend := range renderableSuspendIntervals(sv.span, view) {
		renderedSpan.Subspan(
			suspend.Start,
			suspend.End,
			color.Primary("rgba(128, 128, 128, 0.48)"),
			color.Secondary("rgba(128, 128, 128, 0.48)"),
			color.Stroke("#202124"),
			util.StringProperty("subspan_kind", "suspend"),
			util.StringProperty("span_id", payload.SpanID),
			util.StringProperty("span_name", labelText),
			util.StringProperty("service_name", payload.ServiceName),
			util.StringProperty("process_id", payload.ProcessID),
			util.StringProperty("operation_name", payload.OperationName),
		)
	}
	for _, eventChip := range renderableCausalEventChips(sv.span, view, sv.namer) {
		eventStyle := renderedCausalEventStyles[eventChip.Event.kind]
		renderedSpan.Subspan(
			eventChip.Start,
			eventChip.End,
			color.Primary(eventStyle.color),
			color.Secondary(eventStyle.color),
			color.Stroke("#202124"),
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
		)
	}
	return renderedSpan, nil
}

func addCriticalPathOverlayNodes(
	renderedSpan *tvtrace.Span[time.Duration],
	view rendertrace.RenderView,
	spanID rendertrace.SpanID,
) {
	for _, node := range view.CriticalPathOverlay.NodesForSpan(spanID) {
		traceedge.New(
			view.TemporalAxis,
			renderedSpan,
			node.Moment,
			node.ID,
			node.EndpointNodeIDs...,
		).With(
			color.Stroke("#3730a3"),
			color.Secondary("#3730a3"),
			util.StringProperty("trace_edge_kind", "critical_path"),
			util.StringProperty("critical_path_strategy", view.Request.ResolvedCriticalPathStrategy()),
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
	rawSuspends := suspendIntervals(span, view.TemporalDomain)
	if len(rawSuspends) == 0 {
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

func serviceColor(serviceName string) string {
	return fmt.Sprintf("hsl(%d, 32%%, 85%%)", serviceHue(serviceName))
}

func serviceHeatmapColor(serviceName string, intensity float64) string {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}
	saturation := 18 + int(math.Round(22*intensity))
	lightness := 96 - int(math.Round(18*intensity))
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
