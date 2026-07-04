// Package rendertrace defines trace-format-independent interfaces for turning
// Tracey traces into TraceViz trace responses.
//
// The frontend contract remains the existing TraceViz data model. This package
// describes the server-side semantic layer that decides what subset of a rich
// Tracey trace should be rendered, which categories are visible, which
// trace-format-specific view objects should be asked to render themselves, and
// which TraceViz properties should be attached to rendered items.
package rendertrace

import (
	"context"
	"fmt"
	"time"

	"github.com/ilhamster/traceviz/server/go/category"
	categoryaxis "github.com/ilhamster/traceviz/server/go/category_axis"
	continuousaxis "github.com/ilhamster/traceviz/server/go/continuous_axis"
	tvtrace "github.com/ilhamster/traceviz/server/go/trace"
	"github.com/ilhamster/traceviz/server/go/util"
	criticalpath "github.com/ilhamster/tracey/critical_path"
	"github.com/ilhamster/tracey/trace"
)

// TraceID is a stable identifier for a renderable trace within a collection.
type TraceID string

// CategoryID is a stable identifier for a category in a specific hierarchy.
type CategoryID string

// SpanID is a stable identifier for a Tracey span.
type SpanID string

// RenderNamer names and formats non-generic rendered trace elements.
type RenderNamer interface {
	// MomentString returns a human-facing representation of a render-time
	// moment.
	MomentString(moment time.Duration) string
}

// TimeRange is a closed interval in render time.
//
// Render time is currently represented as time.Duration. Typed adapters may
// project richer temporal domains, such as time.Time, into this coordinate
// space while preserving original timestamps in labels and TraceViz
// properties.
type TimeRange struct {
	Start time.Duration
	End   time.Duration
}

// DisplayMode controls coarse filtering of the rendered view.
type DisplayMode string

const (
	// DisplayAll renders the normal hierarchy-filtered view.
	DisplayAll DisplayMode = "all"
	// DisplayOnlyMatches renders search matches plus required ancestry.
	DisplayOnlyMatches DisplayMode = "matches"
)

// Theme selects a broad visual palette for backend-rendered trace data.
type Theme string

const (
	// ThemeLight renders colors for a light frontend shell.
	ThemeLight Theme = "light"
	// ThemeDark renders colors for a dark frontend shell.
	ThemeDark Theme = "dark"
)

