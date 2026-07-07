package datasource

import (
	"testing"

	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	"github.com/ilhamster/traceviz/server/go/util"
)

func TestCriticalPathTraceQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:       "renders overtime trace",
			source:     traceIndexSource(composePostCorpusPath, 0),
			queryName:  criticalPathTraceQuery,
			seriesName: "critical-path",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'axis_type': 'duration'",
				"Prop 'trace_view_kind': 'critical_path_overtime'",
				"Prop 'category_label': 'Temporal critical path'",
				"Prop 'span_kind': 'critical_path_leaf'",
				"Prop 'critical_path_strategy': 'temporal_most_work'",
			},
		},
		{
			name:   "renders communication delay for transformed causal dependency gap",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				transformTemplateKey:    util.StringValue("scale spans(**/3) by .5;"),
				criticalPathStartKey:    stringValue(rendertrace.DefaultCriticalPathStart),
				criticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				criticalPathStrategyKey: util.StringValue("causal"),
			},
			queryName:  criticalPathTraceQuery,
			seriesName: "critical-path",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(1200),
			},
			want: []string{
				"Prop 'span_kind': 'critical_path_communication_delay'",
				"Prop 'span_name': 'Communications delay'",
				"Critical path is waiting on a dependency edge.",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}
