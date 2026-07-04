package datasource

import (
	"testing"

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
	}
	runDataSeriesQueryCases(t, tests)
}