// RenderRequest contains all semantic view state needed to render a trace.
//
// Frontends should hold this state in TraceViz Values and send it on normal
// DataSeriesRequests. TemporalDomain and TraceViewRangePx define the
// horizontal domain/range mapping so the backend can avoid emitting more
// visual detail than the current viewport can display.
type RenderRequest struct {
	// TraceID selects the loaded trace to render.
	TraceID TraceID
	// HierarchyType selects the category hierarchy used by the main trace
	// view. Secondary stack views, such as the overtime critical path, use the
	// same hierarchy for their synthetic category frames.
	HierarchyType trace.HierarchyType
	// CriticalPathStart is the user-facing trace position specifier used as the
	// current critical path origin. Empty means DefaultCriticalPathStart.
	CriticalPathStart string
	// CriticalPathEnd is the user-facing trace position specifier used as the
	// current critical path destination. Empty means DefaultCriticalPathEnd.
	CriticalPathEnd string
	// CriticalPathStrategy is the Tracey critical-path strategy name. Empty
	// means DefaultCriticalPathStrategy.
	CriticalPathStrategy string
	// ExplicitExpanded contains category IDs the user has explicitly expanded
	// in the selected hierarchy.
	ExplicitExpanded map[CategoryID]struct{}
	// Search is the raw user search/specifier text. Empty search text disables
	// match-specific rendering.
	Search string
	// ExpandMatches asks the renderer to expand categories needed to reveal
	// matching descendant spans or categories.
	ExpandMatches bool
	// HideEmptyCategories asks the renderer to omit categories with no visible
	// content under the current temporal domain and display mode.
	HideEmptyCategories bool
	// ShowOnlyCriticalPath asks the renderer to omit spans and categories that
	// do not lie on, or contain, the current critical path. Required ancestry is
	// retained so matching critical-path work remains locatable.
	ShowOnlyCriticalPath bool
	// DisplayMode controls coarse filtering such as normal rendering or
	// match-only rendering. Critical-path-only rendering is controlled by
	// ShowOnlyCriticalPath because it also requires critical-path endpoint and
	// strategy state.
	DisplayMode DisplayMode
	// Theme selects the palette used by backend-rendered trace elements.
	// Unknown or empty values should be treated as ThemeLight.
	Theme Theme
	// FocusSpanIDs contains the selected span stack for a focused-span view.
	// The head of the stack is the first element. Implementations should render
	// these spans and whatever category/span ancestry is required to locate
	// them, but should not automatically render descendant spans merely because
	// an ancestor is focused.
	FocusSpanIDs []SpanID
	// TemporalDomain is the visible time interval mapped onto the horizontal
	// trace view. Nil means the trace's full time range.
	TemporalDomain *TimeRange
	// TraceViewRangePx is the horizontal pixel width available for rendering
	// TemporalDomain. Non-positive values mean no pixel budget is known.
	TraceViewRangePx int
	// MinimumFeatureWidthPx is the minimum useful rendered width for temporal
	// features such as event chips, suspend intervals, and heatmap buckets.
	// Non-positive values let the trace adapter choose its default.
	MinimumFeatureWidthPx float64
}

// RenderView contains request-derived state shared by category and span views
// while rendering one TraceViz response.
type RenderView struct {
	// Request is the non-generic render request for the current response.
	Request RenderRequest
	// TemporalDomain is the resolved visible time interval for this render.
	// It equals Request.TemporalDomain when provided, otherwise the trace's
	// full renderable time range.
	TemporalDomain TimeRange
	// TemporalAxis is the TraceViz axis used for this render. Span renderers
	// may use it when emitting time-positioned payloads such as trace-edge
	// nodes.
	TemporalAxis continuousaxis.Axis[time.Duration]
	// SearchResult is the evaluated search result for Request.Search. Nil means
	// no search has been evaluated or no search-specific rendering is needed.
	SearchResult *SearchResult
	// CriticalPathOverlay contains trace-edge nodes for the current critical
	// path overlay, keyed by the rendered span ID to which they should attach.
	// Nil means no critical-path overlay should be rendered.
	CriticalPathOverlay *CriticalPathOverlay
	// FocusDependencyOverlay contains trace-edge nodes for dependencies whose
	// endpoints are both present in a focused-span stack. Nil means no focused
	// dependency overlay should be rendered.
	FocusDependencyOverlay *TraceEdgeOverlay
	// CriticalPathVisibility identifies the spans and categories that should
	// remain visible when Request.ShowOnlyCriticalPath is enabled. Nil means no
	// critical-path visibility filter has been computed.
	CriticalPathVisibility *CriticalPathVisibility
}

const (
	// DefaultCriticalPathStart is the default critical-path origin control
	// value. Empty means the renderer chooses a trace-wide default endpoint.
	DefaultCriticalPathStart = ""
	// DefaultCriticalPathEnd is the default critical-path destination control
	// value. Empty means the renderer chooses a trace-wide default endpoint.
	DefaultCriticalPathEnd = ""
	// DefaultCriticalPathStrategy is the default Tracey critical-path strategy
	// used by the causal tracing tool while causality coverage is incomplete.
	DefaultCriticalPathStrategy = "temporal_most_work"
)

// ResolvedCriticalPathStart returns the request's effective critical-path
// origin specifier.
func (rr RenderRequest) ResolvedCriticalPathStart() string {
	if rr.CriticalPathStart == "" {
		return DefaultCriticalPathStart
	}
	return rr.CriticalPathStart
}

