package datasource

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ilhamster/traceviz/server/go/util"
)

const (
	composePostCorpusPath  = "../testdata/compose-post-ct-logs.json"
	traceyTrace1CorpusPath = "../testdata/tracey-trace1-ct-logs.json"
)

type queryTraceSource struct {
	description string
	build       func(t *testing.T, dataSource *DataSource) map[string]*util.V
}

func corpusSource(corpusPath string) queryTraceSource {
	return queryTraceSource{
		description: corpusPath,
		build: func(t *testing.T, dataSource *DataSource) map[string]*util.V {
			t.Helper()
			return map[string]*util.V{
				corpusPathKey: util.StringValue(corpusPath),
			}
		},
	}
}

func traceIDSource(corpusPath string, traceID string) queryTraceSource {
	return queryTraceSource{
		description: fmt.Sprintf("%s:%s", corpusPath, traceID),
		build: func(t *testing.T, dataSource *DataSource) map[string]*util.V {
			t.Helper()
			return map[string]*util.V{
				corpusPathKey: util.StringValue(corpusPath),
				traceIDKey:    util.StringValue(traceID),
			}
		},
	}
}

func traceIndexSource(corpusPath string, traceIndex int) queryTraceSource {
	return queryTraceSource{
		description: fmt.Sprintf("%s[%d]", corpusPath, traceIndex),
		build: func(t *testing.T, dataSource *DataSource) map[string]*util.V {
			t.Helper()
			coll, err := dataSource.fetchCollection(context.Background(), corpusPath)
			if err != nil {
				t.Fatalf("fetchCollection(%q) failed: %v", corpusPath, err)
			}
			if traceIndex < 0 || traceIndex >= len(coll.Converted) {
				t.Fatalf("trace index %d out of range for %q with %d traces", traceIndex, corpusPath, len(coll.Converted))
			}
			return map[string]*util.V{
				corpusPathKey: util.StringValue(corpusPath),
				traceIDKey:    util.StringValue(coll.Converted[traceIndex].RawTrace().TraceID),
			}
		},
	}
}

func noTraceSource() queryTraceSource {
	return queryTraceSource{
		description: "none",
		build: func(t *testing.T, dataSource *DataSource) map[string]*util.V {
			t.Helper()
			return map[string]*util.V{}
		},
	}
}

type dataSeriesQueryCase struct {
	name          string
	source        queryTraceSource
	globalFilters map[string]*util.V
	queryName     string
	seriesName    string
	options       map[string]*util.V
	want          []string
	dontWant      []string
	wantErr       string
}

func runDataSeriesQueryCase(t *testing.T, dataSource *DataSource, test dataSeriesQueryCase) string {
	t.Helper()
	globalFilters := test.source.build(t, dataSource)
	for key, value := range test.globalFilters {
		globalFilters[key] = value
	}
	builder := util.NewDataResponseBuilder()
	err := dataSource.HandleDataSeriesRequests(
		context.Background(),
		globalFilters,
		builder,
		[]*util.DataSeriesRequest{{
			QueryName:  test.queryName,
			SeriesName: test.seriesName,
			Options:    test.options,
		}},
	)
	if test.wantErr != "" {
		if err == nil {
			t.Fatalf("HandleDataSeriesRequests() succeeded, want error containing %q", test.wantErr)
		}
		if !strings.Contains(err.Error(), test.wantErr) {
			t.Fatalf("HandleDataSeriesRequests() error = %q, want %q", err.Error(), test.wantErr)
		}
		return ""
	}
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
	for _, dontWant := range test.dontWant {
		if strings.Contains(pretty, dontWant) {
			t.Fatalf("PrettyPrint() contains %q:\n%s", dontWant, pretty)
		}
	}
	return pretty
}

func runDataSeriesQueryCases(t *testing.T, tests []dataSeriesQueryCase) {
	t.Helper()
	dataSource := newTestDataSource(t, "")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runDataSeriesQueryCase(t, dataSource, test)
		})
	}
}

func newTestDataSource(t *testing.T, defaultTracePath string) *DataSource {
	t.Helper()
	dataSource, err := New(defaultTracePath, NewFileTraceFetcher(""))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return dataSource
}

func stringValue(str string) *util.V {
	return util.StringValue(url.QueryEscape(str))
}

