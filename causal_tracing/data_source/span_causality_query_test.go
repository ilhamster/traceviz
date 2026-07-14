package datasource

import (
	"testing"

	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	"github.com/ilhamster/traceviz/server/go/util"
)

func TestSpanCausalityQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:   "lists suspends and dependency entries",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				focusSpanIDsKey: util.StringsValue("s0.0.0/0/3"),
			},
			queryName:  spanCausalityQuery,
			seriesName: "span-causality",
			want: []string{
				"Prop 'secondary_color': 'rgba(148, 163, 184, 0.30)'",
				"Prop 'causality_entry_id': 's0.0.0/0/3:suspend:",
				"Prop 'causality_entry_id': 's0.0.0/0/3:event:",
				"Prop 'table_cell': 'suspend'",
				"Prop 'table_cell': 10ms",
				"Prop 'table_cell': 'incoming_dependency'",
				"Prop 'table_cell': 'outgoing_dependency'",
				"Prop 'table_cell': 'call'",
				"Prop 'table_cell': 'return'",
				"Prop 'table_cell': '0-calls-3'",
				"Prop 'table_cell': 'signal'",
				"Prop 'table_cell': 'signal-to-3'",
				"Prop 'table_cell': 's0.1.0'",
			},
		},
		{
			name:   "lists marks and call-return entries",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				focusSpanIDsKey: util.StringsValue("s0.0.0"),
			},
			queryName:  spanCausalityQuery,
			seriesName: "span-causality",
			want: []string{
				"Prop 'causality_entry_id': 's0.0.0:event:",
				"Prop 'table_cell': 'event'",
				"Prop 'table_cell': 'mark'",
				"Prop 'table_cell': 'start'",
				"Prop 'table_cell': 'end'",
				"Prop 'table_cell': 'outgoing_dependency'",
				"Prop 'table_cell': 'incoming_dependency'",
				"Prop 'table_cell': 'call'",
				"Prop 'table_cell': 'return'",
				"Prop 'table_cell': 's0.0.0-calls-0'",
			},
			wantCount: map[string]int{
				"Prop 'table_cell': 'mark'":                2,
				"Prop 'table_cell': 'start'":               1,
				"Prop 'table_cell': 'end'":                 1,
				"Prop 'table_cell': 'outgoing_dependency'": 1,
				"Prop 'table_cell': 'incoming_dependency'": 1,
				"Prop 'table_cell': 's0.0.0-calls-0'":      2,
			},
		},
		{
			name:   "uses dark-theme row highlights",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				focusSpanIDsKey: util.StringsValue("s0.0.0"),
				themeKey:        util.StringValue(string(rendertrace.ThemeDark)),
			},
			queryName:  spanCausalityQuery,
			seriesName: "span-causality",
			want: []string{
				"Prop 'secondary_color': 'rgba(100, 116, 139, 0.48)'",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}
