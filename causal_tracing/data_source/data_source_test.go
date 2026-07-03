package datasource

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	"github.com/ilhamster/traceviz/server/go/util"
)

func newTestDataSource(t *testing.T, defaultTracePath string) *DataSource {
	t.Helper()
	dataSource, err := New(defaultTracePath, NewFileTraceFetcher(""))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return dataSource
}

func TestLoadStatusQueryLoadsSample(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	builder := util.NewDataResponseBuilder()
	err := dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{tracePathKey: util.StringValue(tracePath)},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  loadStatusQuery,
			SeriesName: "status",
			Options:    map[string]*util.V{},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'table_cell': 'ok'",
		"Prop 'table_cell': 346",
		"Prop 'table_cell': 10851",
		"Prop 'table_cell': 1041",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestLoadStatusQueryReportsLoadError(t *testing.T) {
	dataSource := newTestDataSource(t, "missing.json")
	builder := util.NewDataResponseBuilder()
	err := dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  loadStatusQuery,
			SeriesName: "status",
			Options:    map[string]*util.V{},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'table_cell': 'error'",
		"Prop 'table_cell': 'missing.json'",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestLoadDiagnosticsQueryListsSampleDiagnostics(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	builder := util.NewDataResponseBuilder()
	err := dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{tracePathKey: util.StringValue(tracePath)},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  loadDiagnosticsQuery,
			SeriesName: "diagnostics",
			Options:    map[string]*util.V{},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'category_display_name': 'Trace ID'",
		"Prop 'category_display_name': 'Span ID'",
		"Prop 'category_display_name': 'Issue'",
		"call event without matching server start",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestCorpusTracesQueryListsSelectableTraces(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	builder := util.NewDataResponseBuilder()
	err := dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{corpusPathKey: util.StringValue(tracePath)},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  corpusTracesQuery,
			SeriesName: "traces",
			Options:    map[string]*util.V{},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'category_display_name': 'Trace ID'",
		"Prop 'category_display_name': 'Duration'",
		"Prop 'trace_id':",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestTraceQueriesUseSelectedTrace(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	coll, err := dataSource.fetchCollection(context.Background(), tracePath)
	if err != nil {
		t.Fatalf("fetchCollection() failed: %v", err)
	}
	if len(coll.Converted) == 0 {
		t.Fatal("sample has no converted traces")
	}
	selectedTraceID := coll.Converted[0].RawTrace().TraceID

	builder := util.NewDataResponseBuilder()
	err = dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{
			corpusPathKey: util.StringValue(tracePath),
			traceIDKey:    util.StringValue(selectedTraceID),
		},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  traceStatusQuery,
			SeriesName: "trace-status",
			Options:    map[string]*util.V{},
		}, {
			QueryName:  traceDiagnosticsQuery,
			SeriesName: "trace-diagnostics",
			Options:    map[string]*util.V{},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'table_cell': '" + selectedTraceID + "'",
		"smart dependencies",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestTraceQueryRendersTraceVizTrace(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	coll, err := dataSource.fetchCollection(context.Background(), tracePath)
	if err != nil {
		t.Fatalf("fetchCollection() failed: %v", err)
	}
	selectedTraceID := coll.Converted[0].RawTrace().TraceID

	builder := util.NewDataResponseBuilder()
	err = dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{
			corpusPathKey:          util.StringValue(tracePath),
			traceIDKey:             util.StringValue(selectedTraceID),
			expandedCategoryIDsKey: util.StringsValue("c2VydmljZTp0ZXh0LXNlcnZpY2U="),
		},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  traceQuery,
			SeriesName: "trace",
			Options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'axis_type': 'duration'",
		"Prop 'trace_node_type': 0",
		"Prop 'trace_node_type': 1",
		"Prop 'span_id':",
		"Prop 'service_name':",
		"Prop 'payload_type': 'trace_edge_payload'",
		"Prop 'trace_edge_kind': 'critical_path'",
		"Prop 'critical_path_strategy': 'temporal_most_work'",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestTraceQueryDefaultsToCollapsedCategories(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	coll, err := dataSource.fetchCollection(context.Background(), tracePath)
	if err != nil {
		t.Fatalf("fetchCollection() failed: %v", err)
	}
	selectedTraceID := coll.Converted[0].RawTrace().TraceID

	builder := util.NewDataResponseBuilder()
	err = dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{
			corpusPathKey: util.StringValue(tracePath),
			traceIDKey:    util.StringValue(selectedTraceID),
		},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  traceQuery,
			SeriesName: "trace",
			Options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'category_expansion_state': 'collapsed'",
		"Prop 'category_label': '▶ text-service'",
		"Prop 'label_format': ''",
		"Prop 'span_kind': 'synthetic_service'",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestTraceQuerySearchExpandsAndHighlightsMatches(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	coll, err := dataSource.fetchCollection(context.Background(), tracePath)
	if err != nil {
		t.Fatalf("fetchCollection() failed: %v", err)
	}
	selectedTraceID := coll.Converted[0].RawTrace().TraceID

	builder := util.NewDataResponseBuilder()
	err = dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{
			corpusPathKey:           util.StringValue(tracePath),
			traceIDKey:              util.StringValue(selectedTraceID),
			searchKey:               util.StringValue("write_home_timeline_client"),
			expandMatchesKey:        util.StringValue("true"),
			criticalPathStartKey:    util.StringValue(rendertrace.DefaultCriticalPathStart),
			criticalPathEndKey:      util.StringValue(rendertrace.DefaultCriticalPathEnd),
			criticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
		},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  traceQuery,
			SeriesName: "trace",
			Options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'category_expansion_state': 'force_expanded'",
		"Prop 'category_label': '◆ compose-post-service'",
		"Prop 'span_name': 'write_home_timeline_client'",
		"Prop 'primary_color': '#f97316'",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestValidateSearchQueryReturnsParseStatus(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	tests := []struct {
		name        string
		draftSearch string
		want        []string
	}{
		{
			name:        "valid",
			draftSearch: "(.*text.*)",
			want: []string{
				"Prop 'status': 'ok'",
				"Prop 'draft_search': '(.*text.*)'",
				"Prop 'message': ''",
			},
		},
		{
			name:        "parse error",
			draftSearch: "(.*text",
			want: []string{
				"Prop 'status': 'error'",
				"Prop 'draft_search': '(.*text'",
				"Prop 'message':",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := util.NewDataResponseBuilder()
			err := dataSource.HandleDataSeriesRequests(
				context.Background(),
				map[string]*util.V{
					corpusPathKey:  util.StringValue(tracePath),
					draftSearchKey: util.StringValue(test.draftSearch),
				},
				builder,
				[]*util.DataSeriesRequest{{
					QueryName:  validateSearchQuery,
					SeriesName: "validate-search",
					Options:    map[string]*util.V{},
				}},
			)
			if err != nil {
				t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
			}
			got, err := builder.Data()
			if err != nil {
				t.Fatalf("builder.Data() failed: %v", err)
			}
			pretty := got.PrettyPrint()
			for _, want := range test.want {
				if !strings.Contains(pretty, want) {
					t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
				}
			}
		})
	}
}

func TestValidateTransformQueryReturnsTransformStatus(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	coll, err := dataSource.fetchCollection(context.Background(), tracePath)
	if err != nil {
		t.Fatalf("fetchCollection() failed: %v", err)
	}
	selectedTraceID := coll.Converted[0].RawTrace().TraceID
	tests := []struct {
		name          string
		draftTemplate string
		want          []string
	}{
		{
			name:          "valid",
			draftTemplate: "scale spans(**) by .5;",
			want: []string{
				"Prop 'status': 'ok'",
				"Prop 'draft_transform_template': 'scale spans(**) by .5;'",
				"Prop 'message': ''",
			},
		},
		{
			name:          "parse error",
			draftTemplate: "scale spans(** by .5;",
			want: []string{
				"Prop 'status': 'error'",
				"Prop 'draft_transform_template': 'scale spans(** by .5;'",
				"Prop 'message':",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := util.NewDataResponseBuilder()
			err := dataSource.HandleDataSeriesRequests(
				context.Background(),
				map[string]*util.V{
					corpusPathKey:     util.StringValue(tracePath),
					traceIDKey:        util.StringValue(selectedTraceID),
					draftTransformKey: util.StringValue(test.draftTemplate),
				},
				builder,
				[]*util.DataSeriesRequest{{
					QueryName:  validateTransformQuery,
					SeriesName: "validate-transform",
					Options:    map[string]*util.V{},
				}},
			)
			if err != nil {
				t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
			}
			got, err := builder.Data()
			if err != nil {
				t.Fatalf("builder.Data() failed: %v", err)
			}
			pretty := got.PrettyPrint()
			for _, want := range test.want {
				if !strings.Contains(pretty, want) {
					t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
				}
			}
		})
	}
}

func TestTraceStatusQueryUsesTransformedTraceVariant(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	coll, err := dataSource.fetchCollection(context.Background(), tracePath)
	if err != nil {
		t.Fatalf("fetchCollection() failed: %v", err)
	}
	selectedTraceID := coll.Converted[0].RawTrace().TraceID

	builder := util.NewDataResponseBuilder()
	err = dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{
			corpusPathKey:        util.StringValue(tracePath),
			traceIDKey:           util.StringValue(selectedTraceID),
			transformTemplateKey: util.StringValue("scale spans(**) by .5;"),
		},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  traceStatusQuery,
			SeriesName: "trace-status",
			Options:    map[string]*util.V{},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'table_cell': 'ok'",
		"Prop 'table_cell': '" + selectedTraceID + "'",
		"Prop 'table_cell': 'transformed and selected'",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestTraceQueryUsesTemporalDomain(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	coll, err := dataSource.fetchCollection(context.Background(), tracePath)
	if err != nil {
		t.Fatalf("fetchCollection() failed: %v", err)
	}
	selectedTraceID := coll.Converted[0].RawTrace().TraceID

	builder := util.NewDataResponseBuilder()
	err = dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{
			corpusPathKey:          util.StringValue(tracePath),
			traceIDKey:             util.StringValue(selectedTraceID),
			temporalDomainStartKey: util.DurationValue(2_000_000),
			temporalDomainEndKey:   util.DurationValue(4_000_000),
		},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  traceQuery,
			SeriesName: "trace",
			Options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'axis_min': 2ms",
		"Prop 'axis_max': 4ms",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestCriticalPathTraceQueryRendersOvertimeTrace(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	coll, err := dataSource.fetchCollection(context.Background(), tracePath)
	if err != nil {
		t.Fatalf("fetchCollection() failed: %v", err)
	}
	selectedTraceID := coll.Converted[0].RawTrace().TraceID

	builder := util.NewDataResponseBuilder()
	err = dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{
			corpusPathKey: util.StringValue(tracePath),
			traceIDKey:    util.StringValue(selectedTraceID),
		},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  criticalPathTraceQuery,
			SeriesName: "critical-path",
			Options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	for _, want := range []string{
		"Prop 'axis_type': 'duration'",
		"Prop 'trace_view_kind': 'critical_path_overtime'",
		"Prop 'category_label': 'Temporal critical path'",
		"Prop 'span_kind': 'critical_path_leaf'",
		"Prop 'critical_path_strategy': 'temporal_most_work'",
	} {
		if !strings.Contains(pretty, want) {
			t.Fatalf("PrettyPrint() missing %q:\n%s", want, pretty)
		}
	}
}

func TestTraceQueryIgnoresMissingTraceID(t *testing.T) {
	tracePath := filepath.Join("..", "testdata", "compose-post-ct-logs.json")
	dataSource := newTestDataSource(t, "")
	builder := util.NewDataResponseBuilder()
	err := dataSource.HandleDataSeriesRequests(
		context.Background(),
		map[string]*util.V{corpusPathKey: util.StringValue(tracePath)},
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  traceQuery,
			SeriesName: "trace",
			Options:    map[string]*util.V{},
		}},
	)
	if err != nil {
		t.Fatalf("HandleDataSeriesRequests() failed: %v", err)
	}
	got, err := builder.Data()
	if err != nil {
		t.Fatalf("builder.Data() failed: %v", err)
	}
	pretty := got.PrettyPrint()
	if strings.Contains(pretty, "trace_node_type") {
		t.Fatalf("PrettyPrint() rendered trace nodes for empty trace_id:\n%s", pretty)
	}
}
