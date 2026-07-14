package datasource

import (
	"testing"

	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	"github.com/ilhamster/traceviz/server/go/util"
)

func TestTraceQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:   "renders TraceViz trace",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				expandedCategoryIDsKey: util.StringsValue("c2VydmljZTp0ZXh0LXNlcnZpY2U="),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'axis_type': 'duration'",
				"Prop 'trace_node_type': 0",
				"Prop 'trace_node_type': 1",
				"Prop 'span_id':",
				"Prop 'service_name':",
				"Prop 'payload_type': 'trace_edge_payload'",
				"Prop 'trace_edge_kind': 'critical_path'",
				"Prop 'critical_path_strategy': 'temporal_most_work'",
			},
		},
		{
			name:   "renders mark event tooltips",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				focusSpanIDsKey: util.StringsValue("s0.0.0"),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'subspan_kind': 'causal_event'",
				"Prop 'causality_entry_ids': [ 's0.0.0:event:",
				"Prop 'event_type': 'mark'",
				"Prop 'event_display_name': 'Mark'",
				"Prop 'event_label': 'start'",
				"Prop 'event_label': 'end'",
				"Label: start",
				"Label: end",
			},
		},
		{
			name:   "renders dependency edges between focused stack spans",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				focusSpanIDsKey: util.StringsValue("s0.0.0/0", "s0.1.0", "s1.0.0"),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'payload_type': 'trace_edge_payload'",
				"Prop 'trace_edge_kind': 'focus_dependency'",
				"Prop 'trace_edge_node_id': 'focus-dependency:",
				"Prop 'span_id': 's0.0.0/0'",
				"Prop 'span_id': 's0.1.0'",
				"Prop 'span_id': 's1.0.0'",
			},
			wantCount: map[string]int{
				"Prop 'trace_edge_kind': 'focus_dependency'": 6,
			},
			dontWant: []string{
				"Prop 'trace_edge_kind': 'critical_path'",
			},
		},
		{
			name:   "renders focused nested span",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				focusSpanIDsKey: util.StringsValue("s0.0.0/0"),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'span_id': 's0.0.0/0'",
				"Prop 'span_name': '0'",
			},
		},
		{
			name:   "renders focused nested span with cross-service stack",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				focusSpanIDsKey: util.StringsValue("s0.0.0/0", "s1.0.0"),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'span_id': 's0.0.0'",
				"Prop 'span_id': 's0.0.0/0'",
				"Prop 'span_id': 's1.0.0'",
				"Prop 'trace_edge_kind': 'focus_dependency'",
			},
			wantCount: map[string]int{
				"Prop 'trace_edge_kind': 'focus_dependency'": 2,
			},
		},
		{
			name:   "search finds nested numeric span name",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				searchKey:        util.StringValue("**/3"),
				expandMatchesKey: util.StringValue("true"),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'span_id': 's0.0.0/0/3'",
				"Prop 'span_name': '3'",
				"Prop 'primary_color': '#f97316'",
			},
		},
		{
			name:       "defaults to collapsed categories",
			source:     traceIndexSource(composePostCorpusPath, 0),
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'category_expansion_state': 'collapsed'",
				"Prop 'category_label': '▶ text-service'",
				"Prop 'label_format': ''",
				"Prop 'span_kind': 'synthetic_service'",
			},
		},
		{
			name:   "renders selected service spawning hierarchy",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				hierarchyTypeKey: util.StringValue("service_spawn"),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'category_defined_id': 'service-spawn:",
				"Prop 'span_kind': 'synthetic_service'",
			},
		},
		{
			name:   "rolls critical path overlay to collapsed service-spawn ancestor",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				hierarchyTypeKey:        util.StringValue("service_spawn"),
				criticalPathStartKey:    stringValue(rendertrace.DefaultCriticalPathStart),
				criticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				criticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
				showOnlyCriticalPathKey: util.StringValue("true"),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'category_label': '▶ p0'",
				"Prop 'span_kind': 'synthetic_service'",
				"Prop 'trace_edge_kind': 'critical_path'",
				"Prop 'category_id': 'service-spawn:p0'",
			},
		},
		{
			name:   "search expands and highlights matches",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				searchKey:               util.StringValue("write_home_timeline_client"),
				expandMatchesKey:        util.StringValue("true"),
				criticalPathStartKey:    stringValue(rendertrace.DefaultCriticalPathStart),
				criticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				criticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'category_expansion_state': 'force_expanded'",
				"Prop 'category_label': '◆ compose-post-service'",
				"Prop 'span_name': 'write_home_timeline_client'",
				"Prop 'primary_color': '#f97316'",
			},
		},
		{
			name:   "dark theme selects dark trace palette",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				searchKey:               util.StringValue("write_home_timeline_client"),
				expandMatchesKey:        util.StringValue("true"),
				themeKey:                util.StringValue("dark"),
				criticalPathStartKey:    stringValue(rendertrace.DefaultCriticalPathStart),
				criticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				criticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'span_name': 'write_home_timeline_client'",
				"Prop 'primary_color': '#fb923c'",
				"Prop 'trace_edge_kind': 'critical_path'",
				"Prop 'stroke_color': '#c084fc'",
			},
		},
		{
			name:   "invalid theme falls back to light palette",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				searchKey:               util.StringValue("write_home_timeline_client"),
				expandMatchesKey:        util.StringValue("true"),
				themeKey:                util.StringValue("midnight"),
				criticalPathStartKey:    stringValue(rendertrace.DefaultCriticalPathStart),
				criticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				criticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'span_name': 'write_home_timeline_client'",
				"Prop 'primary_color': '#f97316'",
			},
		},
		{
			name:   "can hide non-matching search rows",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				searchKey:               util.StringValue("write_home_timeline_client"),
				expandMatchesKey:        util.StringValue("true"),
				hideNonMatchingKey:      util.StringValue("true"),
				criticalPathStartKey:    stringValue(rendertrace.DefaultCriticalPathStart),
				criticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				criticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'span_name': 'write_home_timeline_client'",
				"Prop 'primary_color': '#f97316'",
			},
			dontWant: []string{
				"Prop 'category_label': '▶ text-service'",
			},
		},
		{
			name:   "can show only critical path rows",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				temporalDomainStartKey:  util.DurationValue(70_000_000),
				temporalDomainEndKey:    util.DurationValue(100_000_000),
				showOnlyCriticalPathKey: util.StringValue("true"),
				criticalPathStartKey:    stringValue(rendertrace.DefaultCriticalPathStart),
				criticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				criticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'category_label': '▶ p0'",
				"Prop 'span_kind': 'synthetic_service'",
				"Prop 'trace_edge_kind': 'critical_path'",
			},
			dontWant: []string{
				"Prop 'category_label': '▶ p1'",
			},
		},
		{
			name:   "uses temporal domain",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				temporalDomainStartKey: util.DurationValue(2_000_000),
				temporalDomainEndKey:   util.DurationValue(4_000_000),
			},
			queryName:  traceQuery,
			seriesName: "trace",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'axis_min': 2ms",
				"Prop 'axis_max': 4ms",
			},
		},
		{
			name:       "ignores missing trace ID",
			source:     corpusSource(composePostCorpusPath),
			queryName:  traceQuery,
			seriesName: "trace",
			want:       []string{},
			dontWant: []string{
				"trace_node_type",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}
