// Package service wires causal tracing data sources into TraceViz HTTP
// handlers.
package service

import (
	"net/http"

	datasource "github.com/ilhamster/traceviz/causal_tracing/data_source"
	"github.com/ilhamster/traceviz/server/go/handlers"
	querydispatcher "github.com/ilhamster/traceviz/server/go/query_dispatcher"
)

// Service owns the TraceViz query handlers for the causal tracing tool.
type Service struct {
	queryHandler handlers.QueryHandler
}

// New creates a causal tracing TraceViz service.
func New(traceRoot, defaultTracePath string) (*Service, error) {
	fetcher := datasource.NewFileTraceFetcher(traceRoot)
	dataSource, err := datasource.New(defaultTracePath, fetcher)
	if err != nil {
		return nil, err
	}
	dispatcher, err := querydispatcher.New(dataSource)
	if err != nil {
		return nil, err
	}
	return &Service{
		queryHandler: handlers.NewQueryHandler(dispatcher),
	}, nil
}

// RegisterHandlers registers the service's HTTP handlers.
func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	for path, handler := range s.queryHandler.HandlersByPath() {
		mux.HandleFunc(path, handler)
	}
}
