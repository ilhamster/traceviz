package concurrency

import (
	"testing"

	"github.com/ilhamster/tracey/trace"
)

func TestFromIntervalsBuildsPiecewiseProfile(t *testing.T) {
	profile := FromIntervals("category", []Interval{
		{Start: 0, End: 10, Weight: 1},
		{Start: 5, End: 15, Weight: 1},
	})

	if profile.ID != "category" {
		t.Fatalf("profile.ID = %q, want category", profile.ID)
	}
	if profile.Start != 0 || profile.End != 15 {
		t.Fatalf("profile range = %f..%f, want 0..15", profile.Start, profile.End)
	}
	if profile.Peak != 2 {
		t.Fatalf("profile.Peak = %d, want 2", profile.Peak)
	}
	want := []Segment{
		{Start: 0, End: 5, Count: 1},
		{Start: 5, End: 10, Count: 2},
		{Start: 10, End: 15, Count: 1},
	}
	if len(profile.Segments) != len(want) {
		t.Fatalf("len(profile.Segments) = %d, want %d", len(profile.Segments), len(want))
	}
	for idx := range want {
		if profile.Segments[idx] != want[idx] {
			t.Fatalf("profile.Segments[%d] = %+v, want %+v", idx, profile.Segments[idx], want[idx])
		}
	}
}

func TestMergeSumsProfiles(t *testing.T) {
	left := FromIntervals("left", []Interval{{Start: 0, End: 10, Weight: 1}})
	right := FromIntervals("right", []Interval{{Start: 5, End: 15, Weight: 1}})

	profile := Merge("merged", left, right)

	if profile.ID != "merged" {
		t.Fatalf("profile.ID = %q, want merged", profile.ID)
	}
	if profile.Peak != 2 {
		t.Fatalf("profile.Peak = %d, want 2", profile.Peak)
	}
	if len(profile.Segments) != 3 {
		t.Fatalf("len(profile.Segments) = %d, want 3", len(profile.Segments))
	}
	if profile.Segments[1].Count != 2 {
		t.Fatalf("overlap count = %d, want 2", profile.Segments[1].Count)
	}
}

func TestBucketsDownsamplesAndClipsProfile(t *testing.T) {
	profile := FromIntervals("category", []Interval{
		{Start: 0, End: 10, Weight: 1},
		{Start: 5, End: 15, Weight: 1},
	})

	buckets := profile.Buckets(BucketOptions{
		Domain:                Range{Start: 0, End: 20},
		Clip:                  Range{Start: 5, End: 15},
		WidthPx:               16,
		MinimumFeatureWidthPx: 4,
	})

	if len(buckets) != 2 {
		t.Fatalf("len(buckets) = %d, want 2: %+v", len(buckets), buckets)
	}
	first := buckets[0]
	if first.Start != 5 || first.End != 10 {
		t.Fatalf("first bucket range = %f..%f, want 5..10", first.Start, first.End)
	}
	if first.Peak != 2 {
		t.Fatalf("first bucket.Peak = %d, want 2", first.Peak)
	}
	if first.Avg != 2 {
		t.Fatalf("first bucket.Avg = %f, want 2", first.Avg)
	}
	second := buckets[1]
	if second.Start != 10 || second.End != 15 {
		t.Fatalf("second bucket range = %f..%f, want 10..15", second.Start, second.End)
	}
	if second.Peak != 1 {
		t.Fatalf("second bucket.Peak = %d, want 1", second.Peak)
	}
	if second.Avg != 1 {
		t.Fatalf("second bucket.Avg = %f, want 1", second.Avg)
	}
}

type testPayload string

func (tp testPayload) String() string {
	return string(tp)
}

type testNamer struct{}

func (testNamer) CategoryName(category trace.Category[float64, testPayload, testPayload, testPayload]) string {
	return string(category.Payload())
}

func (testNamer) CategoryUniqueID(category trace.Category[float64, testPayload, testPayload, testPayload]) string {
	return "cat:" + string(category.Payload())
}

func (testNamer) SpanName(span trace.Span[float64, testPayload, testPayload, testPayload]) string {
	return string(span.Payload())
}

func (testNamer) SpanUniqueID(span trace.Span[float64, testPayload, testPayload, testPayload]) string {
	return "span:" + string(span.Payload())
}

func (testNamer) HierarchyTypes() *trace.HierarchyTypes {
	return trace.NewHierarchyTypes().With(trace.FirstUserDefinedHierarchyType, "test", "Test hierarchy")
}

func (testNamer) DependencyTypes() *trace.DependencyTypes {
	return trace.NewDependencyTypes()
}

func (testNamer) MomentString(moment float64) string {
	return trace.DoubleComparator.DurationString(moment)
}

func TestProfilesByTraceCategoriesUsesTraceNamerAndComparator(t *testing.T) {
	tr := trace.NewTrace[float64, testPayload, testPayload, testPayload](
		trace.DoubleComparator,
		testNamer{},
	)
	category := tr.NewRootCategory(trace.FirstUserDefinedHierarchyType, testPayload("root"))
	span := tr.NewRootSpan(100, 120, testPayload("span"))
	if err := category.AddRootSpan(span); err != nil {
		t.Fatalf("AddRootSpan() failed: %v", err)
	}
	if err := span.Suspend(trace.DoubleComparator, 105, 115); err != nil {
		t.Fatalf("Suspend() failed: %v", err)
	}

	profiles := ProfilesByTraceCategories(tr, 100, trace.FirstUserDefinedHierarchyType)
	profile := profiles["cat:root"]
	if profile == nil {
		t.Fatalf("profiles missing cat:root: %+v", profiles)
	}
	want := []Segment{
		{Start: 0, End: 5, Count: 1},
		{Start: 15, End: 20, Count: 1},
	}
	if len(profile.Segments) != len(want) {
		t.Fatalf("len(profile.Segments) = %d, want %d: %+v", len(profile.Segments), len(want), profile.Segments)
	}
	for idx := range want {
		if profile.Segments[idx] != want[idx] {
			t.Fatalf("profile.Segments[%d] = %+v, want %+v", idx, profile.Segments[idx], want[idx])
		}
	}
}