// ResolvedCriticalPathEnd returns the request's effective critical-path
// destination specifier.
func (rr RenderRequest) ResolvedCriticalPathEnd() string {
	if rr.CriticalPathEnd == "" {
		return DefaultCriticalPathEnd
	}
	return rr.CriticalPathEnd
}

// ResolvedCriticalPathStrategy returns the request's effective critical-path
// strategy name.
func (rr RenderRequest) ResolvedCriticalPathStrategy() string {
	if rr.CriticalPathStrategy == "" {
		return DefaultCriticalPathStrategy
	}
	return rr.CriticalPathStrategy
}

// SearchResult identifies spans and categories matching a user search.
type SearchResult struct {
	// Categories are direct category matches.
	Categories map[CategoryID]struct{}
	// Spans are direct span matches.
	Spans map[SpanID]struct{}
	// CategoriesWithDescendantMatch are categories that contain at least one
	// matched category or span below them. Renderers can use this for breadcrumb
	// chips and force-expansion without treating the ancestor as a direct match.
	CategoriesWithDescendantMatch map[CategoryID]struct{}
	// SpansWithDescendantMatch are spans that contain at least one matched
	// descendant span. Renderers can use this to retain required span ancestry
	// without recursively searching the span tree at render time.
	SpansWithDescendantMatch map[SpanID]struct{}
}

// TraceVizCategoryParent is implemented by TraceViz trace and category
// builders that can contain child categories.
type TraceVizCategoryParent interface {
	Category(category *category.Category, properties ...util.PropertyUpdate) *tvtrace.Category[time.Duration]
}

// TraceVizSpanParent is implemented by TraceViz category and span builders
// that can contain child spans.
type TraceVizSpanParent interface {
	Span(start, end time.Duration, properties ...util.PropertyUpdate) *tvtrace.Span[time.Duration]
}

// CategoryView is a non-generic renderable category in the selected view.
//
// A CategoryView may be backed by a Tracey category, a trace-format-specific
// grouping, or another implementation detail. The generic renderer uses it for
// traversal; the view implementation owns its TraceViz rendering details.
type CategoryView interface {
	// ID returns a stable category ID in the selected hierarchy.
	ID() CategoryID
	// ChildCategories returns visible child categories for this render view.
	ChildCategories(ctx context.Context, view RenderView) ([]CategoryView, error)
	// RootSpans returns root spans directly displayed within this category.
	// Returned spans may be real Tracey spans or synthetic spans.
	RootSpans(ctx context.Context, view RenderView) ([]SpanView, error)
	// RenderTraceVizCategory renders this category under the provided TraceViz
	// parent and returns the created TraceViz category.
	RenderTraceVizCategory(
		ctx context.Context,
		view RenderView,
		parent TraceVizCategoryParent,
	) (*tvtrace.Category[time.Duration], error)
}

// SpanView is a non-generic renderable span in the selected view.
//
// A SpanView may be backed by a Tracey span or may be fully synthetic, such as
// a collapsed-category summary span. The generic renderer should not need to
// distinguish those cases.
type SpanView interface {
	// ID returns a stable span ID. Synthetic spans should use IDs stable across
	// equivalent render requests.
	ID() SpanID
	// TimeRange returns the rendered span extent in the canonical duration
	// coordinate.
	TimeRange() TimeRange
	// ChildSpans returns visible child spans for this render view.
	ChildSpans(ctx context.Context, view RenderView) ([]SpanView, error)
	// RenderTraceVizSpan renders this span under the provided TraceViz parent
	// and returns the created TraceViz span. Implementations may add subspans,
	// payloads, and trace-format-specific properties here.
	RenderTraceVizSpan(
		ctx context.Context,
		view RenderView,
		parent TraceVizSpanParent,
	) (*tvtrace.Span[time.Duration], error)
}

