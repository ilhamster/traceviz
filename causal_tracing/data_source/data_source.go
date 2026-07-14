// Package datasource provides TraceViz data sources for causal trace data.
package datasource

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/ilhamster/traceviz/causal_tracing/extendedotel"
	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	"github.com/ilhamster/traceviz/server/go/category"
	"github.com/ilhamster/traceviz/server/go/color"
	"github.com/ilhamster/traceviz/server/go/table"
	"github.com/ilhamster/traceviz/server/go/util"
	criticalpath "github.com/ilhamster/tracey/critical_path"
	"github.com/ilhamster/tracey/trace"
	traceparser "github.com/ilhamster/tracey/trace/parser"
)

const (
	// loadStatusQuery returns corpus load status as a TraceViz table with one
	// row. It consumes corpus_path, falling back to trace_path for compatibility.
	loadStatusQuery = "causal_tracing.load_status"
	// loadDiagnosticsQuery returns corpus-level nonfatal conversion diagnostics
	// as a TraceViz table with one row per diagnostic. It consumes corpus_path,
	// falling back to trace_path for compatibility.
	loadDiagnosticsQuery = "causal_tracing.load_diagnostics"
	// corpusTracesQuery returns a TraceViz table with one row per trace in the
	// loaded corpus. It consumes corpus_path, falling back to trace_path for
	// compatibility, and emits trace_id as a row property for selection.
	corpusTracesQuery = "causal_tracing.corpus_traces"
	// traceStatusQuery returns selected trace status as a TraceViz table with
	// one row. It consumes corpus_path and trace_id.
	traceStatusQuery = "causal_tracing.trace_status"
	// hierarchyTypesQuery returns the category hierarchies supported by the
	// selected trace. It consumes corpus_path, trace_id, and transform_template.
	hierarchyTypesQuery = "causal_tracing.hierarchy_types"
	// criticalPathStrategiesQuery returns the critical path strategies supported
	// by the selected trace. It consumes corpus_path, trace_id, and
	// transform_template.
	criticalPathStrategiesQuery = "causal_tracing.critical_path_strategies"
	// traceDiagnosticsQuery returns selected-trace nonfatal conversion
	// diagnostics as a TraceViz table with one row per diagnostic. It consumes
	// corpus_path and trace_id.
	traceDiagnosticsQuery = "causal_tracing.trace_diagnostics"
	// traceQuery returns the selected trace rendered into the TraceViz trace
	// data model. It consumes corpus_path and trace_id, and accepts
	// expanded_category_ids, focus_span_ids, temporal_domain_start, and
	// temporal_domain_end, search, and expand_matches as ambient global filters.
	// It also consumes hide_non_matching, hide_empty,
	// show_only_critical_path, critical_path_start, critical_path_end, and
	// critical_path_strategy to render the main-trace critical path overlay and
	// optional critical-path visibility filter. It consumes theme to select
	// backend-rendered colors. It accepts trace_view_width_px as a query
	// parameter. When focus_span_ids is present, it renders the
	// focused span stack and required ancestry.
	traceQuery = "causal_tracing.trace"
	// traceMinimapQuery returns a reduced-detail, full-temporal-domain TraceViz
	// trace for overview rendering. It shares the main trace's hierarchy,
	// expansion, search, and visibility controls, but omits focus state, labels,
	// tooltips, event chips, and trace edges. temporal_domain_start and
	// temporal_domain_end affect it only when hide_empty is enabled.
	traceMinimapQuery = "causal_tracing.trace_minimap"
	// criticalPathTraceQuery returns the selected trace's current critical path
	// rendered as an overtime TraceViz trace. It consumes corpus_path, trace_id,
	// temporal_domain_start, temporal_domain_end, critical_path_start,
	// critical_path_end, critical_path_strategy, and theme as ambient global
	// filters. It accepts trace_view_width_px as a query parameter.
	criticalPathTraceQuery = "causal_tracing.critical_path_trace"
	// spanCausalityQuery returns a table of causality entries for the current
	// focused span stack head. It consumes corpus_path, trace_id, and
	// focus_span_ids and theme.
	spanCausalityQuery = "causal_tracing.span_causality"
	// validateSearchQuery validates the current draft search string as a Tracey
	// span specifier and returns a small status response. Parse failures are
	// ordinary response data, not request errors.
	validateSearchQuery = "causal_tracing.validate_search"
	// validateTransformQuery validates and applies the current draft transform
	// template. Parse and transformation failures are ordinary response data,
	// not request errors; successful responses populate the transformed trace
	// cache for subsequent committed trace requests.
	validateTransformQuery = "causal_tracing.validate_transform"
	// validateCriticalPathQuery validates the current draft critical-path
	// endpoint and strategy controls against the selected trace. Parse,
	// ambiguity, and no-match failures are ordinary response data, not request
	// errors.
	validateCriticalPathQuery = "causal_tracing.validate_critical_path"

	corpusPathKey                = "corpus_path"
	expandedCategoryIDsKey       = "expanded_category_ids"
	focusSpanIDsKey              = "focus_span_ids"
	tracePathKey                 = "trace_path"
	statusKey                    = "status"
	causalityKindKey             = "kind"
	causalityTypeKey             = "type"
	messageKey                   = "message"
	traceIDKey                   = "trace_id"
	spanIDKey                    = "span_id"
	causalityTimeKey             = "time"
	traceCountKey                = "trace_count"
	spanCountKey                 = "span_count"
	diagnosticCountKey           = "diagnostic_count"
	durationKey                  = "duration"
	labelKey                     = "label"
	dependencyTypeKey            = "dependency_type"
	dependencyKeyKey             = "dependency_key"
	otherSpanIDKey               = "other_span_id"
	detailKey                    = "detail"
	serviceCountKey              = "service_count"
	hierarchyTypeKey             = "hierarchy_type"
	hierarchyNameKey             = "hierarchy_name"
	hierarchyDescriptionKey      = "hierarchy_description"
	traceViewWidthPxKey          = "trace_view_width_px"
	temporalDomainStartKey       = "temporal_domain_start"
	temporalDomainEndKey         = "temporal_domain_end"
	criticalPathStartKey         = "critical_path_start"
	criticalPathEndKey           = "critical_path_end"
	criticalPathStrategyKey      = "critical_path_strategy"
	criticalPathStrategyNameKey  = "critical_path_strategy_name"
	criticalPathStrategyDescKey  = "critical_path_strategy_description"
	draftCriticalPathStartKey    = "draft_critical_path_start"
	draftCriticalPathEndKey      = "draft_critical_path_end"
	draftCriticalPathStrategyKey = "draft_critical_path_strategy"
	searchKey                    = "search"
	draftSearchKey               = "draft_search"
	transformTemplateKey         = "transform_template"
	draftTransformKey            = "draft_transform_template"
	expandMatchesKey             = "expand_matches"
	hideNonMatchingKey           = "hide_non_matching"
	hideEmptyKey                 = "hide_empty"
	showOnlyCriticalPathKey      = "show_only_critical_path"
	themeKey                     = "theme"
)

