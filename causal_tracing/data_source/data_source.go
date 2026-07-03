// Package datasource provides TraceViz data sources for causal trace data.
package datasource

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/ilhamster/traceviz/causal_tracing/extendedotel"
	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	"github.com/ilhamster/traceviz/server/go/category"
	"github.com/ilhamster/traceviz/server/go/table"
	"github.com/ilhamster/traceviz/server/go/util"
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
	// traceDiagnosticsQuery returns selected-trace nonfatal conversion
	// diagnostics as a TraceViz table with one row per diagnostic. It consumes
	// corpus_path and trace_id.
	traceDiagnosticsQuery = "causal_tracing.trace_diagnostics"
	// traceQuery returns the selected trace rendered into the TraceViz trace
	// data model. It consumes corpus_path and trace_id, and accepts
	// expanded_category_ids, focus_span_ids, temporal_domain_start, and
	// temporal_domain_end, search, and expand_matches as ambient global filters.
	// It also consumes
	// critical_path_start, critical_path_end, and critical_path_strategy to
	// render the main-trace critical path overlay. It accepts trace_view_width_px
	// as a query parameter. When focus_span_ids is present, it renders the
	// focused span stack and required ancestry.
	traceQuery = "causal_tracing.trace"
	// criticalPathTraceQuery returns the selected trace's current critical path
	// rendered as an overtime TraceViz trace. It consumes corpus_path, trace_id,
	// temporal_domain_start, temporal_domain_end, critical_path_start,
	// critical_path_end, and critical_path_strategy as ambient global filters. It
	// accepts trace_view_width_px as a query parameter.
	criticalPathTraceQuery = "causal_tracing.critical_path_trace"
	// spanCausalityQuery returns a table of causality entries for the current
	// focused span stack head. It consumes corpus_path, trace_id, and
	// focus_span_ids.
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

	corpusPathKey           = "corpus_path"
	expandedCategoryIDsKey  = "expanded_category_ids"
	focusSpanIDsKey         = "focus_span_ids"
	tracePathKey            = "trace_path"
	statusKey               = "status"
	causalityKindKey        = "kind"
	causalityTypeKey        = "type"
	messageKey              = "message"
	traceIDKey              = "trace_id"
	spanIDKey               = "span_id"
	causalityTimeKey        = "time"
	traceCountKey           = "trace_count"
	spanCountKey            = "span_count"
	diagnosticCountKey      = "diagnostic_count"
	durationKey             = "duration"
	labelKey                = "label"
	dependencyTypeKey       = "dependency_type"
	dependencyKeyKey        = "dependency_key"
	otherSpanIDKey          = "other_span_id"
	detailKey               = "detail"
	serviceCountKey         = "service_count"
	traceViewWidthPxKey     = "trace_view_width_px"
	temporalDomainStartKey  = "temporal_domain_start"
	temporalDomainEndKey    = "temporal_domain_end"
	criticalPathStartKey    = "critical_path_start"
	criticalPathEndKey      = "critical_path_end"
	criticalPathStrategyKey = "critical_path_strategy"
	searchKey               = "search"
	draftSearchKey          = "draft_search"
	transformTemplateKey    = "transform_template"
	draftTransformKey       = "draft_transform_template"
	expandMatchesKey        = "expand_matches"
)

var (
	statusCol   = table.Column(category.New(statusKey, "Status", "Whether the trace file loaded and converted successfully."))
	pathCol     = table.Column(category.New(corpusPathKey, "Corpus File", "The corpus file requested from the backend."))
	tracesCol   = table.Column(category.New(traceCountKey, "Traces", "The number of traces converted from the response."))
	spansCol    = table.Column(category.New(spanCountKey, "Spans", "The number of OTel spans converted into Tracey root spans."))
	diagsCol    = table.Column(category.New(diagnosticCountKey, "Diagnostics", "The number of non-fatal conversion diagnostics."))
	durationCol = table.Column(category.New(durationKey, "Duration", "The trace duration from the earliest span start to the latest span end."))
	servicesCol = table.Column(category.New(serviceCountKey, "Services", "The number of services observed in this trace."))
	msgCol      = table.Column(category.New(messageKey, "Message", "Additional load or conversion detail."))

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
	Raw                *extendedotel.RawResponse
	Converted          []*extendedotel.Trace
	convertedByTraceID map[string]*extendedotel.Trace
	traceVariants      map[string]*extendedotel.Trace
}

