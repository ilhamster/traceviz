package extendedotel

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ilhamster/traceviz/causal_tracing/concurrency"
	"github.com/ilhamster/traceviz/causal_tracing/rendertrace"
	criticalpath "github.com/ilhamster/tracey/critical_path"
	"github.com/ilhamster/tracey/trace"
)

func TestConvertExtendedOtelTraceSample(t *testing.T) {
	response, err := LoadRawResponseFile(filepath.Join("..", "testdata", "compose-post-ct-logs.json"))
	if err != nil {
		t.Fatalf("LoadRawResponseFile() failed: %v", err)
	}
	if len(response.Data) == 0 {
		t.Fatal("sample contains no traces")
	}

	converted, err := ConvertExtendedOtelTrace(response.Data[0])
	if err != nil {
		t.Fatalf("ConvertExtendedOtelTrace() failed: %v", err)
	}
	gotTrace := converted.Trace()
	if got, want := len(gotTrace.RootSpans()), len(response.Data[0].Spans); got != want {
		t.Fatalf("converted root spans = %d, want %d", got, want)
	}

	requireDependencyType(t, gotTrace, DependencyRPC)

	span := converted.SpanByID("5ac49ee5b962ac09")
	if span == nil {
		t.Fatal("expected compose_post_server span to be converted")
	}
	if len(span.ElementarySpans()) < 2 {
		t.Fatalf("compose_post_server elementary spans = %d, want at least 2 after suspends", len(span.ElementarySpans()))
	}
}