var (
	statusCol               = table.Column(category.New(statusKey, "Status", "Whether the trace file loaded and converted successfully."))
	pathCol                 = table.Column(category.New(corpusPathKey, "Corpus File", "The corpus file requested from the backend."))
	tracesCol               = table.Column(category.New(traceCountKey, "Traces", "The number of traces converted from the response."))
	spansCol                = table.Column(category.New(spanCountKey, "Spans", "The number of OTel spans converted into Tracey root spans."))
	diagsCol                = table.Column(category.New(diagnosticCountKey, "Diagnostics", "The number of non-fatal conversion diagnostics."))
	durationCol             = table.Column(category.New(durationKey, "Duration", "The trace duration from the earliest span start to the latest span end."))
	servicesCol             = table.Column(category.New(serviceCountKey, "Services", "The number of services observed in this trace."))
	msgCol                  = table.Column(category.New(messageKey, "Message", "Additional load or conversion detail."))
	hierarchyNameCol        = table.Column(category.New(hierarchyNameKey, "Name", "The Tracey hierarchy type name."))
	hierarchyDescriptionCol = table.Column(category.New(hierarchyDescriptionKey, "Description", "The hierarchy type description."))
	strategyNameCol         = table.Column(category.New(criticalPathStrategyNameKey, "Name", "The Tracey critical path strategy name."))
	strategyDescriptionCol  = table.Column(category.New(criticalPathStrategyDescKey, "Description", "The critical path strategy description."))

	diagTraceCol = table.Column(category.New(traceIDKey, "Trace ID", "The trace containing the diagnostic."))
	diagSpanCol  = table.Column(category.New(spanIDKey, "Span ID", "The span containing the diagnostic, if known."))
	diagMsgCol   = table.Column(category.New(messageKey, "Issue", "The non-fatal conversion issue."))

	causalityKindCol     = table.Column(category.New(causalityKindKey, "Kind", "Whether this row is an event or suspend interval."))
	causalityTypeCol     = table.Column(category.New(causalityTypeKey, "Type", "The event or interval type."))
	causalityTimeCol     = table.Column(category.New(causalityTimeKey, "Time", "The row's start time in the trace's duration coordinate."))
	causalityDurationCol = table.Column(category.New(durationKey, "Duration", "The duration for interval rows."))
	causalityLabelCol    = table.Column(category.New(labelKey, "Label", "The Tracey mark label, if any."))
	causalityDepTypeCol  = table.Column(category.New(dependencyTypeKey, "Dependency", "The dependency type, if any."))
	causalityDepKeyCol   = table.Column(category.New(dependencyKeyKey, "Dependency Key", "The dependency correlation key, if any."))
	causalityOtherCol    = table.Column(category.New(otherSpanIDKey, "Other Span", "The other dependency endpoint span, if any."))
	causalityDetailCol   = table.Column(category.New(detailKey, "Detail", "Additional event detail."))

	loadStatusRenderSettings = &table.RenderSettings{
		RowHeightPx: 28,
		FontSizePx:  14,
	}
	loadDiagnosticsRenderSettings = &table.RenderSettings{
		RowHeightPx: 24,
		FontSizePx:  13,
	}
)

// Collection is one loaded extended-OTel response and its converted Tracey
// traces.
type Collection struct {
	Path               string
	Converted          []*extendedotel.Trace
	convertedByTraceID map[string]*extendedotel.Trace
	mu                 sync.Mutex
	traceVariants      map[string]*extendedotel.Trace
}

type corpusTraceVariantKey struct {
	corpusPath        string
	traceID           string
	transformTemplate string
}

func (k corpusTraceVariantKey) String() string {
	return "corpus=" + strconv.Quote(k.corpusPath) +
		";trace=" + strconv.Quote(k.traceID) +
		";transform=" + strconv.Quote(k.transformTemplate)
}

// TraceFetcher fetches and converts trace collections by path.
type TraceFetcher interface {
	Fetch(ctx context.Context, tracePath string) (*Collection, error)
}

// FileTraceFetcher loads extended-OTel trace JSON from disk.
type FileTraceFetcher struct {
	root string
}

// NewFileTraceFetcher returns a fetcher rooted at root.
func NewFileTraceFetcher(root string) *FileTraceFetcher {
	return &FileTraceFetcher{root: root}
}

// Fetch loads and converts the requested extended-OTel trace response.
func (f *FileTraceFetcher) Fetch(ctx context.Context, tracePath string) (*Collection, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	resolvedPath, err := f.resolveTracePath(tracePath)
	if err != nil {
		return nil, err
	}
	raw, err := extendedotel.LoadRawResponseFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	converted, err := extendedotel.ConvertExtendedOtelResponse(raw)
	if err != nil {
		return nil, err
	}
	return newCollection(tracePath, converted), nil
}

func (f *FileTraceFetcher) resolveTracePath(tracePath string) (string, error) {
	if f.root == "" {
		return tracePath, nil
	}
	root, err := filepath.Abs(f.root)
	if err != nil {
		return "", err
	}
	cleanTracePath := filepath.Clean(tracePath)
	if !filepath.IsAbs(cleanTracePath) {
		cleanTracePath = filepath.Join(root, cleanTracePath)
	}
	resolvedPath, err := filepath.Abs(cleanTracePath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolvedPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("trace path %q escapes trace root %q", tracePath, f.root)
	}
	return resolvedPath, nil
}

func newCollection(path string, converted []*extendedotel.Trace) *Collection {
	ret := &Collection{
		Path:               path,
		Converted:          converted,
		convertedByTraceID: map[string]*extendedotel.Trace{},
		traceVariants:      map[string]*extendedotel.Trace{},
	}
	for _, convertedTrace := range converted {
		traceID := convertedTrace.TraceID()
		ret.convertedByTraceID[traceID] = convertedTrace
		ret.traceVariants[corpusTraceVariantKey{
			corpusPath: path,
			traceID:    traceID,
		}.String()] = convertedTrace
	}
	return ret
}

func (c *Collection) convertedTrace(traceID string, transformTemplate string) (*extendedotel.Trace, error) {
	if traceID == "" {
		return nil, fmt.Errorf("no trace selected")
	}
	convertedTrace := c.convertedByTraceID[traceID]
	if convertedTrace == nil {
		return nil, fmt.Errorf("unknown trace_id %q", traceID)
	}
	key := corpusTraceVariantKey{
		corpusPath:        c.Path,
		traceID:           traceID,
		transformTemplate: transformTemplate,
	}
	c.mu.Lock()
	if cachedVariant := c.traceVariants[key.String()]; cachedVariant != nil {
		c.mu.Unlock()
		return cachedVariant, nil
	}
	c.mu.Unlock()
	transformedTrace, err := convertedTrace.TransformTemplate(transformTemplate)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.traceVariants[key.String()] = transformedTrace
	c.mu.Unlock()
	return transformedTrace, nil
}

