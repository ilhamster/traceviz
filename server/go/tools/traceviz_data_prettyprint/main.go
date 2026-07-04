package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ilhamster/traceviz/server/go/util"
)

func prettyPrintDataResponse(input io.Reader) (string, error) {
	var data util.Data
	decoder := json.NewDecoder(input)
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return "", fmt.Errorf("decode TraceViz data response: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return "", fmt.Errorf("decode trailing input: %w", err)
		}
		return "", fmt.Errorf("decode TraceViz data response: trailing JSON value")
	}
	pretty, err := prettyPrintWellFormedData(data)
	if err != nil {
		return "", err
	}
	return pretty, nil
}

func prettyPrintWellFormedData(data util.Data) (pretty string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			// The util.Data pretty-printer assumes a well-formed TraceViz
			// response; convert malformed-response panics into CLI errors.
			err = fmt.Errorf("pretty-print TraceViz data response: malformed response: %v", recovered)
		}
	}()
	return data.PrettyPrint(), nil
}

func inputReader(path string) (io.ReadCloser, error) {
	if path == "" || path == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	return file, nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [response.json|-]\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Pretty-prints a compact TraceViz /GetData JSON response.")
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	path := "-"
	if flag.NArg() == 1 {
		path = flag.Arg(0)
	}
	input, err := inputReader(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer input.Close()
	pretty, err := prettyPrintDataResponse(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(pretty)
}
