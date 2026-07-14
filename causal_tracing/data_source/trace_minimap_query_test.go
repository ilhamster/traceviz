package datasource

import (
	"testing"

	"github.com/ilhamster/traceviz/server/go/util"
)

func TestTraceMinimapQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:   "renders reduced detail over the full temporal domain",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				temporalDomainStartKey: util.DurationValue(10_000_000),
				temporalDomainEndKey:   util.DurationValue(20_000_000),
			},
			queryName:  traceMinimapQuery,
			seriesName: "minimap",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(800),
			},
			want: []string{
				"Prop 'axis_min': 0s",
				"Prop 'axis_max': 100ms",
				"Prop 'span_kind': 'synthetic_service'",
				"Prop 'label_format': ''",
			},
			dontWant: []string{
				"Prop 'tooltip':",
				"Prop 'subspan_kind': 'causal_event'",
				"Prop 'trace_edge_kind':",
			},
		},
		{
			name:   "projects hidden matches onto collapsed summaries",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				searchKey: util.StringValue("**/3"),
			},
			queryName:  traceMinimapQuery,
			seriesName: "minimap",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(800),
			},
			want: []string{
				"Prop 'span_kind': 'synthetic_service'",
				"Prop 'subspan_kind': 'minimap_match'",
				"Prop 'trace_minimap_highlight_color': '#f97316'",
				"Prop 'trace_minimap_match_kind': 'requires_expansion'",
				"Prop 'trace_start': 40ms",
				"Prop 'trace_end': 70ms",
			},
		},
		{
			name:   "preserves search matches as minimap highlights",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				searchKey:        util.StringValue("**/3"),
				expandMatchesKey: util.StringValue("true"),
			},
			queryName:  traceMinimapQuery,
			seriesName: "minimap",
			options: map[string]*util.V{
				traceViewWidthPxKey: util.IntegerValue(800),
			},
			want: []string{
				"Prop 'span_id': 's0.0.0/0/3'",
				"Prop 'subspan_kind': 'minimap_match'",
				"Prop 'trace_minimap_highlight_color': '#f97316'",
				"Prop 'trace_minimap_match_kind': 'direct'",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}