// TraceEdgeOverlayNode is one trace-edge endpoint in an overlaid graph,
// expressed in the generic render-time domain.
type TraceEdgeOverlayNode struct {
	// ID is the unique TraceViz trace-edge node ID.
	ID string
	// Moment is the endpoint's time within the rendered trace domain.
	Moment time.Duration
	// EndpointNodeIDs are the trace-edge node IDs connected from this node.
	EndpointNodeIDs []string
}

// TraceEdgeOverlay is a render-time projection of extra trace-edge nodes onto
// the currently visible trace view.
type TraceEdgeOverlay struct {
	NodesBySpanID map[SpanID][]TraceEdgeOverlayNode
}

// NodesForSpan returns overlay nodes that should attach to a rendered span.
func (teo *TraceEdgeOverlay) NodesForSpan(spanID SpanID) []TraceEdgeOverlayNode {
	if teo == nil {
		return nil
	}
	return teo.NodesBySpanID[spanID]
}

// CriticalPathOverlayNode is one trace-edge endpoint in a critical path
// overlay, expressed in the generic render-time domain.
type CriticalPathOverlayNode = TraceEdgeOverlayNode

// CriticalPathOverlay is a render-time projection of a critical path into the
// currently visible trace view.
type CriticalPathOverlay = TraceEdgeOverlay

// CriticalPathVisibility identifies render elements retained by the
// show-only-critical-path display policy. The maps should include required
// ancestry, not only direct critical-path leaves.
type CriticalPathVisibility struct {
	Categories map[CategoryID]struct{}
	Spans      map[SpanID]struct{}
}

// ContainsCategory returns whether the category should remain visible under
// the critical-path visibility filter.
func (cpv *CriticalPathVisibility) ContainsCategory(categoryID CategoryID) bool {
	if cpv == nil {
		return false
	}
	_, ok := cpv.Categories[categoryID]
	return ok
}

// ContainsSpan returns whether the span should remain visible under the
// critical-path visibility filter.
func (cpv *CriticalPathVisibility) ContainsSpan(spanID SpanID) bool {
	if cpv == nil {
		return false
	}
	_, ok := cpv.Spans[spanID]
	return ok
}

// RenderableTrace is the runtime-facing, non-generic interface consumed by the
// causal tracing tool.
//
// Concrete trace formats should usually implement TypedTraceAdapter and then
// expose this interface via AdaptTypedTrace. Keeping this boundary non-generic
// lets one tool binary load and render different Tracey specializations.
type RenderableTrace interface {
	RenderNamer
	// ID returns a stable trace identifier within its collection.
	ID() TraceID
	// DisplayName returns a human-facing trace label.
	DisplayName() string
	// TimeRange returns the full renderable time range.
	TimeRange() TimeRange
	// HierarchyTypes returns the category hierarchies this trace can render.
	HierarchyTypes() *trace.HierarchyTypes
	// Search evaluates a user search string and returns matching category and
	// span IDs. Empty searches should return an empty result, not all items.
	Search(ctx context.Context, hierarchy trace.HierarchyType, query string, temporalDomain TimeRange) (*SearchResult, error)
	// RootCategories returns visible root categories for the render view.
	RootCategories(ctx context.Context, view RenderView) ([]CategoryView, error)
	// RenderTraceViz renders the requested view into the existing TraceViz data
	// model.
	RenderTraceViz(ctx context.Context, req RenderRequest, out util.DataBuilder) error
	// RenderCriticalPathTraceViz renders the current critical path as a
	// standalone TraceViz trace. The critical path element type remains inside
	// the typed adapter boundary.
	RenderCriticalPathTraceViz(ctx context.Context, req RenderRequest, out util.DataBuilder) error
}

