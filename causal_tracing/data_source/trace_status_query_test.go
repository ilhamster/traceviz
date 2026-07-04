package datasource

import (
	"testing"

	"github.com/ilhamster/traceviz/server/go/util"
)

func TestTraceStatusQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:   "uses transformed trace variant",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				transformTemplateKey: util.StringValue("scale spans(**) by .5;"),
			},
			queryName:  traceStatusQuery,
			seriesName: "trace-status",
			want: []string{
				"Prop 'table_cell': 'ok'",
				"Prop 'table_cell': 'transformed and selected'",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}

func TestHierarchyTypesQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:       "lists selected trace hierarchies",
			source:     traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			queryName:  hierarchyTypesQuery,
			seriesName: "hierarchy-types",
			want: []string{
				"Prop 'hierarchy_type': 'service'",
				"Prop 'hierarchy_type': 'process'",
				"Prop 'hierarchy_type': 'service_spawn'",
				"Prop 'table_cell': 'Service spawning hierarchy'",
			},
			dontWant: []string{
				"Prop 'hierarchy_type': 'span'",
				"Prop 'hierarchy_type': 'otel_span'",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}

func TestCriticalPathStrategiesQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:       "lists Tracey common critical path strategies",
			source:     traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			queryName:  criticalPathStrategiesQuery,
			seriesName: "critical-path-strategies",
			want: []string{
				"Prop 'critical_path_strategy': 'Temporal Max work (non causal)'",
				"Prop 'critical_path_strategy_name': 'temporal_most_work'",
				"Prop 'critical_path_strategy_description': 'Temporal Max work (non causal)'",
				"Prop 'critical_path_strategy': 'Maximize work'",
				"Prop 'critical_path_strategy_name': 'most_work'",
				"Prop 'critical_path_strategy_description': 'Maximize work'",
				"Prop 'critical_path_strategy': 'Prefer traversing causal dependencies'",
				"Prop 'critical_path_strategy_name': 'causal'",
				"Prop 'critical_path_strategy_description': 'Prefer traversing causal dependencies'",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}