// DataSource loads, caches, and serves causal trace data through TraceViz
// DataSeriesRequests.
type DataSource struct {
	defaultTracePath string
	fetcher          TraceFetcher

	mu  sync.Mutex
	lru *simplelru.LRU
}

// New returns a DataSource using fetcher. defaultTracePath is used when a
// request does not provide a corpus_path global filter.
func New(defaultTracePath string, fetcher TraceFetcher) (*DataSource, error) {
	lru, err := simplelru.NewLRU(10, nil)
	if err != nil {
		return nil, err
	}
	return &DataSource{
		defaultTracePath: defaultTracePath,
		fetcher:          fetcher,
		lru:              lru,
	}, nil
}

// SupportedDataSeriesQueries returns the DataSeriesRequest query names this
// source supports.
func (ds *DataSource) SupportedDataSeriesQueries() []string {
	return []string{
		loadStatusQuery,
		loadDiagnosticsQuery,
		corpusTracesQuery,
		traceStatusQuery,
		hierarchyTypesQuery,
		criticalPathStrategiesQuery,
		traceDiagnosticsQuery,
		traceQuery,
		traceMinimapQuery,
		criticalPathTraceQuery,
		spanCausalityQuery,
		validateSearchQuery,
		validateTransformQuery,
		validateCriticalPathQuery,
	}
}

func (ds *DataSource) corpusPath(globalFilters map[string]*util.V) (string, error) {
	if corpusPathValue, ok := globalFilters[corpusPathKey]; ok {
		corpusPath, err := util.ExpectStringValue(corpusPathValue)
		if err != nil {
			return "", fmt.Errorf("global filter %q must be a string: %w", corpusPathKey, err)
		}
		if corpusPath != "" {
			return corpusPath, nil
		}
	}
	// Compatibility with the first loader UI.
	if tracePathValue, ok := globalFilters[tracePathKey]; ok {
		tracePath, err := util.ExpectStringValue(tracePathValue)
		if err != nil {
			return "", fmt.Errorf("global filter %q must be a string: %w", tracePathKey, err)
		}
		if tracePath != "" {
			return tracePath, nil
		}
	}
	if ds.defaultTracePath == "" {
		return "", fmt.Errorf("no trace path provided")
	}
	return ds.defaultTracePath, nil
}

func traceID(globalFilters map[string]*util.V) (string, error) {
	traceIDValue, ok := globalFilters[traceIDKey]
	if !ok {
		return "", nil
	}
	traceID, err := util.ExpectStringValue(traceIDValue)
	if err != nil {
		return "", fmt.Errorf("global filter %q must be a string: %w", traceIDKey, err)
	}
	return traceID, nil
}

func focusSpanIDs(globalFilters map[string]*util.V) ([]string, error) {
	focusValue, ok := globalFilters[focusSpanIDsKey]
	if !ok {
		return nil, nil
	}
	focusSpanIDs, err := util.ExpectStringsValue(focusValue)
	if err != nil {
		return nil, fmt.Errorf("global filter %q must be a string list: %w", focusSpanIDsKey, err)
	}
	return focusSpanIDs, nil
}

func expandedCategoryIDs(globalFilters map[string]*util.V) ([]string, error) {
	expandedValue, ok := globalFilters[expandedCategoryIDsKey]
	if !ok {
		return nil, nil
	}
	expandedCategoryIDs, err := util.ExpectStringsValue(expandedValue)
	if err != nil {
		return nil, fmt.Errorf("global filter %q must be a string set: %w", expandedCategoryIDsKey, err)
	}
	return expandedCategoryIDs, nil
}

func temporalDomain(globalFilters map[string]*util.V) (*rendertrace.TimeRange, error) {
	startValue, hasStart := globalFilters[temporalDomainStartKey]
	endValue, hasEnd := globalFilters[temporalDomainEndKey]
	if !hasStart && !hasEnd {
		return nil, nil
	}
	if hasStart != hasEnd {
		return nil, fmt.Errorf("global filters %q and %q must be provided together", temporalDomainStartKey, temporalDomainEndKey)
	}
	start, err := util.ExpectDurationValue(startValue)
	if err != nil {
		return nil, fmt.Errorf("global filter %q must be a duration: %w", temporalDomainStartKey, err)
	}
	end, err := util.ExpectDurationValue(endValue)
	if err != nil {
		return nil, fmt.Errorf("global filter %q must be a duration: %w", temporalDomainEndKey, err)
	}
	if start == 0 && end == 0 {
		return nil, nil
	}
	return &rendertrace.TimeRange{Start: start, End: end}, nil
}

func criticalPathControls(globalFilters map[string]*util.V) (start, end, strategy string, err error) {
	if startValue, ok := globalFilters[criticalPathStartKey]; ok {
		start, err = util.ExpectStringValue(startValue)
		if err != nil {
			return "", "", "", fmt.Errorf("global filter %q must be a string: %w", criticalPathStartKey, err)
		}
	}
	if endValue, ok := globalFilters[criticalPathEndKey]; ok {
		end, err = util.ExpectStringValue(endValue)
		if err != nil {
			return "", "", "", fmt.Errorf("global filter %q must be a string: %w", criticalPathEndKey, err)
		}
	}
	if strategyValue, ok := globalFilters[criticalPathStrategyKey]; ok {
		strategy, err = util.ExpectStringValue(strategyValue)
		if err != nil {
			return "", "", "", fmt.Errorf("global filter %q must be a string: %w", criticalPathStrategyKey, err)
		}
	}
	return start, end, strategy, nil
}

func draftCriticalPathControls(globalFilters map[string]*util.V) (start, end, strategy string, err error) {
	if startValue, ok := globalFilters[draftCriticalPathStartKey]; ok {
		start, err = util.ExpectStringValue(startValue)
		if err != nil {
			return "", "", "", fmt.Errorf("global filter %q must be a string: %w", draftCriticalPathStartKey, err)
		}
	}
	if endValue, ok := globalFilters[draftCriticalPathEndKey]; ok {
		end, err = util.ExpectStringValue(endValue)
		if err != nil {
			return "", "", "", fmt.Errorf("global filter %q must be a string: %w", draftCriticalPathEndKey, err)
		}
	}
	if strategyValue, ok := globalFilters[draftCriticalPathStrategyKey]; ok {
		strategy, err = util.ExpectStringValue(strategyValue)
		if err != nil {
			return "", "", "", fmt.Errorf("global filter %q must be a string: %w", draftCriticalPathStrategyKey, err)
		}
	}
	return start, end, strategy, nil
}

func searchControls(globalFilters map[string]*util.V) (search string, expandMatches bool, err error) {
	if searchValue, ok := globalFilters[searchKey]; ok {
		search, err = util.ExpectStringValue(searchValue)
		if err != nil {
			return "", false, fmt.Errorf("global filter %q must be a string: %w", searchKey, err)
		}
	}
	if expandMatchesValue, ok := globalFilters[expandMatchesKey]; ok {
		expandMatches, err = expectStringBoolGlobalFilter(expandMatchesKey, expandMatchesValue)
		if err != nil {
			return "", false, err
		}
	}
	return search, expandMatches, nil
}