// TypedTraceAdapter is the central typed adapter between a concrete
// Tracey-backed trace format and the generic causal tracing renderer.
//
// Tool code should generally store the non-generic RenderableTrace interface;
// renderer internals use this typed interface after AdaptTypedTrace has bound
// the adapter's Tracey specialization.
type TypedTraceAdapter[T any, CP, SP, DP fmt.Stringer] interface {
	RenderNamer
	// Trace returns the underlying Tracey trace.
	Trace() trace.Trace[T, CP, SP, DP]
	// ID returns a stable trace identifier within its collection.
	ID() TraceID
	// DisplayName returns a human-facing trace label.
	DisplayName() string
	// TimeRange returns the full renderable time range in the canonical
	// duration coordinate. Implementations that use another native temporal
	// type should track their own origin/scale and translate internally.
	TimeRange() TimeRange
	// Namer returns the Tracey namer used for paths and search.
	Namer() trace.Namer[T, CP, SP, DP]
	// Search evaluates a user search string and returns matching category and
	// span IDs. Empty searches should return an empty result, not all items.
	Search(ctx context.Context, hierarchy trace.HierarchyType, query string, temporalDomain TimeRange) (*SearchResult, error)
	// RootCategories returns visible root category views for the render view.
	// Category and span views own their TraceViz rendering details, including
	// synthetic spans and subspans.
	RootCategories(ctx context.Context, view RenderView) ([]CategoryView, error)
}

// CriticalPathTraceAdapter is optionally implemented by typed adapters that
// can render a Tracey critical path as a standalone TraceViz trace.
type CriticalPathTraceAdapter[T any, CP, SP, DP fmt.Stringer] interface {
	CriticalPathRootCategories(
		ctx context.Context,
		view RenderView,
		path *criticalpath.Path[T, CP, SP, DP],
	) ([]CategoryView, error)
}

// CriticalPathOverlayAdapter is optionally implemented by typed adapters that
// can project a Tracey critical path into trace-edge nodes on the main trace
// view.
type CriticalPathOverlayAdapter[T any, CP, SP, DP fmt.Stringer] interface {
	CriticalPathOverlay(
		ctx context.Context,
		view RenderView,
		path *criticalpath.Path[T, CP, SP, DP],
	) (*CriticalPathOverlay, error)
}

// FocusDependencyOverlayAdapter is optionally implemented by typed adapters
// that can project dependencies between focused spans into trace-edge nodes on
// the focused trace view.
type FocusDependencyOverlayAdapter[T any, CP, SP, DP fmt.Stringer] interface {
	FocusDependencyOverlay(
		ctx context.Context,
		view RenderView,
	) (*TraceEdgeOverlay, error)
}

// CriticalPathVisibilityAdapter is optionally implemented by typed adapters
// that can identify the spans and categories retained by
// RenderRequest.ShowOnlyCriticalPath.
type CriticalPathVisibilityAdapter[T any, CP, SP, DP fmt.Stringer] interface {
	CriticalPathVisibility(
		ctx context.Context,
		view RenderView,
		path *criticalpath.Path[T, CP, SP, DP],
	) (*CriticalPathVisibility, error)
}

// TraceVizRenderSettings returns the TraceViz trace render settings to attach
// to a rendered trace response.
type TraceVizRenderSettings interface {
	RenderSettings() *tvtrace.RenderSettings
}

type typedRenderableTrace[T any, CP, SP, DP fmt.Stringer] struct {
	adapter TypedTraceAdapter[T, CP, SP, DP]
}