// TraceVariantKey identifies a loaded trace variant. Empty transform templates
// identify the base trace.
type TraceVariantKey interface {
	fmt.Stringer
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
	resolvedPath := tracePath
	if f.root != "" && !filepath.IsAbs(tracePath) {
		resolvedPath = filepath.Join(f.root, tracePath)
	}
	raw, err := extendedotel.LoadRawResponseFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	converted, err := extendedotel.ConvertExtendedOtelResponse(raw)
	if err != nil {
		return nil, err
	}
	return newCollection(tracePath, raw, converted), nil
}

func newCollection(path string, raw *extendedotel.RawResponse, converted []*extendedotel.Trace) *Collection {
	ret := &Collection{
		Path:               path,
		Raw:                raw,
		Converted:          converted,
		convertedByTraceID: map[string]*extendedotel.Trace{},
		traceVariants:      map[string]*extendedotel.Trace{},
	}
	for _, convertedTrace := range converted {
		traceID := convertedTrace.RawTrace().TraceID
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
	if cachedVariant := c.traceVariants[key.String()]; cachedVariant != nil {
		return cachedVariant, nil
	}
	transformedTrace, err := convertedTrace.TransformTemplate(transformTemplate)
	if err != nil {
		return nil, err
	}
	c.traceVariants[key.String()] = transformedTrace
	return transformedTrace, nil
}

// DataSource loads, caches, and serves causal trace data through TraceViz
// DataSeriesRequests.
type DataSource struct {
	defaultTracePath string
	fetcher          TraceFetcher

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
		traceDiagnosticsQuery,
		traceQuery,
		criticalPathTraceQuery,
		spanCausalityQuery,
		validateSearchQuery,
		validateTransformQuery,
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

func searchControls(globalFilters map[string]*util.V) (search string, expandMatches bool, err error) {
	if searchValue, ok := globalFilters[searchKey]; ok {
		search, err = util.ExpectStringValue(searchValue)
		if err != nil {
			return "", false, fmt.Errorf("global filter %q must be a string: %w", searchKey, err)
		}
	}
	if expandMatchesValue, ok := globalFilters[expandMatchesKey]; ok {
		expandMatchesString, err := util.ExpectStringValue(expandMatchesValue)
		if err != nil {
			return "", false, fmt.Errorf("global filter %q must be a string bool: %w", expandMatchesKey, err)
		}
		switch expandMatchesString {
		case "true":
			expandMatches = true
		case "", "false":
			expandMatches = false
		default:
			return "", false, fmt.Errorf("global filter %q must be \"true\" or \"false\"", expandMatchesKey)
		}
	}
	return search, expandMatches, nil
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

func (ds *DataSource) fetchCollection(ctx context.Context, tracePath string) (*Collection, error) {
	if collIf, ok := ds.lru.Get(tracePath); ok {
		coll, ok := collIf.(*Collection)
		if !ok {
			return nil, fmt.Errorf("cached corpus %q has unexpected type %T", tracePath, collIf)
		}
		return coll, nil
	}
	coll, err := ds.fetcher.Fetch(ctx, tracePath)
	if err != nil {
		return nil, err
	}
	ds.lru.Add(tracePath, coll)
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
	search, expandMatches, searchControlsErr := searchControls(globalFilters)
	searchDraft, draftSearchErr := draftSearch(globalFilters)
	committedTransformTemplate, transformTemplateErr := transformTemplate(globalFilters)
	draftTransformTemplate, draftTransformErr := draftTransformTemplate(globalFilters)
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
	} else if searchControlsErr != nil {
		loadErr = searchControlsErr
	} else if draftSearchErr != nil {
		loadErr = draftSearchErr
	} else if transformTemplateErr != nil {
		loadErr = transformTemplateErr
	} else if draftTransformErr != nil {
		loadErr = draftTransformErr
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
		case traceDiagnosticsQuery:
			handleTraceDiagnosticsQuery(series, coll, selectedTraceID, committedTransformTemplate, loadErr)
		case traceQuery:
			if loadErr != nil {
				return loadErr
			}
			if err := handleTraceQuery(ctx, series, req.Options, coll, selectedTraceID, committedTransformTemplate, selectedFocusSpanIDs, selectedExpandedCategoryIDs, selectedTemporalDomain, criticalPathStart, criticalPathEnd, criticalPathStrategy, search, expandMatches); err != nil {
				return err
			}
		case criticalPathTraceQuery:
			if loadErr != nil {
				return loadErr
			}
			if err := handleCriticalPathTraceQuery(ctx, series, req.Options, coll, selectedTraceID, committedTransformTemplate, selectedTemporalDomain, criticalPathStart, criticalPathEnd, criticalPathStrategy); err != nil {
				return err
			}
		case spanCausalityQuery:
			if loadErr != nil {
				return loadErr
			}
			if err := handleSpanCausalityQuery(series, coll, selectedTraceID, committedTransformTemplate, selectedFocusSpanIDs); err != nil {
				return err
			}
		case validateSearchQuery:
			if loadErr != nil {
				return loadErr
			}
			handleValidateSearchQuery(series, searchDraft)
		case validateTransformQuery:
			if loadErr != nil {
				return loadErr
			}
			handleValidateTransformQuery(series, coll, selectedTraceID, draftTransformTemplate)
		default:
			return fmt.Errorf("unsupported data query %q", req.QueryName)
		}
	}
	return nil
}

func handleValidateSearchQuery(db util.DataBuilder, searchDraft string) {
	if searchDraft == "" {
		db.With(
			util.StringProperty(statusKey, "ok"),
			util.StringProperty(draftSearchKey, searchDraft),
			util.StringProperty(messageKey, ""),
		)
		return
	}
	if _, err := traceparser.ParseSpanSpecifierPatterns(extendedotel.ServiceHierarchyType, searchDraft); err != nil {
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

func handleTraceQuery(
	ctx context.Context,
	db util.DataBuilder,
	reqOpts map[string]*util.V,
	coll *Collection,
	selectedTraceID string,
	transformTemplate string,
	focusSpanIDs []string,
	expandedCategoryIDs []string,
	temporalDomain *rendertrace.TimeRange,
	criticalPathStart string,
	criticalPathEnd string,
	criticalPathStrategy string,
	search string,
	expandMatches bool,
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
		HierarchyType:        extendedotel.ServiceHierarchyType,
		DisplayMode:          rendertrace.DisplayAll,
		TraceViewRangePx:     traceViewWidthPx,
		FocusSpanIDs:         renderFocusSpanIDs(focusSpanIDs),
		ExplicitExpanded:     renderExpandedCategoryIDs(expandedCategoryIDs),
		TemporalDomain:       temporalDomain,
		CriticalPathStart:    criticalPathStart,
		CriticalPathEnd:      criticalPathEnd,
		CriticalPathStrategy: criticalPathStrategy,
		Search:               search,
		ExpandMatches:        expandMatches,
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
	temporalDomain *rendertrace.TimeRange,
	criticalPathStart string,
	criticalPathEnd string,
	criticalPathStrategy string,
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
		HierarchyType:        extendedotel.ServiceHierarchyType,
		DisplayMode:          rendertrace.DisplayAll,
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
		).With(util.StringProperty(spanIDKey, focusSpanID))
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
		)
	}
	return nil
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
		rawTrace := convertedTrace.RawTrace()
		traceTable.Row(
			table.Cell(diagTraceCol, util.String(rawTrace.TraceID)),
			table.Cell(spansCol, util.Integer(int64(len(convertedTrace.Trace().RootSpans())))),
			table.Cell(diagsCol, util.Integer(int64(len(convertedTrace.Diagnostics())))),
			table.Cell(durationCol, util.Duration(rawTraceDuration(rawTrace))),
			table.Cell(servicesCol, util.Integer(int64(rawTraceServiceCount(rawTrace)))),
		).With(util.StringProperty(traceIDKey, rawTrace.TraceID))
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

func rawTraceDuration(rawTrace extendedotel.RawTrace) time.Duration {
	if len(rawTrace.Spans) == 0 {
		return 0
	}
	start := rawTrace.Spans[0].StartTime
	end := rawTrace.Spans[0].StartTime + rawTrace.Spans[0].Duration
	for _, span := range rawTrace.Spans[1:] {
		if span.StartTime < start {
			start = span.StartTime
		}
		spanEnd := span.StartTime + span.Duration
		if spanEnd > end {
			end = spanEnd
		}
	}
	return time.Duration(end-start) * time.Microsecond
}

func rawTraceServiceCount(rawTrace extendedotel.RawTrace) int {
	services := map[string]struct{}{}
	for _, span := range rawTrace.Spans {
		process := rawTrace.Processes[span.ProcessID]
		serviceName := process.ServiceName
		if serviceName == "" {
			serviceName = "unknown-service"
		}
		services[serviceName] = struct{}{}
	}
	return len(services)
}

func convertedTraceDuration(convertedTrace *extendedotel.Trace) time.Duration {
	timeRange := convertedTrace.TimeRange()
	return timeRange.End - timeRange.Start
}

func traceServiceCount(convertedTrace *extendedotel.Trace) int {
	services := map[string]struct{}{}
	for _, rootSpan := range convertedTrace.Trace().RootSpans() {
		payload := rootSpan.Payload()
		if payload == nil || payload.ServiceName == "" {
			services["unknown-service"] = struct{}{}
			continue
		}
		services[payload.ServiceName] = struct{}{}
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