func displayControls(globalFilters map[string]*util.V) (hideNonMatching, hideEmpty, showOnlyCriticalPath bool, err error) {
	if hideNonMatchingValue, ok := globalFilters[hideNonMatchingKey]; ok {
		hideNonMatching, err = expectStringBoolGlobalFilter(hideNonMatchingKey, hideNonMatchingValue)
		if err != nil {
			return false, false, false, err
		}
	}
	if hideEmptyValue, ok := globalFilters[hideEmptyKey]; ok {
		hideEmpty, err = expectStringBoolGlobalFilter(hideEmptyKey, hideEmptyValue)
		if err != nil {
			return false, false, false, err
		}
	}
	if showOnlyCriticalPathValue, ok := globalFilters[showOnlyCriticalPathKey]; ok {
		showOnlyCriticalPath, err = expectStringBoolGlobalFilter(showOnlyCriticalPathKey, showOnlyCriticalPathValue)
		if err != nil {
			return false, false, false, err
		}
	}
	return hideNonMatching, hideEmpty, showOnlyCriticalPath, nil
}

func theme(globalFilters map[string]*util.V) (rendertrace.Theme, error) {
	themeValue, ok := globalFilters[themeKey]
	if !ok {
		return rendertrace.ThemeLight, nil
	}
	themeName, err := util.ExpectStringValue(themeValue)
	if err != nil {
		return "", fmt.Errorf("global filter %q must be a string: %w", themeKey, err)
	}
	switch rendertrace.Theme(themeName) {
	case "", rendertrace.ThemeLight:
		return rendertrace.ThemeLight, nil
	case rendertrace.ThemeDark:
		return rendertrace.ThemeDark, nil
	default:
		return rendertrace.ThemeLight, nil
	}
}

func expectStringBoolGlobalFilter(key string, value *util.V) (bool, error) {
	stringValue, err := util.ExpectStringValue(value)
	if err != nil {
		return false, fmt.Errorf("global filter %q must be a string bool: %w", key, err)
	}
	switch stringValue {
	case "true":
		return true, nil
	case "", "false":
		return false, nil
	default:
		return false, fmt.Errorf("global filter %q must be \"true\" or \"false\"", key)
	}
}

func draftSearch(globalFilters map[string]*util.V) (string, error) {
	draftSearchValue, ok := globalFilters[draftSearchKey]
	if !ok {
		return "", nil
	}
	search, err := util.ExpectStringValue(draftSearchValue)
	if err != nil {
		return "", fmt.Errorf("global filter %q must be a string: %w", draftSearchKey, err)
	}
	return search, nil
}

func transformTemplate(globalFilters map[string]*util.V) (string, error) {
	transformTemplateValue, ok := globalFilters[transformTemplateKey]
	if !ok {
		return "", nil
	}
	template, err := util.ExpectStringValue(transformTemplateValue)
	if err != nil {
		return "", fmt.Errorf("global filter %q must be a string: %w", transformTemplateKey, err)
	}
	return template, nil
}

func draftTransformTemplate(globalFilters map[string]*util.V) (string, error) {
	draftTransformValue, ok := globalFilters[draftTransformKey]
	if !ok {
		return "", nil
	}
	template, err := util.ExpectStringValue(draftTransformValue)
	if err != nil {
		return "", fmt.Errorf("global filter %q must be a string: %w", draftTransformKey, err)
	}
	return template, nil
}

func hierarchyName(globalFilters map[string]*util.V) (string, error) {
	hierarchyValue, ok := globalFilters[hierarchyTypeKey]
	if !ok {
		return "", nil
	}
	hierarchyName, err := util.ExpectStringValue(hierarchyValue)
	if err != nil {
		return "", fmt.Errorf("global filter %q must be a string: %w", hierarchyTypeKey, err)
	}
	return hierarchyName, nil
}

func (ds *DataSource) fetchCollection(ctx context.Context, tracePath string) (*Collection, error) {
	ds.mu.Lock()
	if collIf, ok := ds.lru.Get(tracePath); ok {
		ds.mu.Unlock()
		coll, ok := collIf.(*Collection)
		if !ok {
			return nil, fmt.Errorf("cached corpus %q has unexpected type %T", tracePath, collIf)
		}
		return coll, nil
	}
	ds.mu.Unlock()
	coll, err := ds.fetcher.Fetch(ctx, tracePath)
	if err != nil {
		return nil, err
	}
	ds.mu.Lock()
	if collIf, ok := ds.lru.Get(tracePath); ok {
		ds.mu.Unlock()
		cachedColl, ok := collIf.(*Collection)
		if !ok {
			return nil, fmt.Errorf("cached corpus %q has unexpected type %T", tracePath, collIf)
		}
		return cachedColl, nil
	}
	ds.lru.Add(tracePath, coll)
	ds.mu.Unlock()
	return coll, nil
}