// AdaptTypedTrace exposes a typed trace adapter through the runtime-facing
// RenderableTrace interface.
func AdaptTypedTrace[T any, CP, SP, DP fmt.Stringer](
	adapter TypedTraceAdapter[T, CP, SP, DP],
) RenderableTrace {
	return &typedRenderableTrace[T, CP, SP, DP]{
		adapter: adapter,
	}
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) ID() TraceID {
	return trt.adapter.ID()
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) DisplayName() string {
	return trt.adapter.DisplayName()
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) TimeRange() TimeRange {
	return trt.adapter.TimeRange()
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) HierarchyTypes() *trace.HierarchyTypes {
	return trt.adapter.Namer().HierarchyTypes()
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) MomentString(moment time.Duration) string {
	return trt.adapter.MomentString(moment)
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) Search(
	ctx context.Context,
	hierarchy trace.HierarchyType,
	query string,
	temporalDomain TimeRange,
) (*SearchResult, error) {
	return trt.adapter.Search(ctx, hierarchy, query, temporalDomain)
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) RootCategories(
	ctx context.Context,
	view RenderView,
) ([]CategoryView, error) {
	return trt.adapter.RootCategories(ctx, view)
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) RenderTraceViz(
	ctx context.Context,
	req RenderRequest,
	out util.DataBuilder,
) error {
	return renderTypedTrace(ctx, trt.adapter, req, out)
}

func (trt *typedRenderableTrace[T, CP, SP, DP]) RenderCriticalPathTraceViz(
	ctx context.Context,
	req RenderRequest,
	out util.DataBuilder,
) error {
	return renderTypedCriticalPathTrace(ctx, trt.adapter, req, out)
}

func renderTypedTrace[T any, CP, SP, DP fmt.Stringer](
	ctx context.Context,
	adapter TypedTraceAdapter[T, CP, SP, DP],
	req RenderRequest,
	out util.DataBuilder,
) error {
	fullTimeRange := adapter.TimeRange()
	timeRange := fullTimeRange
	if req.TemporalDomain != nil {
		timeRange = clampTemporalDomain(*req.TemporalDomain, fullTimeRange)
	}
	if timeRange.End < timeRange.Start {
		return fmt.Errorf("trace temporal domain ends before it starts")
	}
	searchResult, err := adapter.Search(ctx, req.HierarchyType, req.Search, timeRange)
	if err != nil {
		return err
	}
	axis := continuousaxis.NewDurationAxis(
		category.New("x_axis", "Time", "Time from trace start"),
		timeRange.Start,
		timeRange.End,
	)
	view := RenderView{
		Request:        req,
		TemporalDomain: timeRange,
		TemporalAxis:   axis,
		SearchResult:   searchResult,
	}
	_, wantsOverlay := any(adapter).(CriticalPathOverlayAdapter[T, CP, SP, DP])
	if len(req.FocusSpanIDs) == 0 && (wantsOverlay || req.ShowOnlyCriticalPath) {
		path, err := findCriticalPath(ctx, adapter.Trace(), req)
		if err != nil {
			if ctx.Err() != nil || req.ShowOnlyCriticalPath {
				return err
			}
		} else {
			if req.ShowOnlyCriticalPath {
				visibilityAdapter, ok := any(adapter).(CriticalPathVisibilityAdapter[T, CP, SP, DP])
				if !ok {
					return fmt.Errorf("trace %q does not support critical path visibility filtering", adapter.DisplayName())
				}
				visibility, err := visibilityAdapter.CriticalPathVisibility(ctx, view, path)
				if err != nil {
					return err
				}
				view.CriticalPathVisibility = visibility
			}
			overlayAdapter, ok := any(adapter).(CriticalPathOverlayAdapter[T, CP, SP, DP])
			if ok {
				overlay, err := overlayAdapter.CriticalPathOverlay(ctx, view, path)
				if err != nil && ctx.Err() != nil {
					return err
				}
				if err == nil {
					view.CriticalPathOverlay = overlay
				}
			}
		}
	}
	if len(req.FocusSpanIDs) > 0 {
		focusOverlayAdapter, ok := any(adapter).(FocusDependencyOverlayAdapter[T, CP, SP, DP])
		if ok {
			overlay, err := focusOverlayAdapter.FocusDependencyOverlay(ctx, view)
			if err != nil {
				return err
			}
			view.FocusDependencyOverlay = overlay
		}
	}
	renderSettings := defaultTraceVizRenderSettings(req.TraceViewRangePx)
	if settingsProvider, ok := any(adapter).(TraceVizRenderSettings); ok {
		renderSettings = settingsProvider.RenderSettings()
	}
	traceVizTrace := tvtrace.New[time.Duration](
		out,
		axis,
		renderSettings,
	).With(
		util.StringProperty("trace_id", string(req.TraceID)),
		util.StringProperty("critical_path_start", req.ResolvedCriticalPathStart()),
		util.StringProperty("critical_path_end", req.ResolvedCriticalPathEnd()),
		util.StringProperty("critical_path_strategy", req.ResolvedCriticalPathStrategy()),
		util.DurationProperty("trace_full_start", fullTimeRange.Start),
		util.DurationProperty("trace_full_end", fullTimeRange.End),
	)
	rootCategories, err := adapter.RootCategories(ctx, view)
	if err != nil {
		return err
	}
	for _, rootCategory := range rootCategories {
		if err := renderCategoryView(ctx, view, traceVizTrace, rootCategory); err != nil {
			return err
		}
	}
	return nil
}

