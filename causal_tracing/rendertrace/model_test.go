package rendertrace

import (
	"context"
	"strings"
	"testing"
	"time"

	testtrace "github.com/ilhamster/tracey/test_trace"
)

func TestClampTemporalDomain(t *testing.T) {
	full := TimeRange{Start: 0, End: 100 * time.Millisecond}
	tests := []struct {
		name      string
		requested TimeRange
		want      TimeRange
	}{
		{
			name:      "inside full range",
			requested: TimeRange{Start: 20 * time.Millisecond, End: 60 * time.Millisecond},
			want:      TimeRange{Start: 20 * time.Millisecond, End: 60 * time.Millisecond},
		},
		{
			name:      "shift right when start is before full range",
			requested: TimeRange{Start: -10 * time.Millisecond, End: 40 * time.Millisecond},
			want:      TimeRange{Start: 0, End: 50 * time.Millisecond},
		},
		{
			name:      "shift left when end is after full range",
			requested: TimeRange{Start: 70 * time.Millisecond, End: 120 * time.Millisecond},
			want:      TimeRange{Start: 50 * time.Millisecond, End: 100 * time.Millisecond},
		},
		{
			name:      "use full range when requested range is too wide",
			requested: TimeRange{Start: -50 * time.Millisecond, End: 150 * time.Millisecond},
			want:      full,
		},
		{
			name:      "use full range for invalid request",
			requested: TimeRange{Start: 80 * time.Millisecond, End: 20 * time.Millisecond},
			want:      full,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := clampTemporalDomain(test.requested, full)
			if got != test.want {
				t.Fatalf("clampTemporalDomain(%+v, %+v) = %+v, want %+v", test.requested, full, got, test.want)
			}
		})
	}
}

func TestFindCriticalPathWithDefaultEndpointControls(t *testing.T) {
	var buildErr error
	tr := testtrace.NewTraceBuilderWithErrorHandler(func(err error) {
		buildErr = err
	}).
		WithRootSpans(
			testtrace.RootSpan(0, 100*time.Nanosecond, "long-overlapping", testtrace.ParentCategories()),
			testtrace.RootSpan(0, 10*time.Nanosecond, "first", testtrace.ParentCategories()),
			testtrace.RootSpan(10*time.Nanosecond, 40*time.Nanosecond, "middle-a", testtrace.ParentCategories()),
			testtrace.RootSpan(40*time.Nanosecond, 90*time.Nanosecond, "middle-b", testtrace.ParentCategories()),
			testtrace.RootSpan(90*time.Nanosecond, 100*time.Nanosecond, "last", testtrace.ParentCategories()),
		).
		Build()
	if buildErr != nil {
		t.Fatalf("failed to build test trace: %v", buildErr)
	}
	got, err := findCriticalPath(context.Background(), tr, RenderRequest{
		HierarchyType: testtrace.None,
	})
	if err != nil {
		t.Fatalf("findCriticalPath() returned error: %v", err)
	}
	var gotSpans []string
	for _, element := range got.CriticalPath {
		gotSpans = append(gotSpans, element.Span().Payload().String())
	}
	wantSpans := []string{"first", "middle-a", "middle-b", "last"}
	if strings.Join(gotSpans, ",") != strings.Join(wantSpans, ",") {
		t.Fatalf("critical path spans = %v, want %v", gotSpans, wantSpans)
	}
	if got.Start != 0 || got.End != 100*time.Nanosecond {
		t.Fatalf("critical path bounds = %v-%v, want 0s-100ns", got.Start, got.End)
	}
}

func TestFindCriticalPathAllowsOneDefaultEndpoint(t *testing.T) {
	var buildErr error
	tr := testtrace.NewTraceBuilderWithErrorHandler(func(err error) {
		buildErr = err
	}).
		WithRootSpans(
			testtrace.RootSpan(0, 10*time.Nanosecond, "first", testtrace.ParentCategories()),
			testtrace.RootSpan(10*time.Nanosecond, 40*time.Nanosecond, "middle", testtrace.ParentCategories()),
			testtrace.RootSpan(40*time.Nanosecond, 100*time.Nanosecond, "last", testtrace.ParentCategories()),
		).
		Build()
	if buildErr != nil {
		t.Fatalf("failed to build test trace: %v", buildErr)
	}
	tests := []struct {
		name  string
		start string
		end   string
	}{
		{
			name:  "explicit start default end",
			start: "**/(first) @ 0%",
			end:   DefaultCriticalPathEnd,
		},
		{
			name:  "default start explicit end",
			start: DefaultCriticalPathStart,
			end:   "**/(last) @ 100%",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := findCriticalPath(context.Background(), tr, RenderRequest{
				HierarchyType:        testtrace.None,
				CriticalPathStart:    test.start,
				CriticalPathEnd:      test.end,
				CriticalPathStrategy: "Temporal Max work (non causal)",
			}); err != nil {
				t.Fatalf("findCriticalPath() returned error: %v", err)
			}
		})
	}
}

func TestFindCriticalPathRejectsAmbiguousEndpoints(t *testing.T) {
	var buildErr error
	tr := testtrace.NewTraceBuilderWithErrorHandler(func(err error) {
		buildErr = err
	}).
		WithRootSpans(
			testtrace.RootSpan(0, 100*time.Nanosecond, "first", testtrace.ParentCategories()),
			testtrace.RootSpan(10*time.Nanosecond, 100*time.Nanosecond, "second", testtrace.ParentCategories()),
		).
		Build()
	if buildErr != nil {
		t.Fatalf("failed to build test trace: %v", buildErr)
	}
	_, err := findCriticalPath(context.Background(), tr, RenderRequest{
		HierarchyType:     testtrace.None,
		CriticalPathStart: "** @ 0%",
		CriticalPathEnd:   "** @ 100% latest",
	})
	if err == nil {
		t.Fatalf("findCriticalPath() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "add earliest or latest to disambiguate") {
		t.Fatalf("findCriticalPath() error = %q, want ambiguous endpoint error", err.Error())
	}
}

func TestFindCriticalPathExplainsSameElementarySpanEndpoints(t *testing.T) {
	var buildErr error
	tr := testtrace.NewTraceBuilderWithErrorHandler(func(err error) {
		buildErr = err
	}).
		WithRootSpans(
			testtrace.RootSpan(0, 100*time.Nanosecond, "long-overlapping", testtrace.ParentCategories()),
			testtrace.RootSpan(0, 10*time.Nanosecond, "first", testtrace.ParentCategories()),
			testtrace.RootSpan(90*time.Nanosecond, 100*time.Nanosecond, "last", testtrace.ParentCategories()),
		).
		Build()
	if buildErr != nil {
		t.Fatalf("failed to build test trace: %v", buildErr)
	}
	_, err := findCriticalPath(context.Background(), tr, RenderRequest{
		HierarchyType:        testtrace.None,
		CriticalPathStart:    "** @0% earliest",
		CriticalPathEnd:      "** @100% latest",
		CriticalPathStrategy: "temporal_most_work",
	})
	if err == nil {
		t.Fatalf("findCriticalPath() succeeded, want error")
	}
	for _, want := range []string{
		"same elementary span long-overlapping",
		"** @0% earliest",
		"** @100% latest",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("findCriticalPath() error = %q, want %q", err.Error(), want)
		}
	}
}