// HandleDataSeriesRequests handles TraceViz data requests.
func (ds *DataSource) HandleDataSeriesRequests(
	ctx context.Context,
	globalFilters map[string]*util.V,
	drb *util.DataResponseBuilder,
	reqs []*util.DataSeriesRequest,
) error {
	start := time.Now()
	defer func() {
		fmt.Printf("Handled causal tracing queries in %s\n", time.Since(start))
	}()
	corpusPath, corpusPathErr := ds.corpusPath(globalFilters)
	selectedTraceID, traceIDErr := traceID(globalFilters)
	selectedFocusSpanIDs, focusSpanIDsErr := focusSpanIDs(globalFilters)
	selectedExpandedCategoryIDs, expandedCategoryIDsErr := expandedCategoryIDs(globalFilters)
	selectedTemporalDomain, temporalDomainErr := temporalDomain(globalFilters)
	criticalPathStart, criticalPathEnd, criticalPathStrategy, criticalPathControlsErr := criticalPathControls(globalFilters)
	draftCriticalPathStart, draftCriticalPathEnd, draftCriticalPathStrategy, draftCriticalPathControlsErr := draftCriticalPathControls(globalFilters)
	search, expandMatches, searchControlsErr := searchControls(globalFilters)
	hideNonMatching, hideEmpty, showOnlyCriticalPath, displayControlsErr := displayControls(globalFilters)
	selectedTheme, themeErr := theme(globalFilters)
	searchDraft, draftSearchErr := draftSearch(globalFilters)
	committedTransformTemplate, transformTemplateErr := transformTemplate(globalFilters)
	draftTransformTemplate, draftTransformErr := draftTransformTemplate(globalFilters)
	selectedHierarchyName, hierarchyNameErr := hierarchyName(globalFilters)
	var coll *Collection
	var loadErr error
	if corpusPathErr != nil {
		loadErr = corpusPathErr
	} else if traceIDErr != nil {
		loadErr = traceIDErr
	} else if focusSpanIDsErr != nil {
		loadErr = focusSpanIDsErr
	} else if expandedCategoryIDsErr != nil {
		loadErr = expandedCategoryIDsErr
	} else if temporalDomainErr != nil {
		loadErr = temporalDomainErr
	} else if criticalPathControlsErr != nil {
		loadErr = criticalPathControlsErr
	} else if draftCriticalPathControlsErr != nil {
		loadErr = draftCriticalPathControlsErr
	} else if searchControlsErr != nil {
		loadErr = searchControlsErr
	} else if displayControlsErr != nil {
		loadErr = displayControlsErr
	} else if themeErr != nil {
		loadErr = themeErr
	} else if draftSearchErr != nil {
		loadErr = draftSearchErr
	} else if transformTemplateErr != nil {
		loadErr = transformTemplateErr
	} else if draftTransformErr != nil {
		loadErr = draftTransformErr
	} else if hierarchyNameErr != nil {
		loadErr = hierarchyNameErr
	} else {
		coll, loadErr = ds.fetchCollection(ctx, corpusPath)
	}
	for _, req := range reqs {
		series := drb.DataSeries(req)
		switch req.QueryName {
		case loadStatusQuery:
			handleLoadStatusQuery(series, corpusPath, coll, loadErr)
		case loadDiagnosticsQuery:
			handleLoadDiagnosticsQuery(series, coll, loadErr)
		case corpusTracesQuery:
			handleCorpusTracesQuery(series, coll, loadErr)
		case traceStatusQuery:
			handleTraceStatusQuery(series, coll, selectedTraceID, committedTransformTemplate, loadErr)
		case hierarchyTypesQuery:
			handleHierarchyTypesQuery(series, coll, selectedTraceID, committedTransformTemplate, loadErr)
		case criticalPathStrategiesQuery:
			handleCriticalPathStrategiesQuery(series, coll, selectedTraceID, committedTransformTemplate, loadErr)
		case traceDiagnosticsQuery:
			handleTraceDiagnosticsQuery(series, coll, selectedTraceID, committedTransformTemplate, loadErr)
		case traceQuery:
			if loadErr != nil {
				return loadErr
			}
			if err := handleTraceQuery(ctx, series, req.Options, coll, selectedTraceID, committedTransformTemplate, selectedHierarchyName, selectedExpandedCategoryIDs, criticalPathStart, criticalPathEnd, criticalPathStrategy, search, expandMatches, hideNonMatching, hideEmpty, showOnlyCriticalPath, selectedTheme, traceQueryRenderMode{
				focusSpanIDs:             selectedFocusSpanIDs,
				temporalDomain:           selectedTemporalDomain,
				visibilityTemporalDomain: selectedTemporalDomain,
			}); err != nil {
				return err
			}
		case traceMinimapQuery:
			if loadErr != nil {
				return loadErr
			}
			features := rendertrace.MinimapRenderFeatures()
			var visibilityTemporalDomain *rendertrace.TimeRange
			if hideEmpty {
				visibilityTemporalDomain = selectedTemporalDomain
			}
			if err := handleTraceQuery(ctx, series, req.Options, coll, selectedTraceID, committedTransformTemplate, selectedHierarchyName, selectedExpandedCategoryIDs, criticalPathStart, criticalPathEnd, criticalPathStrategy, search, expandMatches, hideNonMatching, hideEmpty, showOnlyCriticalPath, selectedTheme, traceQueryRenderMode{
				visibilityTemporalDomain: visibilityTemporalDomain,
				features:                 &features,
			}); err != nil {
				return err
			}
		case criticalPathTraceQuery:
			if loadErr != nil {
				return loadErr
			}
			if err := handleCriticalPathTraceQuery(ctx, series, req.Options, coll, selectedTraceID, committedTransformTemplate, selectedHierarchyName, selectedTemporalDomain, criticalPathStart, criticalPathEnd, criticalPathStrategy, selectedTheme); err != nil {
				return err
			}
		case spanCausalityQuery:
			if loadErr != nil {
				return loadErr
			}
			if err := handleSpanCausalityQuery(series, coll, selectedTraceID, committedTransformTemplate, selectedFocusSpanIDs, selectedTheme); err != nil {
				return err
			}
		case validateSearchQuery:
			if loadErr != nil {
				return loadErr
			}
			handleValidateSearchQuery(series, coll, selectedTraceID, committedTransformTemplate, selectedHierarchyName, searchDraft)
		case validateTransformQuery:
			if loadErr != nil {
				return loadErr
			}
			handleValidateTransformQuery(series, coll, selectedTraceID, draftTransformTemplate)
		case validateCriticalPathQuery:
			if loadErr != nil {
				return loadErr
			}
			handleValidateCriticalPathQuery(ctx, series, coll, selectedTraceID, committedTransformTemplate, selectedHierarchyName, draftCriticalPathStart, draftCriticalPathEnd, draftCriticalPathStrategy)
		default:
			return fmt.Errorf("unsupported data query %q", req.QueryName)
		}
	}
	return nil
}

func handleValidateSearchQuery(
	db util.DataBuilder,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	selectedHierarchyName string,
	searchDraft string,
) {
	if searchDraft == "" {
		db.With(
			util.StringProperty(statusKey, "ok"),
			util.StringProperty(draftSearchKey, searchDraft),
			util.StringProperty(messageKey, ""),
		)
		return
	}
	hierarchyType := extendedotel.ServiceHierarchyType
	if coll != nil && selectedTraceID != "" {
		convertedTrace, err := coll.convertedTrace(selectedTraceID, transformTemplate)
		if err != nil {
			db.With(
				util.StringProperty(statusKey, "error"),
				util.StringProperty(draftSearchKey, searchDraft),
				util.StringProperty(messageKey, err.Error()),
			)
			return
		}
		hierarchyType, err = resolveHierarchyType(convertedTrace, selectedHierarchyName)
		if err != nil {
			db.With(
				util.StringProperty(statusKey, "error"),
				util.StringProperty(draftSearchKey, searchDraft),
				util.StringProperty(messageKey, err.Error()),
			)
			return
		}
	}
	if _, err := traceparser.ParseSpanSpecifierPatterns(hierarchyType, searchDraft); err != nil {
		db.With(
			util.StringProperty(statusKey, "error"),
			util.StringProperty(draftSearchKey, searchDraft),
			util.StringProperty(messageKey, err.Error()),
		)
		return
	}
	db.With(
		util.StringProperty(statusKey, "ok"),
		util.StringProperty(draftSearchKey, searchDraft),
		util.StringProperty(messageKey, ""),
	)
}

func handleValidateTransformQuery(
	db util.DataBuilder,
	coll *Collection,
	selectedTraceID string,
	draftTemplate string,
) {
	if coll == nil {
		db.With(
			util.StringProperty(statusKey, "error"),
			util.StringProperty(draftTransformKey, draftTemplate),
			util.StringProperty(messageKey, "corpus is not loaded"),
		)
		return
	}
	if _, err := coll.convertedTrace(selectedTraceID, draftTemplate); err != nil {
		db.With(
			util.StringProperty(statusKey, "error"),
			util.StringProperty(draftTransformKey, draftTemplate),
			util.StringProperty(messageKey, err.Error()),
		)
		return
	}
	db.With(
		util.StringProperty(statusKey, "ok"),
		util.StringProperty(draftTransformKey, draftTemplate),
		util.StringProperty(messageKey, ""),
	)
}