func renderTypedCriticalPathTrace[T any, CP, SP, DP fmt.Stringer](
	ctx context.Context,
	adapter TypedTraceAdapter[T, CP, SP, DP],
	req RenderRequest,
	out util.DataBuilder,
) error {
	criticalPathAdapter, ok := any(adapter).(CriticalPathTraceAdapter[T, CP, SP, DP])
	if !ok {
		return fmt.Errorf("trace %q does not support critical path trace rendering", adapter.DisplayName())
	}
	fullTimeRange := adapter.TimeRange()
	timeRange := fullTimeRange
	if req.TemporalDomain != nil {
		timeRange = clampTemporalDomain(*req.TemporalDomain, fullTimeRange)
	}
	if timeRange.End < timeRange.Start {
		return fmt.Errorf("trace temporal domain ends before it starts")
	}
	path, err := findCriticalPath(ctx, adapter.Trace(), req)
	if err != nil {
		return err
	}
	axis := continuousaxis.NewDurationAxis(
		category.New("x_axis", "Time", "Time from trace start"),
		timeRange.Start,
		timeRange.End,
	)
	view := RenderView{
		Request:        req,
		TemporalDomain: timeRange,
		TemporalAxis:   axis,
	}
	renderSettings := defaultTraceVizRenderSettings(req.TraceViewRangePx)
	if settingsProvider, ok := any(adapter).(TraceVizRenderSettings); ok {
		renderSettings = settingsProvider.RenderSettings()
	}
	traceVizTrace := tvtrace.New[time.Duration](
		out,
		axis,
		renderSettings,
	).With(
		util.StringProperty("trace_id", string(req.TraceID)),
		util.StringProperty("trace_view_kind", "critical_path_overtime"),
		util.StringProperty("critical_path_start", req.ResolvedCriticalPathStart()),
		util.StringProperty("critical_path_end", req.ResolvedCriticalPathEnd()),
		util.StringProperty("critical_path_strategy", req.ResolvedCriticalPathStrategy()),
		util.DurationProperty("trace_full_start", fullTimeRange.Start),
		util.DurationProperty("trace_full_end", fullTimeRange.End),
	)
	rootCategories, err := criticalPathAdapter.CriticalPathRootCategories(ctx, view, path)
	if err != nil {
		return err
	}
	for _, rootCategory := range rootCategories {
		if err := renderCategoryView(ctx, view, traceVizTrace, rootCategory); err != nil {
			return err
		}
	}
	return nil
}

func clampTemporalDomain(requested, full TimeRange) TimeRange {
	if full.End <= full.Start {
		return full
	}
	if requested.End <= requested.Start {
		return full
	}
	requestedWidth := requested.End - requested.Start
	fullWidth := full.End - full.Start
	if requestedWidth >= fullWidth {
		return full
	}
	if requested.Start < full.Start {
		return TimeRange{
			Start: full.Start,
			End:   full.Start + requestedWidth,
		}
	}
	if requested.End > full.End {
		return TimeRange{
			Start: full.End - requestedWidth,
			End:   full.End,
		}
	}
	return requested
}

