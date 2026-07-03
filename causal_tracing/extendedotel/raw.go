package extendedotel

import (
	"encoding/json"
	"io"
	"os"
)

// RawResponse is the top-level Jaeger-style response produced by the extended
// OTel trace source.
type RawResponse struct {
	Data   []RawTrace `json:"data"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
	Errors []any      `json:"errors"`
}

// RawTrace is one trace entry in a RawResponse.
type RawTrace struct {
	TraceID   string                `json:"traceID"`
	Spans     []RawSpan             `json:"spans"`
	Processes map[string]RawProcess `json:"processes"`
	Warnings  []string              `json:"warnings"`
}

// RawSpan is a Jaeger span with causal instrumentation in Logs.
type RawSpan struct {
	TraceID       string         `json:"traceID"`
	SpanID        string         `json:"spanID"`
	Flags         int64          `json:"flags"`
	OperationName string         `json:"operationName"`
	References    []RawReference `json:"references"`
	StartTime     int64          `json:"startTime"`
	Duration      int64          `json:"duration"`
	Tags          []KeyValue     `json:"tags"`
	Logs          []RawLog       `json:"logs"`
	ProcessID     string         `json:"processID"`
	Warnings      []string       `json:"warnings"`
}

// RawReference describes a Jaeger span reference.
type RawReference struct {
	RefType string `json:"refType"`
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}

// RawLog is a timestamped span log event.
type RawLog struct {
	Timestamp int64      `json:"timestamp"`
	Fields    []KeyValue `json:"fields"`
}

// RawProcess contains process-level OTel metadata.
type RawProcess struct {
	ServiceName string     `json:"serviceName"`
	Tags        []KeyValue `json:"tags"`
}

// KeyValue is a typed key/value pair from span tags, process tags, or log
// fields.
type KeyValue struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// DecodeRawResponse decodes a RawResponse from JSON.
func DecodeRawResponse(r io.Reader) (*RawResponse, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	var ret RawResponse
	if err := decoder.Decode(&ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

// LoadRawResponseFile decodes a RawResponse from a local JSON file.
func LoadRawResponseFile(path string) (*RawResponse, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return DecodeRawResponse(file)
}