func handleHierarchyTypesQuery(
	db util.DataBuilder,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	loadErr error,
) {
	hierarchyTable := table.New(
		db,
		loadDiagnosticsRenderSettings,
		hierarchyNameCol,
		hierarchyDescriptionCol,
	)
	if loadErr != nil || coll == nil || selectedTraceID == "" {
		return
	}
	convertedTrace, err := coll.convertedTrace(selectedTraceID, transformTemplate)
	if err != nil {
		hierarchyTable.Row(
			table.Cell(hierarchyNameCol, util.String("error")),
			table.Cell(hierarchyDescriptionCol, util.String(err.Error())),
		).With(util.StringProperty(statusKey, "error"))
		return
	}
	for _, hierarchyData := range convertedTrace.RenderableTrace().HierarchyTypes().OrderedTypeData() {
		hierarchyTable.Row(
			table.Cell(hierarchyNameCol, util.String(hierarchyData.Name)),
			table.Cell(hierarchyDescriptionCol, util.String(hierarchyData.Description)),
		).With(
			util.StringProperty(hierarchyTypeKey, hierarchyData.Name),
			util.StringProperty(hierarchyNameKey, hierarchyData.Name),
			util.StringProperty(hierarchyDescriptionKey, hierarchyData.Description),
		)
	}
}

func handleCriticalPathStrategiesQuery(
	db util.DataBuilder,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	loadErr error,
) {
	strategyTable := table.New(
		db,
		loadDiagnosticsRenderSettings,
		strategyNameCol,
		strategyDescriptionCol,
	)
	if loadErr != nil || coll == nil || selectedTraceID == "" {
		return
	}
	if _, err := coll.convertedTrace(selectedTraceID, transformTemplate); err != nil {
		strategyTable.Row(
			table.Cell(strategyNameCol, util.String("error")),
			table.Cell(strategyDescriptionCol, util.String(err.Error())),
		).With(util.StringProperty(statusKey, "error"))
		return
	}
	for _, strategyData := range criticalpath.CommonStrategies.OrderedTypeData() {
		strategyTable.Row(
			table.Cell(strategyNameCol, util.String(strategyData.Name)),
			table.Cell(strategyDescriptionCol, util.String(strategyData.Description)),
		).With(
			util.StringProperty(criticalPathStrategyKey, strategyData.Description),
			util.StringProperty(criticalPathStrategyNameKey, strategyData.Name),
			util.StringProperty(criticalPathStrategyDescKey, strategyData.Description),
		)
	}
}

func resolveHierarchyType(convertedTrace *extendedotel.Trace, hierarchyName string) (trace.HierarchyType, error) {
	if convertedTrace == nil {
		return 0, fmt.Errorf("trace is not loaded")
	}
	if hierarchyName == "" {
		hierarchyName = "service"
	}
	hierarchyData, err := convertedTrace.RenderableTrace().HierarchyTypes().ByName(hierarchyName)
	if err != nil {
		return 0, err
	}
	return hierarchyData.Type, nil
}

func handleValidateCriticalPathQuery(
	ctx context.Context,
	db util.DataBuilder,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	selectedHierarchyName string,
	draftStart string,
	draftEnd string,
	draftStrategy string,
) {
	if coll == nil {
		writeCriticalPathValidationStatus(db, "error", draftStart, draftEnd, draftStrategy, "corpus is not loaded")
		return
	}
	convertedTrace, err := coll.convertedTrace(selectedTraceID, transformTemplate)
	if err != nil {
		writeCriticalPathValidationStatus(db, "error", draftStart, draftEnd, draftStrategy, err.Error())
		return
	}
	hierarchyType, err := resolveHierarchyType(convertedTrace, selectedHierarchyName)
	if err != nil {
		writeCriticalPathValidationStatus(db, "error", draftStart, draftEnd, draftStrategy, err.Error())
		return
	}
	req := rendertrace.RenderRequest{
		TraceID:              rendertrace.TraceID(selectedTraceID),
		HierarchyType:        hierarchyType,
		CriticalPathStart:    draftStart,
		CriticalPathEnd:      draftEnd,
		CriticalPathStrategy: draftStrategy,
	}
	if err := rendertrace.ValidateCriticalPath(ctx, convertedTrace.Trace(), req); err != nil {
		writeCriticalPathValidationStatus(db, "error", draftStart, draftEnd, draftStrategy, err.Error())
		return
	}
	writeCriticalPathValidationStatus(db, "ok", draftStart, draftEnd, draftStrategy, "")
}

func writeCriticalPathValidationStatus(
	db util.DataBuilder,
	status string,
	draftStart string,
	draftEnd string,
	draftStrategy string,
	message string,
) {
	db.With(
		util.StringProperty(statusKey, status),
		util.StringProperty(draftCriticalPathStartKey, draftStart),
		util.StringProperty(draftCriticalPathEndKey, draftEnd),
		util.StringProperty(draftCriticalPathStrategyKey, draftStrategy),
		util.StringProperty(messageKey, message),
	)
}

type traceQueryRenderMode struct {
	focusSpanIDs             []string
	temporalDomain           *rendertrace.TimeRange
	visibilityTemporalDomain *rendertrace.TimeRange
	features                 *rendertrace.RenderFeatures
}

func handleTraceQuery(
	ctx context.Context,
	db util.DataBuilder,
	reqOpts map[string]*util.V,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	selectedHierarchyName string,
	expandedCategoryIDs []string,
	criticalPathStart string,
	criticalPathEnd string,
	criticalPathStrategy string,
	search string,
	expandMatches bool,
	hideNonMatching bool,
	hideEmpty bool,
	showOnlyCriticalPath bool,
	theme rendertrace.Theme,
	mode traceQueryRenderMode,
) error {
	if coll == nil {
		return fmt.Errorf("corpus is not loaded")
	}
	if selectedTraceID == "" {
		return nil
	}
	convertedTrace, err := coll.convertedTrace(selectedTraceID, transformTemplate)
	if err != nil {
		return err
	}
	hierarchyType, err := resolveHierarchyType(convertedTrace, selectedHierarchyName)
	if err != nil {
		return err
	}
	traceViewWidthPx := 0
	if widthValue, ok := reqOpts[traceViewWidthPxKey]; ok {
		width, err := util.ExpectIntegerValue(widthValue)
		if err != nil {
			return fmt.Errorf("query parameter %q must be an integer: %w", traceViewWidthPxKey, err)
		}
		traceViewWidthPx = int(width)
	}
	displayMode := rendertrace.DisplayAll
	if hideNonMatching && search != "" {
		displayMode = rendertrace.DisplayOnlyMatches
	}
	req := rendertrace.RenderRequest{
		TraceID:                  rendertrace.TraceID(selectedTraceID),
		HierarchyType:            hierarchyType,
		DisplayMode:              displayMode,
		Theme:                    theme,
		TraceViewRangePx:         traceViewWidthPx,
		FocusSpanIDs:             renderFocusSpanIDs(mode.focusSpanIDs),
		ExplicitExpanded:         renderExpandedCategoryIDs(expandedCategoryIDs),
		TemporalDomain:           mode.temporalDomain,
		VisibilityTemporalDomain: mode.visibilityTemporalDomain,
		CriticalPathStart:        criticalPathStart,
		CriticalPathEnd:          criticalPathEnd,
		CriticalPathStrategy:     criticalPathStrategy,
		Search:                   search,
		ExpandMatches:            expandMatches,
		HideEmptyCategories:      hideEmpty,
		ShowOnlyCriticalPath:     showOnlyCriticalPath,
		Features:                 mode.features,
	}
	return convertedTrace.RenderableTrace().RenderTraceViz(ctx, req, db)
}