func defaultTraceVizRenderSettings(traceViewRangePx int) *tvtrace.RenderSettings {
	categoryBaseWidthValPx := 260
	if traceViewRangePx > 0 {
		categoryWidthValPx := clampInt(int(float64(traceViewRangePx)*0.28), 240, 360)
		categoryBaseWidthValPx = categoryWidthValPx - 20
	}
	return &tvtrace.RenderSettings{
		SpanWidthCatPx:   18,
		SpanPaddingCatPx: 0,
		CategoryAxisRenderSettings: &categoryaxis.RenderSettings{
			CategoryHeaderCatPx:    22,
			CategoryHandleValPx:    10,
			CategoryPaddingCatPx:   3,
			CategoryMarginValPx:    10,
			CategoryMinWidthCatPx:  18,
			CategoryBaseWidthValPx: int64(categoryBaseWidthValPx),
		},
		ContinuousAxisRenderSettings: continuousaxis.NewXAxisRenderSettings(
			continuousaxis.RenderSettings{
				LabelHeightPx:   14,
				MarkersHeightPx: 24,
			},
		),
	}
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func renderCategoryView(
	ctx context.Context,
	view RenderView,
	parent TraceVizCategoryParent,
	categoryView CategoryView,
) error {
	renderedCategory, err := categoryView.RenderTraceVizCategory(ctx, view, parent)
	if err != nil {
		return err
	}
	rootSpans, err := categoryView.RootSpans(ctx, view)
	if err != nil {
		return err
	}
	for _, rootSpan := range rootSpans {
		if err := renderSpanView(ctx, view, renderedCategory, rootSpan); err != nil {
			return err
		}
	}
	childCategories, err := categoryView.ChildCategories(ctx, view)
	if err != nil {
		return err
	}
	for _, childCategory := range childCategories {
		if err := renderCategoryView(ctx, view, renderedCategory, childCategory); err != nil {
			return err
		}
	}
	return nil
}

func renderSpanView(
	ctx context.Context,
	view RenderView,
	parent TraceVizSpanParent,
	spanView SpanView,
) error {
	renderedSpan, err := spanView.RenderTraceVizSpan(ctx, view, parent)
	if err != nil {
		return err
	}
	childSpans, err := spanView.ChildSpans(ctx, view)
	if err != nil {
		return err
	}
	for _, childSpan := range childSpans {
		if err := renderSpanView(ctx, view, renderedSpan, childSpan); err != nil {
			return err
		}
	}
	return nil
}

// CategoryPathID returns a stable category ID from a Tracey unique category
// path.
func CategoryPathID(path ...string) CategoryID {
	return CategoryID(trace.EncodePath(path...))
}

// SpanPathID returns a stable span ID from a Tracey unique span path.
func SpanPathID(path ...string) SpanID {
	return SpanID(trace.EncodePath(path...))
}

// DefaultCategoryID returns a category ID derived from Tracey's unique category
// path helper and the supplied namer.
func DefaultCategoryID[T any, CP, SP, DP fmt.Stringer](
	category trace.Category[T, CP, SP, DP],
	namer trace.Namer[T, CP, SP, DP],
) CategoryID {
	return CategoryPathID(trace.GetCategoryUniquePath(category, namer)...)
}

// DefaultSpanID returns a span ID derived from Tracey's unique span path helper
// and the supplied namer.
func DefaultSpanID[T any, CP, SP, DP fmt.Stringer](
	span trace.Span[T, CP, SP, DP],
	namer trace.Namer[T, CP, SP, DP],
) SpanID {
	return SpanPathID(trace.GetSpanUniquePath(span, namer)...)
}
