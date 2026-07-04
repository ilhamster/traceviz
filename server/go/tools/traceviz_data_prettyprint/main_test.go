package main

import (
	"strings"
	"testing"
)

func TestPrettyPrintDataResponse(t *testing.T) {
	input := `{
		"StringTable": [
			"greeting",
			"Hello!",
			"count",
			"items",
			"apple",
			"banana"
		],
		"DataSeries": [
			{
				"SeriesName": "trace",
				"Root": [
					[],
					[
						[
							[
								[0, [2, 1]],
								[2, [5, 100]]
							],
							[
								[
									[
										[3, [4, [4, 5]]]
									],
									[]
								]
							]
						]
					]
				]
			}
		]
	}`
	want := `Data:
  Series trace
    Root:
      Child:
        Prop 'count': 100
        Prop 'greeting': 'Hello!'
        Child:
          Prop 'items': [ 'apple', 'banana' ]`
	got, err := prettyPrintDataResponse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("prettyPrintDataResponse() failed: %v", err)
	}
	if got != want {
		t.Fatalf("prettyPrintDataResponse() =\n%s\nwant:\n%s", got, want)
	}
}

func TestPrettyPrintDataResponseRejectsTrailingJSON(t *testing.T) {
	_, err := prettyPrintDataResponse(strings.NewReader(`{"StringTable":[],"DataSeries":[]} {}`))
	if err == nil {
		t.Fatal("prettyPrintDataResponse() succeeded, want trailing JSON error")
	}
	if !strings.Contains(err.Error(), "trailing JSON value") {
		t.Fatalf("prettyPrintDataResponse() error = %q, want trailing JSON error", err)
	}
}

func TestPrettyPrintDataResponseRejectsMalformedTraceVizData(t *testing.T) {
	_, err := prettyPrintDataResponse(strings.NewReader(`{
		"StringTable": [],
		"DataSeries": [{"SeriesName": "broken"}]
	}`))
	if err == nil {
		t.Fatal("prettyPrintDataResponse() succeeded, want malformed response error")
	}
	if !strings.Contains(err.Error(), "malformed response") {
		t.Fatalf("prettyPrintDataResponse() error = %q, want malformed response error", err)
	}
}