func handleCriticalPathTraceQuery(
	ctx context.Context,
	db util.DataBuilder,
	reqOpts map[string]*util.V,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	selectedHierarchyName string,
	temporalDomain *rendertrace.TimeRange,
	criticalPathStart string,
	criticalPathEnd string,
	criticalPathStrategy string,
	theme rendertrace.Theme,
) error {
	if coll == nil {
		return fmt.Errorf("corpus is not loaded")
	}
	if selectedTraceID == "" {
		return nil
	}
	convertedTrace, err := coll.convertedTrace(selectedTraceID, transformTemplate)
	if err != nil {
		return err
	}
	hierarchyType, err := resolveHierarchyType(convertedTrace, selectedHierarchyName)
	if err != nil {
		return err
	}
	traceViewWidthPx := 0
	if widthValue, ok := reqOpts[traceViewWidthPxKey]; ok {
		width, err := util.ExpectIntegerValue(widthValue)
		if err != nil {
			return fmt.Errorf("query parameter %q must be an integer: %w", traceViewWidthPxKey, err)
		}
		traceViewWidthPx = int(width)
	}
	req := rendertrace.RenderRequest{
		TraceID:              rendertrace.TraceID(selectedTraceID),
		HierarchyType:        hierarchyType,
		DisplayMode:          rendertrace.DisplayAll,
		Theme:                theme,
		TraceViewRangePx:     traceViewWidthPx,
		TemporalDomain:       temporalDomain,
		CriticalPathStart:    criticalPathStart,
		CriticalPathEnd:      criticalPathEnd,
		CriticalPathStrategy: criticalPathStrategy,
	}
	return convertedTrace.RenderableTrace().RenderCriticalPathTraceViz(ctx, req, db)
}

func handleSpanCausalityQuery(
	db util.DataBuilder,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	focusSpanIDs []string,
	theme rendertrace.Theme,
) error {
	causalityTable := table.New(
		db,
		loadDiagnosticsRenderSettings,
		causalityKindCol,
		causalityTypeCol,
		causalityTimeCol,
		causalityDurationCol,
		causalityLabelCol,
		causalityDepTypeCol,
		causalityDepKeyCol,
		causalityOtherCol,
		causalityDetailCol,
	)
	if coll == nil || selectedTraceID == "" || len(focusSpanIDs) == 0 {
		return nil
	}
	convertedTrace, err := coll.convertedTrace(selectedTraceID, transformTemplate)
	if err != nil {
		return err
	}
	focusSpanID := focusSpanIDs[0]
	entries, err := convertedTrace.SpanCausalityEntries(focusSpanID)
	if err != nil {
		causalityTable.Row(
			table.Cell(causalityKindCol, util.String("error")),
			table.Cell(causalityTypeCol, util.String("")),
			table.Cell(causalityTimeCol, util.String("")),
			table.Cell(causalityDurationCol, util.String("")),
			table.Cell(causalityLabelCol, util.String("")),
			table.Cell(causalityDepTypeCol, util.String("")),
			table.Cell(causalityDepKeyCol, util.String("")),
			table.Cell(causalityOtherCol, util.String("")),
			table.Cell(causalityDetailCol, util.String(err.Error())),
		).With(
			util.StringProperty(spanIDKey, focusSpanID),
			util.StringProperty(extendedotel.CausalityEntryIDProperty, ""),
		)
		return nil
	}
	for _, entry := range entries {
		durationValue := util.String("")
		if entry.Duration > 0 {
			durationValue = util.Duration(entry.Duration)
		}
		causalityTable.Row(
			table.Cell(causalityKindCol, util.String(string(entry.Kind))),
			table.Cell(causalityTypeCol, util.String(entry.Type)),
			table.Cell(causalityTimeCol, util.Duration(entry.Time)),
			table.Cell(causalityDurationCol, durationValue),
			table.Cell(causalityLabelCol, util.String(entry.Label)),
			table.Cell(causalityDepTypeCol, util.String(entry.DependencyType)),
			table.Cell(causalityDepKeyCol, util.String(entry.DependencyKey)),
			table.Cell(causalityOtherCol, util.String(entry.OtherSpanID)),
			table.Cell(causalityDetailCol, util.String(entry.Detail)),
		).With(
			util.StringProperty(spanIDKey, focusSpanID),
			util.StringProperty(otherSpanIDKey, entry.OtherSpanID),
			util.StringProperty(extendedotel.CausalityEntryIDProperty, entry.ID),
			color.Secondary(spanCausalityRowHighlightColor(theme)),
		)
	}
	return nil
}

func spanCausalityRowHighlightColor(theme rendertrace.Theme) string {
	if theme == rendertrace.ThemeDark {
		return "rgba(100, 116, 139, 0.48)"
	}
	return "rgba(148, 163, 184, 0.30)"
}

func renderFocusSpanIDs(focusSpanIDs []string) []rendertrace.SpanID {
	if len(focusSpanIDs) == 0 {
		return nil
	}
	ret := make([]rendertrace.SpanID, 0, len(focusSpanIDs))
	for _, focusSpanID := range focusSpanIDs {
		if focusSpanID == "" {
			continue
		}
		ret = append(ret, rendertrace.SpanID(focusSpanID))
	}
	return ret
}

func renderExpandedCategoryIDs(expandedCategoryIDs []string) map[rendertrace.CategoryID]struct{} {
	if len(expandedCategoryIDs) == 0 {
		return nil
	}
	ret := make(map[rendertrace.CategoryID]struct{}, len(expandedCategoryIDs))
	for _, categoryID := range expandedCategoryIDs {
		if categoryID == "" {
			continue
		}
		ret[rendertrace.CategoryID(categoryID)] = struct{}{}
	}
	return ret
}