func TestSupportedDataSeriesQueriesIncludesValidationQueries(t *testing.T) {
	dataSource := newTestDataSource(t, "")
	got := map[string]struct{}{}
	for _, query := range dataSource.SupportedDataSeriesQueries() {
		got[query] = struct{}{}
	}
	for _, want := range []string{
		validateSearchQuery,
		validateTransformQuery,
		validateCriticalPathQuery,
		hierarchyTypesQuery,
	} {
		if _, ok := got[want]; !ok {
			t.Fatalf("SupportedDataSeriesQueries() missing %q; got %v", want, dataSource.SupportedDataSeriesQueries())
		}
	}
}

func TestLoadStatusQuery(t *testing.T) {
	tests := []struct {
		name             string
		defaultTracePath string
		globalFilters    map[string]*util.V
		want             []string
	}{
		{
			name: "loads sample",
			globalFilters: map[string]*util.V{
				tracePathKey: util.StringValue(composePostCorpusPath),
			},
			want: []string{
				"Prop 'table_cell': 'ok'",
				"Prop 'table_cell': 346",
				"Prop 'table_cell': 10851",
				"Prop 'table_cell': 1041",
			},
		},
		{
			name:             "reports load error",
			defaultTracePath: "missing.json",
			want: []string{
				"Prop 'table_cell': 'error'",
				"Prop 'table_cell': 'missing.json'",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataSource := newTestDataSource(t, test.defaultTracePath)
			runDataSeriesQueryCase(t, dataSource, dataSeriesQueryCase{
				source:        noTraceSource(),
				globalFilters: test.globalFilters,
				queryName:     loadStatusQuery,
				seriesName:    "status",
				want:          test.want,
			})
		})
	}
}

func TestFileTraceFetcherRejectsPathsEscapingRoot(t *testing.T) {
	fetcher := NewFileTraceFetcher(filepath.Join("..", "testdata"))
	for _, tracePath := range []string{
		filepath.Join("..", "outside.json"),
		filepath.Join("nested", "..", "..", "outside.json"),
		filepath.Join(string(filepath.Separator), "tmp", "outside.json"),
	} {
		t.Run(tracePath, func(t *testing.T) {
			if _, err := fetcher.resolveTracePath(tracePath); err == nil {
				t.Fatalf("resolveTracePath(%q) succeeded, want error", tracePath)
			}
		})
	}
}

func TestFileTraceFetcherAllowsAbsolutePathsUnderRoot(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "testdata"))
	if err != nil {
		t.Fatalf("filepath.Abs() failed: %v", err)
	}
	tracePath := filepath.Join(root, "tracey-trace1-ct-logs.json")
	fetcher := NewFileTraceFetcher(root)
	got, err := fetcher.resolveTracePath(tracePath)
	if err != nil {
		t.Fatalf("resolveTracePath(%q) failed: %v", tracePath, err)
	}
	if got != tracePath {
		t.Fatalf("resolveTracePath(%q) = %q, want %q", tracePath, got, tracePath)
	}
}

func TestLoadDiagnosticsQuery(t *testing.T) {
	runDataSeriesQueryCases(t, []dataSeriesQueryCase{{
		name:       "lists sample diagnostics",
		source:     corpusSource(composePostCorpusPath),
		queryName:  loadDiagnosticsQuery,
		seriesName: "diagnostics",
		want: []string{
			"Prop 'category_display_name': 'Trace ID'",
			"Prop 'category_display_name': 'Span ID'",
			"Prop 'category_display_name': 'Issue'",
			"call event without matching server start",
		},
	}})
}

func TestCorpusTracesQuery(t *testing.T) {
	runDataSeriesQueryCases(t, []dataSeriesQueryCase{{
		name:       "lists selectable traces",
		source:     corpusSource(composePostCorpusPath),
		queryName:  corpusTracesQuery,
		seriesName: "traces",
		want: []string{
			"Prop 'category_display_name': 'Trace ID'",
			"Prop 'category_display_name': 'Duration'",
			"Prop 'trace_id':",
		},
	}})
}

func TestTraceStatusAndDiagnosticsQueriesUseSelectedTrace(t *testing.T) {
	dataSource := newTestDataSource(t, "")
	globalFilters := traceIndexSource(composePostCorpusPath, 0).build(t, dataSource)
	selectedTraceID, err := util.ExpectStringValue(globalFilters[traceIDKey])
	if err != nil {
		t.Fatalf("selected trace ID is not a string: %v", err)
	}
	builder := util.NewDataResponseBuilder()
	err = dataSource.HandleDataSeriesRequests(
		context.Background(),
		globalFilters,
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
