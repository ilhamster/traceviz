package extendedotel

import (
	"path/filepath"
	"testing"
	"time"

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