func handleCorpusTracesQuery(db util.DataBuilder, coll *Collection, loadErr error) {
	traceTable := table.New(
		db,
		loadDiagnosticsRenderSettings,
		diagTraceCol,
		spansCol,
		diagsCol,
		durationCol,
		servicesCol,
	)
	if loadErr != nil || coll == nil {
		return
	}
	for _, convertedTrace := range coll.Converted {
		traceID := convertedTrace.TraceID()
		traceTable.Row(
			table.Cell(diagTraceCol, util.String(traceID)),
			table.Cell(spansCol, util.Integer(int64(len(convertedTrace.Trace().RootSpans())))),
			table.Cell(diagsCol, util.Integer(int64(len(convertedTrace.Diagnostics())))),
			table.Cell(durationCol, util.Duration(convertedTraceDuration(convertedTrace))),
			table.Cell(servicesCol, util.Integer(int64(traceServiceCount(convertedTrace)))),
		).With(util.StringProperty(traceIDKey, traceID))
	}
}

func handleTraceStatusQuery(
	db util.DataBuilder,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	loadErr error,
) {
	status := "ok"
	message := "loaded and selected"
	traceCount := int64(0)
	spanCount := int64(0)
	diagnosticCount := int64(0)
	duration := time.Duration(0)
	services := int64(0)
	if loadErr != nil {
		status = "error"
		message = loadErr.Error()
	} else if coll == nil {
		status = "error"
		message = "corpus is not loaded"
	} else {
		convertedTrace, err := coll.convertedTrace(selectedTraceID, transformTemplate)
		if err != nil {
			status = "error"
			message = err.Error()
		} else {
			traceCount = 1
			spanCount = int64(len(convertedTrace.Trace().RootSpans()))
			diagnosticCount = int64(len(convertedTrace.Diagnostics()))
			duration = convertedTraceDuration(convertedTrace)
			services = int64(traceServiceCount(convertedTrace))
			if transformTemplate != "" {
				message = "transformed and selected"
			}
		}
	}
	statusTable := table.New(
		db,
		loadStatusRenderSettings,
		statusCol,
		diagTraceCol,
		tracesCol,
		spansCol,
		diagsCol,
		durationCol,
		servicesCol,
		msgCol,
	)
	statusTable.Row(
		table.Cell(statusCol, util.String(status)),
		table.Cell(diagTraceCol, util.String(selectedTraceID)),
		table.Cell(tracesCol, util.Integer(traceCount)),
		table.Cell(spansCol, util.Integer(spanCount)),
		table.Cell(diagsCol, util.Integer(diagnosticCount)),
		table.Cell(durationCol, util.Duration(duration)),
		table.Cell(servicesCol, util.Integer(services)),
		table.Cell(msgCol, util.String(message)),
	)
}

func handleTraceDiagnosticsQuery(
	db util.DataBuilder,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	loadErr error,
) {
	diagnosticsTable := table.New(
		db,
		loadDiagnosticsRenderSettings,
		diagTraceCol,
		diagSpanCol,
		diagMsgCol,
	)
	if loadErr != nil {
		diagnosticsTable.Row(
			table.Cell(diagTraceCol, util.String(selectedTraceID)),
			table.Cell(diagSpanCol, util.String("")),
			table.Cell(diagMsgCol, util.String(loadErr.Error())),
		)
		return
	}
	if coll == nil {
		return
	}
	convertedTrace, err := coll.convertedTrace(selectedTraceID, transformTemplate)
	if err != nil {
		diagnosticsTable.Row(
			table.Cell(diagTraceCol, util.String(selectedTraceID)),
			table.Cell(diagSpanCol, util.String("")),
			table.Cell(diagMsgCol, util.String(err.Error())),
		)
		return
	}
	for _, diagnostic := range convertedTrace.Diagnostics() {
		diagnosticsTable.Row(
			table.Cell(diagTraceCol, util.String(diagnostic.TraceID)),
			table.Cell(diagSpanCol, util.String(diagnostic.SpanID)),
			table.Cell(diagMsgCol, util.String(diagnostic.Message)),
		)
	}
}

func convertedTraceDuration(convertedTrace *extendedotel.Trace) time.Duration {
	timeRange := convertedTrace.TimeRange()
	return timeRange.End - timeRange.Start
}

func traceServiceCount(convertedTrace *extendedotel.Trace) int {
	services := map[string]struct{}{}
	var visitSpan func(trace.Span[time.Duration, *extendedotel.CategoryPayload, *extendedotel.SpanPayload, *extendedotel.DependencyPayload])
	visitSpan = func(span trace.Span[time.Duration, *extendedotel.CategoryPayload, *extendedotel.SpanPayload, *extendedotel.DependencyPayload]) {
		payload := span.Payload()
		if payload == nil || payload.ServiceName == "" {
			services["unknown-service"] = struct{}{}
		} else {
			services[payload.ServiceName] = struct{}{}
		}
		for _, childSpan := range span.ChildSpans() {
			visitSpan(childSpan)
		}
	}
	for _, rootSpan := range convertedTrace.Trace().RootSpans() {
		visitSpan(rootSpan)
	}
	return len(services)
}

func handleLoadStatusQuery(db util.DataBuilder, tracePath string, coll *Collection, loadErr error) {
	status := "ok"
	message := "loaded and converted"
	traceCount := int64(0)
	spanCount := int64(0)
	diagnosticCount := int64(0)
	if loadErr != nil {
		status = "error"
		message = loadErr.Error()
	} else if coll != nil {
		traceCount = int64(len(coll.Converted))
		for _, convertedTrace := range coll.Converted {
			spanCount += int64(len(convertedTrace.Trace().RootSpans()))
			diagnosticCount += int64(len(convertedTrace.Diagnostics()))
		}
	}
	statusTable := table.New(
		db,
		loadStatusRenderSettings,
		statusCol,
		pathCol,
		tracesCol,
		spansCol,
		diagsCol,
		msgCol,
	)
	statusTable.Row(
		table.Cell(statusCol, util.String(status)),
		table.Cell(pathCol, util.String(tracePath)),
		table.Cell(tracesCol, util.Integer(traceCount)),
		table.Cell(spansCol, util.Integer(spanCount)),
		table.Cell(diagsCol, util.Integer(diagnosticCount)),
		table.Cell(msgCol, util.String(message)),
	)
}

func handleLoadDiagnosticsQuery(db util.DataBuilder, coll *Collection, loadErr error) {
	diagnosticsTable := table.New(
		db,
		loadDiagnosticsRenderSettings,
		diagTraceCol,
		diagSpanCol,
		diagMsgCol,
	)
	if loadErr != nil {
		diagnosticsTable.Row(
			table.Cell(diagTraceCol, util.String("")),
			table.Cell(diagSpanCol, util.String("")),
			table.Cell(diagMsgCol, util.String(loadErr.Error())),
		)
		return
	}
	if coll == nil {
		return
	}
	for _, convertedTrace := range coll.Converted {
		for _, diagnostic := range convertedTrace.Diagnostics() {
			diagnosticsTable.Row(
				table.Cell(diagTraceCol, util.String(diagnostic.TraceID)),
				table.Cell(diagSpanCol, util.String(diagnostic.SpanID)),
				table.Cell(diagMsgCol, util.String(diagnostic.Message)),
			)
		}
	}
}
