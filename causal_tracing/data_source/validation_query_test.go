package datasource

import (
	"testing"

	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	"github.com/ilhamster/traceviz/server/go/util"
)

func TestValidateSearchQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:   "valid",
			source: corpusSource(composePostCorpusPath),
			globalFilters: map[string]*util.V{
				draftSearchKey: util.StringValue("(.*text.*)"),
			},
			queryName:  validateSearchQuery,
			seriesName: "validate-search",
			want: []string{
				"Prop 'status': 'ok'",
				"Prop 'draft_search': '(.*text.*)'",
				"Prop 'message': ''",
			},
		},
		{
			name:   "parse error",
			source: corpusSource(composePostCorpusPath),
			globalFilters: map[string]*util.V{
				draftSearchKey: util.StringValue("(.*text"),
			},
			queryName:  validateSearchQuery,
			seriesName: "validate-search",
			want: []string{
				"Prop 'status': 'error'",
				"Prop 'draft_search': '(.*text'",
				"Prop 'message':",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}

func TestValidateTransformQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:   "valid",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				draftTransformKey: util.StringValue("scale spans(**) by .5;"),
			},
			queryName:  validateTransformQuery,
			seriesName: "validate-transform",
			want: []string{
				"Prop 'status': 'ok'",
				"Prop 'draft_transform_template': 'scale spans(**) by .5;'",
				"Prop 'message': ''",
			},
		},
		{
			name:   "parse error",
			source: traceIndexSource(composePostCorpusPath, 0),
			globalFilters: map[string]*util.V{
				draftTransformKey: util.StringValue("scale spans(** by .5;"),
			},
			queryName:  validateTransformQuery,
			seriesName: "validate-transform",
			want: []string{
				"Prop 'status': 'error'",
				"Prop 'draft_transform_template': 'scale spans(** by .5;'",
				"Prop 'message':",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}

func TestValidateCriticalPathQuery(t *testing.T) {
	tests := []dataSeriesQueryCase{
		{
			name:   "valid default",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				draftCriticalPathStartKey:    stringValue(rendertrace.DefaultCriticalPathStart),
				draftCriticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				draftCriticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
			},
			queryName:  validateCriticalPathQuery,
			seriesName: "validate-critical-path",
			want: []string{
				"Prop 'status': 'ok'",
				"Prop 'draft_critical_path_start': ''",
				"Prop 'draft_critical_path_end': ''",
				"Prop 'draft_critical_path_strategy': 'temporal_most_work'",
				"Prop 'message': ''",
			},
		},
		{
			name:   "parse error",
			source: traceIDSource(traceyTrace1CorpusPath, "tracey-trace1"),
			globalFilters: map[string]*util.V{
				draftCriticalPathStartKey:    stringValue("(**"),
				draftCriticalPathEndKey:      stringValue(rendertrace.DefaultCriticalPathEnd),
				draftCriticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
			},
			queryName:  validateCriticalPathQuery,
			seriesName: "validate-critical-path",
			want: []string{
				"Prop 'status': 'error'",
				"Prop 'draft_critical_path_start': '(**'",
				"Prop 'message':",
			},
		},
		{
			name:   "compose root envelope explicit endpoints",
			source: traceIDSource(composePostCorpusPath, "648ffbcd83cbfaa"),
			globalFilters: map[string]*util.V{
				draftCriticalPathStartKey:    stringValue("** @0% earliest"),
				draftCriticalPathEndKey:      stringValue("** @100% latest"),
				draftCriticalPathStrategyKey: util.StringValue(rendertrace.DefaultCriticalPathStrategy),
			},
			queryName:  validateCriticalPathQuery,
			seriesName: "validate-critical-path",
			want: []string{
				"Prop 'status': 'error'",
				"same elementary span /wrk2-api/post/compose",
				"** @0% earliest",
				"** @100% latest",
			},
		},
	}
	runDataSeriesQueryCases(t, tests)
}