func TestConvertTraceyTrace1Fixture(t *testing.T) {
	response, err := LoadRawResponseFile(filepath.Join("..", "testdata", "tracey-trace1-ct-logs.json"))
	if err != nil {
		t.Fatalf("LoadRawResponseFile() failed: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("fixture traces = %d, want 1", len(response.Data))
	}

	converted, err := ConvertExtendedOtelTrace(response.Data[0])
	if err != nil {
		t.Fatalf("ConvertExtendedOtelTrace() failed: %v", err)
	}
	if got := converted.Diagnostics(); len(got) != 0 {
		t.Fatalf("Diagnostics() = %v, want none", got)
	}
	gotTrace := converted.Trace()
	if got, want := len(gotTrace.RootSpans()), 3; got != want {
		t.Fatalf("converted root spans = %d, want %d", got, want)
	}
	for _, want := range []trace.DependencyType{trace.Call, trace.Return, DependencySpawn, DependencySend, DependencySignal} {
		requireDependencyType(t, gotTrace, want)
	}

	rootSpan := converted.SpanByID("s0.0.0")
	if rootSpan == nil {
		t.Fatal("expected s0.0.0 span to be converted")
	}
	if got, want := markCount(rootSpan), 2; got != want {
		t.Fatalf("s0.0.0 marks = %d, want %d", got, want)
	}

	childSpan := converted.SpanByID("s0.0.0/0")
	if childSpan == nil {
		t.Fatal("expected s0.0.0/0 span to be converted")
	}
	if got, want := childSpan.ParentSpan(), rootSpan; got != want {
		t.Fatalf("s0.0.0/0 parent = %v, want s0.0.0", got)
	}

	suspendedSpan := converted.SpanByID("s0.0.0/0/3")
	if suspendedSpan == nil {
		t.Fatal("expected s0.0.0/0/3 span to be converted")
	}
	if got, want := suspendedSpan.ParentSpan(), childSpan; got != want {
		t.Fatalf("s0.0.0/0/3 parent = %v, want s0.0.0/0", got)
	}
	if !hasElementaryGap(suspendedSpan, 50*time.Millisecond, 60*time.Millisecond) {
		t.Fatal("expected s0.0.0/0/3 to be suspended from 50ms to 60ms")
	}
	if !hasIncomingAt(suspendedSpan, 60*time.Millisecond) {
		t.Fatal("expected signal dependency destination at end of suspend")
	}
}

func TestConvertTraceyTrace1FixtureBuildsServiceSpawnHierarchy(t *testing.T) {
	response, err := LoadRawResponseFile(filepath.Join("..", "testdata", "tracey-trace1-ct-logs.json"))
	if err != nil {
		t.Fatalf("LoadRawResponseFile() failed: %v", err)
	}
	converted, err := ConvertExtendedOtelTrace(response.Data[0])
	if err != nil {
		t.Fatalf("ConvertExtendedOtelTrace() failed: %v", err)
	}
	roots := converted.Trace().RootCategories(ServiceSpawnHierarchyType)
	p0Root := findCategoryByPayloadID(roots, "service-spawn:p0")
	if p0Root == nil {
		t.Fatalf("missing p0 service-spawn root in %v", categoryPayloadIDs(roots))
	}
	for _, rootSpanID := range []string{"s0.0.0", "s0.1.0"} {
		rootSpanCategory := findCategoryByPayloadID(
			p0Root.ChildCategories(),
			"service-spawn:p0:root-span:"+rootSpanID,
		)
		if rootSpanCategory == nil {
			t.Fatalf("missing p0 root-span category %q in %v", rootSpanID, categoryPayloadIDs(p0Root.ChildCategories()))
		}
		if got := len(rootSpanCategory.RootSpans()); got != 1 {
			t.Fatalf("p0 root-span category %q spans = %d, want 1", rootSpanID, got)
		}
	}
	p1Service := findCategoryByPayloadID(p0Root.ChildCategories(), "service-spawn:p0:service:p1")
	if p1Service == nil {
		t.Fatalf("missing spawned p1 service category under p0 in %v", categoryPayloadIDs(p0Root.ChildCategories()))
	}
	p1RootSpan := findCategoryByPayloadID(
		p1Service.ChildCategories(),
		"service-spawn:p0:service:p1:root-span:s1.0.0",
	)
	if p1RootSpan == nil {
		t.Fatalf("missing p1 root-span category in %v", categoryPayloadIDs(p1Service.ChildCategories()))
	}
}

func TestConvertComposeFixtureBuildsServiceSpawnHierarchy(t *testing.T) {
	response, err := LoadRawResponseFile(filepath.Join("..", "testdata", "compose-post-ct-logs.json"))
	if err != nil {
		t.Fatalf("LoadRawResponseFile() failed: %v", err)
	}
	converted, err := ConvertExtendedOtelTrace(response.Data[0])
	if err != nil {
		t.Fatalf("ConvertExtendedOtelTrace() failed: %v", err)
	}
	gotTrace := converted.Trace()
	roots := gotTrace.RootCategories(ServiceSpawnHierarchyType)
	if got := len(roots); got == 0 {
		t.Fatalf("service spawn root categories = %d, want at least 1", got)
	}
	nested := firstDescendantCategoryWithRootSpans(roots)
	if nested == nil {
		t.Fatalf("service spawn hierarchy has no nested category with root spans")
	}
	payload := nested.Payload()
	if payload == nil {
		t.Fatal("nested service-spawn category has nil payload")
	}
	if !strings.Contains(payload.ID, ":service:") {
		t.Fatalf("nested service-spawn category ID = %q, want descendant service path", payload.ID)
	}
}

func TestBuildCriticalPathFrameTreeMergesPrefixesWithoutMisclassifyingSuspends(t *testing.T) {
	response, err := LoadRawResponseFile(filepath.Join("..", "testdata", "tracey-trace1-ct-logs.json"))
	if err != nil {
		t.Fatalf("LoadRawResponseFile() failed: %v", err)
	}
	converted, err := ConvertExtendedOtelTrace(response.Data[0])
	if err != nil {
		t.Fatalf("ConvertExtendedOtelTrace() failed: %v", err)
	}
	rootSpan := converted.SpanByID("s0.0.0")
	childSpan := converted.SpanByID("s0.0.0/0")
	if rootSpan == nil || childSpan == nil {
		t.Fatalf("fixture spans missing: root=%v child=%v", rootSpan, childSpan)
	}

	path := &criticalpath.Path[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{
		CriticalPath: []criticalpath.PathElement[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{
			testCriticalPathElement{span: rootSpan, start: 0, end: 10 * time.Millisecond},
			testCriticalPathElement{span: childSpan, start: 20 * time.Millisecond, end: 30 * time.Millisecond},
			testCriticalPathElement{span: childSpan, start: 40 * time.Millisecond, end: 50 * time.Millisecond},
		},
	}
	roots := buildCriticalPathFrameTree(path, ServiceHierarchyType, converted.namer)
	if got, want := len(roots), 1; got != want {
		t.Fatalf("root frame count = %d, want %d", got, want)
	}
	serviceCategoryFrame := roots[0]
	if got, want := serviceCategoryFrame.start, time.Duration(0); got != want {
		t.Fatalf("service category frame start = %v, want %v", got, want)
	}
	if got, want := serviceCategoryFrame.end, 50*time.Millisecond; got != want {
		t.Fatalf("service category frame end = %v, want %v", got, want)
	}
	if got, want := len(serviceCategoryFrame.children), 1; got != want {
		t.Fatalf("service category child frame count = %d, want %d", got, want)
	}

	operationCategoryFrame := serviceCategoryFrame.children[0]
	if got, want := operationCategoryFrame.start, time.Duration(0); got != want {
		t.Fatalf("operation category frame start = %v, want %v", got, want)
	}
	if got, want := operationCategoryFrame.end, 50*time.Millisecond; got != want {
		t.Fatalf("operation category frame end = %v, want %v", got, want)
	}
	if got, want := len(operationCategoryFrame.children), 1; got != want {
		t.Fatalf("operation category child frame count = %d, want %d", got, want)
	}

	rootLeaf := operationCategoryFrame.children[0]
	if got, want := rootLeaf.start, time.Duration(0); got != want {
		t.Fatalf("root leaf start = %v, want %v", got, want)
	}
	if got, want := rootLeaf.end, 50*time.Millisecond; got != want {
		t.Fatalf("root leaf end = %v, want %v", got, want)
	}
	if got, want := len(rootLeaf.children), 1; got != want {
		t.Fatalf("root leaf child frame count = %d, want %d", got, want)
	}
	if got := len(rootLeaf.gaps); got != 0 {
		t.Fatalf("root leaf no-longer-running gaps = %d, want 0", got)
	}

	childLeaf := rootLeaf.children[0]
	if got, want := childLeaf.start, 20*time.Millisecond; got != want {
		t.Fatalf("child leaf start = %v, want %v", got, want)
	}
	if got, want := childLeaf.end, 50*time.Millisecond; got != want {
		t.Fatalf("child leaf end = %v, want %v", got, want)
	}
	if got := len(childLeaf.gaps); got != 0 {
		t.Fatalf("child leaf no-longer-running gaps = %d, want 0", got)
	}
}

func TestBuildCriticalPathFrameTreeNestsCommunicationDelayUnderOriginStack(t *testing.T) {
	response, err := LoadRawResponseFile(filepath.Join("..", "testdata", "tracey-trace1-ct-logs.json"))
	if err != nil {
		t.Fatalf("LoadRawResponseFile() failed: %v", err)
	}
	converted, err := ConvertExtendedOtelTrace(response.Data[0])
	if err != nil {
		t.Fatalf("ConvertExtendedOtelTrace() failed: %v", err)
	}
	originSpan := converted.SpanByID("s1.0.0")
	destinationSpan := converted.SpanByID("s0.1.0")
	if originSpan == nil || destinationSpan == nil {
		t.Fatalf("fixture spans missing: origin=%v destination=%v", originSpan, destinationSpan)
	}

	path := &criticalpath.Path[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{
		CriticalPath: []criticalpath.PathElement[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{
			testCriticalPathElement{span: originSpan, start: 30 * time.Millisecond, end: 35 * time.Millisecond},
			testCriticalPathElement{span: destinationSpan, start: 40 * time.Millisecond, end: 45 * time.Millisecond},
		},
	}
	roots := buildCriticalPathFrameTree(path, ServiceSpawnHierarchyType, converted.namer)
	delay := firstCriticalPathNodeWithKind(roots, criticalPathCommunicationDelayFrameKind)
	if delay == nil {
		t.Fatalf("critical path frame tree has no communication delay: %#v", roots)
	}
	if delay.parent == nil {
		t.Fatal("communication delay parent is nil, want origin stack leaf")
	}
	if got, want := delay.parent.frame.span, originSpan; got != want {
		t.Fatalf("communication delay parent span = %v, want origin span", got)
	}
	wantStack := []string{"p0", "p1", "s1.0.0", "s1.0.0", "Communications delay"}
	if got := criticalPathFrameStackNames(delay); !reflect.DeepEqual(got, wantStack) {
		t.Fatalf("communication delay stack = %v, want %v", got, wantStack)
	}
}

func TestRenderableConcurrencyBucketsMergeAdjacentSameColorBuckets(t *testing.T) {
	profile := concurrency.FromIntervals("profile", []concurrency.Interval{
		{Start: 0, End: float64(10 * time.Millisecond), Weight: 1},
		{Start: float64(10 * time.Millisecond), End: float64(20 * time.Millisecond), Weight: 2},
		{Start: float64(20 * time.Millisecond), End: float64(30 * time.Millisecond), Weight: 100},
	})

	got := renderableConcurrencyBuckets(
		profile,
		"svc",
		rendertrace.TimeRange{Start: 0, End: 30 * time.Millisecond},
		rendertrace.RenderView{
			Request: rendertrace.RenderRequest{
				Theme:                 rendertrace.ThemeLight,
				TraceViewRangePx:      12,
				MinimumFeatureWidthPx: 4,
			},
			TemporalDomain: rendertrace.TimeRange{Start: 0, End: 30 * time.Millisecond},
		},
	)
	if gotLen, wantLen := len(got), 2; gotLen != wantLen {
		t.Fatalf("renderableConcurrencyBuckets() length = %d, want %d: %+v", gotLen, wantLen, got)
	}
	first := got[0]
	if first.Start != 0 || first.End != 20*time.Millisecond {
		t.Fatalf("first merged bucket range = %v-%v, want 0s-20ms", first.Start, first.End)
	}
	if first.Avg != 1.5 {
		t.Fatalf("first merged bucket avg = %v, want 1.5", first.Avg)
	}
	if first.Peak != 2 {
		t.Fatalf("first merged bucket peak = %d, want 2", first.Peak)
	}
	if got[1].Peak != 100 {
		t.Fatalf("second bucket peak = %d, want 100", got[1].Peak)
	}
}

func firstCriticalPathNodeWithKind(
	nodes []*criticalPathFrameNode,
	kind criticalPathFrameKind,
) *criticalPathFrameNode {
	for _, node := range nodes {
		if node.frame.kind == kind {
			return node
		}
		if child := firstCriticalPathNodeWithKind(node.children, kind); child != nil {
			return child
		}
	}
	return nil
}

func criticalPathFrameStackNames(node *criticalPathFrameNode) []string {
	if node == nil {
		return nil
	}
	var reversed []string
	for cursor := node; cursor != nil; cursor = cursor.parent {
		name := cursor.frame.name
		if name == "" && cursor.frame.categoryPayload != nil {
			name = cursor.frame.categoryPayload.Name
		}
		if name == "" && cursor.frame.payload != nil {
			name = cursor.frame.payload.SpanID
		}
		reversed = append(reversed, name)
	}
	ret := make([]string, 0, len(reversed))
	for idx := len(reversed) - 1; idx >= 0; idx-- {
		ret = append(ret, reversed[idx])
	}
	return ret
}

func firstDescendantCategoryWithRootSpans(
	categories []trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	for _, category := range categories {
		for _, child := range category.ChildCategories() {
			if len(child.RootSpans()) > 0 {
				return child
			}
			if nested := firstDescendantCategoryWithRootSpans(
				[]trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{child},
			); nested != nil {
				return nested
			}
		}
	}
	return nil
}

func findCategoryByPayloadID(
	categories []trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	id string,
) trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	for _, category := range categories {
		payload := category.Payload()
		if payload != nil && payload.ID == id {
			return category
		}
	}
	return nil
}

func categoryPayloadIDs(
	categories []trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) []string {
	ids := make([]string, 0, len(categories))
	for _, category := range categories {
		payload := category.Payload()
		if payload == nil {
			ids = append(ids, "<nil>")
			continue
		}
		ids = append(ids, payload.ID)
	}
	return ids
}

func hasElementaryGap(span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload], start, end time.Duration) bool {
	elementarySpans := span.ElementarySpans()
	for idx := 0; idx < len(elementarySpans)-1; idx++ {
		if elementarySpans[idx].End() == start && elementarySpans[idx+1].Start() == end {
			return true
		}
	}
	return false
}

func hasIncomingAt(span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload], at time.Duration) bool {
	for _, elementarySpan := range span.ElementarySpans() {
		if elementarySpan.Start() == at && elementarySpan.Incoming() != nil {
			return true
		}
	}
	return false
}

func markCount(span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]) int {
	var ret int
	for _, elementarySpan := range span.ElementarySpans() {
		ret += len(elementarySpan.Marks())
	}
	return ret
}

func requireDependencyType(
	t *testing.T,
	tr trace.Trace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	want trace.DependencyType,
) {
	t.Helper()
	for _, got := range tr.DependencyTypes() {
		if got == want {
			return
		}
	}
	t.Fatalf("dependency type %d not observed; got %v", want, tr.DependencyTypes())
}

type testCriticalPathElement struct {
	span  trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	start time.Duration
	end   time.Duration
}

func (e testCriticalPathElement) Start() time.Duration {
	return e.start
}

func (e testCriticalPathElement) End() time.Duration {
	return e.end
}

func (e testCriticalPathElement) Span() trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	return e.span
}

func (e testCriticalPathElement) Marks() []trace.Mark[time.Duration] {
	return nil
}
