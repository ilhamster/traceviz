package extendedotel

import (
	"fmt"
	"sort"
	"strings"
	"time"

	smartdependencies "github.com/ilhamster/tracey/smart_dependencies"
	"github.com/ilhamster/tracey/trace"
)

type conversionState struct {
	raw          RawTrace
	namer        *Namer
	trace        trace.Trace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	originMicros int64
	diagnostics  []Diagnostic
	spansByID    map[string]trace.RootSpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	rawByID      map[string]RawSpan
	categories   map[trace.HierarchyType]map[string]trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
}

type causalEvent struct {
	spanID    string
	timestamp int64
	eventType string
	fields    map[string]string
}

type dependencyID struct {
	kind string
	key  string
}

// ConvertExtendedOtelResponse converts every trace in a raw response.
func ConvertExtendedOtelResponse(raw *RawResponse) ([]*Trace, error) {
	if raw == nil {
		return nil, fmt.Errorf("nil raw response")
	}
	ret := make([]*Trace, 0, len(raw.Data))
	for idx, rawTrace := range raw.Data {
		converted, err := ConvertExtendedOtelTrace(rawTrace)
		if err != nil {
			return nil, fmt.Errorf("convert trace %d (%q): %w", idx, rawTrace.TraceID, err)
		}
		ret = append(ret, converted)
	}
	return ret, nil
}

// ConvertExtendedOtelTrace converts one raw extended OTel trace into a Tracey
// trace wrapper.
func ConvertExtendedOtelTrace(raw RawTrace) (*Trace, error) {
	if len(raw.Spans) == 0 {
		return nil, fmt.Errorf("trace %q has no spans", raw.TraceID)
	}
	state := &conversionState{
		raw:       raw,
		namer:     &Namer{},
		spansByID: map[string]trace.RootSpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{},
		rawByID:   map[string]RawSpan{},
		categories: map[trace.HierarchyType]map[string]trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{
			ServiceHierarchyType: {},
			ProcessHierarchyType: {},
			SpanHierarchyType:    {},
		},
	}
	state.originMicros = minStartMicros(raw.Spans)
	state.trace = trace.NewTrace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
		trace.DurationComparator,
		state.namer,
	)
	if err := state.createSpans(); err != nil {
		return nil, err
	}
	state.applySuspends()
	state.applyLockDependencies()
	state.applyRPCDependencies()
	state.trace.Simplify()
	concurrencyMaps := state.buildConcurrencyMaps()
	return &Trace{
		raw:          raw,
		trace:        state.trace,
		namer:        state.namer,
		originMicros: state.originMicros,
		diagnostics:  append([]Diagnostic(nil), state.diagnostics...),
		spansByID:    state.spansByID,
		concurrency:  concurrencyMaps,
	}, nil
}

func minStartMicros(spans []RawSpan) int64 {
	minStart := spans[0].StartTime
	for _, span := range spans[1:] {
		if span.StartTime < minStart {
			minStart = span.StartTime
		}
	}
	return minStart
}

func (cs *conversionState) createSpans() error {
	for _, rawSpan := range cs.raw.Spans {
		if rawSpan.SpanID == "" {
			return fmt.Errorf("trace %q contains a span with empty spanID", cs.raw.TraceID)
		}
		if _, ok := cs.spansByID[rawSpan.SpanID]; ok {
			return fmt.Errorf("trace %q contains duplicate spanID %q", cs.raw.TraceID, rawSpan.SpanID)
		}
		start := cs.durationFromMicros(rawSpan.StartTime)
		end := cs.durationFromMicros(rawSpan.StartTime + rawSpan.Duration)
		serviceName := cs.serviceName(rawSpan.ProcessID)
		rootSpan := cs.trace.NewRootSpan(start, end, &SpanPayload{
			TraceID:       rawSpan.TraceID,
			SpanID:        rawSpan.SpanID,
			OperationName: rawSpan.OperationName,
			ProcessID:     rawSpan.ProcessID,
			ServiceName:   serviceName,
			StartTime:     rawSpan.StartTime,
			Duration:      rawSpan.Duration,
			Tags:          rawSpan.Tags,
			Logs:          rawSpan.Logs,
			References:    rawSpan.References,
			Warnings:      rawSpan.Warnings,
		})
		cs.spansByID[rawSpan.SpanID] = rootSpan
		cs.rawByID[rawSpan.SpanID] = rawSpan
		if err := cs.addSpanCategories(rootSpan, rawSpan, serviceName); err != nil {
			return err
		}
	}
	return nil
}

func (cs *conversionState) serviceName(processID string) string {
	process, ok := cs.raw.Processes[processID]
	if !ok || process.ServiceName == "" {
		return "unknown-service"
	}
	return process.ServiceName
}

func (cs *conversionState) addSpanCategories(
	rootSpan trace.RootSpan[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	rawSpan RawSpan,
	serviceName string,
) error {
	serviceCategory := cs.category(ServiceHierarchyType, serviceName, &CategoryPayload{
		ID:            "service:" + serviceName,
		Name:          serviceName,
		HierarchyType: ServiceHierarchyType,
		ServiceName:   serviceName,
	})
	operationCategory := cs.childCategory(ServiceHierarchyType, serviceCategory, serviceName+"/"+rawSpan.OperationName, &CategoryPayload{
		ID:            "service:" + serviceName + ":operation:" + rawSpan.OperationName,
		Name:          rawSpan.OperationName,
		HierarchyType: ServiceHierarchyType,
		ServiceName:   serviceName,
		OperationName: rawSpan.OperationName,
	})
	if err := operationCategory.AddRootSpan(rootSpan); err != nil {
		return err
	}

	processCategory := cs.category(ProcessHierarchyType, serviceName, &CategoryPayload{
		ID:            "process-service:" + serviceName,
		Name:          serviceName,
		HierarchyType: ProcessHierarchyType,
		ServiceName:   serviceName,
	})
	processName := rawSpan.ProcessID
	if processName == "" {
		processName = "unknown-process"
	}
	processLeaf := cs.childCategory(ProcessHierarchyType, processCategory, serviceName+"/"+processName, &CategoryPayload{
		ID:            "process-service:" + serviceName + ":process:" + processName,
		Name:          processName,
		HierarchyType: ProcessHierarchyType,
		ServiceName:   serviceName,
		ProcessID:     processName,
	})
	if err := processLeaf.AddRootSpan(rootSpan); err != nil {
		return err
	}

	spanCategory := cs.category(SpanHierarchyType, rawSpan.SpanID, &CategoryPayload{
		ID:            "span:" + rawSpan.SpanID,
		Name:          spanDisplayName(rawSpan),
		HierarchyType: SpanHierarchyType,
		ServiceName:   serviceName,
		ProcessID:     rawSpan.ProcessID,
		OperationName: rawSpan.OperationName,
		SpanID:        rawSpan.SpanID,
	})
	return spanCategory.AddRootSpan(rootSpan)
}

func (cs *conversionState) category(
	hierarchyType trace.HierarchyType,
	key string,
	payload *CategoryPayload,
) trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	if existing := cs.categories[hierarchyType][key]; existing != nil {
		return existing
	}
	category := cs.trace.NewRootCategory(hierarchyType, payload)
	cs.categories[hierarchyType][key] = category
	return category
}

func (cs *conversionState) childCategory(
	hierarchyType trace.HierarchyType,
	parent trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	key string,
	payload *CategoryPayload,
) trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	if existing := cs.categories[hierarchyType][key]; existing != nil {
		return existing
	}
	category := parent.NewChildCategory(payload)
	cs.categories[hierarchyType][key] = category
	return category
}

func spanDisplayName(span RawSpan) string {
	if span.OperationName == "" {
		return span.SpanID
	}
	return span.OperationName + " (" + span.SpanID + ")"
}

func (cs *conversionState) applySuspends() {
	for _, rawSpan := range cs.raw.Spans {
		span := cs.spansByID[rawSpan.SpanID]
		var suspendStart *RawLog
		for _, log := range sortedLogs(rawSpan.Logs) {
			eventType := logField(log, "type")
			switch eventType {
			case "suspend_start":
				localLog := log
				if suspendStart != nil {
					cs.diagnostic(rawSpan.SpanID, "nested or repeated suspend_start; replacing prior unmatched start")
				}
				suspendStart = &localLog
			case "suspend_stop":
				if suspendStart == nil {
					cs.diagnostic(rawSpan.SpanID, "suspend_stop without matching suspend_start")
					continue
				}
				start := cs.durationFromMicros(suspendStart.Timestamp)
				end := cs.durationFromMicros(log.Timestamp)
				if err := span.Suspend(trace.DurationComparator, start, end, trace.SuspendFissionsAroundElementarySpanEndpoints); err != nil {
					cs.diagnostic(rawSpan.SpanID, fmt.Sprintf("could not add suspend [%s, %s]: %v", start, end, err))
				}
				suspendStart = nil
			}
		}
		if suspendStart != nil {
			cs.diagnostic(rawSpan.SpanID, "suspend_start without matching suspend_stop")
		}
	}
}

func (cs *conversionState) applyLockDependencies() {
	smartDeps := smartdependencies.New[dependencyID](
		cs.trace,
	)
	for _, rawSpan := range cs.raw.Spans {
		for _, event := range causalEvents(rawSpan) {
			switch event.eventType {
			case "lock_acquired":
				lockID := event.fields["lock_id"]
				if lockID == "" {
					cs.diagnostic(rawSpan.SpanID, "lock_acquired event without lock_id")
					continue
				}
				dependency, _, err := smartDeps.GetIndexed(
					DependencyLock,
					&DependencyPayload{
						Kind: "lock",
						Key:  lockID,
					},
					dependencyID{kind: "lock", key: lockID},
					smartdependencies.AllowReuse,
				)
				if err != nil {
					cs.diagnostic(rawSpan.SpanID, fmt.Sprintf("could not get lock dependency %q: %v", lockID, err))
					continue
				}
				dependency.WithDestination(
					cs.spansByID[event.spanID],
					cs.durationFromMicros(event.timestamp),
					trace.DependencyEndpointCanFissionSuspend,
				)
			case "lock_released":
				lockID := event.fields["lock_id"]
				if lockID == "" {
					cs.diagnostic(rawSpan.SpanID, "lock_released event without lock_id")
					continue
				}
				dependency, _, err := smartDeps.GetIndexed(
					DependencyLock,
					&DependencyPayload{
						Kind: "lock",
						Key:  lockID,
					},
					dependencyID{kind: "lock", key: lockID},
					smartdependencies.AllowReuse,
				)
				if err != nil {
					cs.diagnostic(rawSpan.SpanID, fmt.Sprintf("could not get lock dependency %q: %v", lockID, err))
					continue
				}
				dependency.WithOrigin(
					cs.spansByID[event.spanID],
					cs.durationFromMicros(event.timestamp),
					trace.DependencyEndpointCanFissionSuspend,
				)
			}
		}
	}
	metrics, err := smartDeps.CloseWithMetrics()
	if err != nil {
		cs.diagnostic("", fmt.Sprintf("could not close lock dependencies: %v", err))
		return
	}
	cs.addSmartDependencyDiagnostics(metrics)
}

func (cs *conversionState) applyRPCDependencies() {
	eventsByConnectionID := map[string][]causalEvent{}
	for _, rawSpan := range cs.raw.Spans {
		for _, event := range causalEvents(rawSpan) {
			connectionID := event.fields["connection_id"]
			if connectionID == "" {
				continue
			}
			switch event.eventType {
			case "call", "start", "finish":
				eventsByConnectionID[connectionID] = append(eventsByConnectionID[connectionID], event)
			}
		}
	}
	for connectionID, events := range eventsByConnectionID {
		sortCausalEvents(events)
		for _, call := range events {
			if call.eventType != "call" {
				continue
			}
			serverStart, ok := cs.firstLaterEvent(events, call, "start", true)
			if !ok {
				cs.diagnostic(call.spanID, "call event without matching server start for connection_id "+connectionID)
				continue
			}
			cs.addDependency(
				DependencyRPC,
				&DependencyPayload{
					Kind:             "rpc_call",
					Key:              connectionID,
					OriginSpanID:     call.spanID,
					OriginTimestamp:  call.timestamp,
					DestSpanID:       serverStart.spanID,
					DestTimestamp:    serverStart.timestamp,
					OriginEvent:      call.eventType,
					DestinationEvent: serverStart.eventType,
				},
				call,
				serverStart,
			)
			serverFinish, ok := cs.firstLaterEvent(events, serverStart, "finish", false)
			if !ok || serverFinish.spanID != serverStart.spanID {
				continue
			}
			callerFinish, ok := cs.firstLaterEvent(events, serverFinish, "finish", true)
			if !ok || callerFinish.spanID != call.spanID {
				continue
			}
			cs.addDependency(
				DependencyRPCReturn,
				&DependencyPayload{
					Kind:             "return",
					Key:              connectionID,
					OriginSpanID:     serverFinish.spanID,
					OriginTimestamp:  serverFinish.timestamp,
					DestSpanID:       callerFinish.spanID,
					DestTimestamp:    callerFinish.timestamp,
					OriginEvent:      serverFinish.eventType,
					DestinationEvent: callerFinish.eventType,
				},
				serverFinish,
				callerFinish,
			)
		}
	}
}

func (cs *conversionState) addSmartDependencyDiagnostics(
	metrics *smartdependencies.Metrics[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) {
	for dependencyType, count := range metrics.UnpairedOriginsByType {
		if count > 0 {
			cs.diagnostic("", fmt.Sprintf("smart dependencies left %d unpaired origins for dependency type %d", count, dependencyType))
		}
	}
	for dependencyType, count := range metrics.UnpairedDestinationsByType {
		if count > 0 {
			cs.diagnostic("", fmt.Sprintf("smart dependencies left %d unpaired destinations for dependency type %d", count, dependencyType))
		}
	}
	for dependencyType, count := range metrics.DroppedDependenciesByType {
		if count > 0 {
			cs.diagnostic("", fmt.Sprintf("smart dependencies dropped %d dependencies for dependency type %d", count, dependencyType))
		}
	}
}

func (cs *conversionState) firstLaterEvent(
	events []causalEvent,
	after causalEvent,
	eventType string,
	differentSpan bool,
) (causalEvent, bool) {
	for _, event := range events {
		if event.timestamp < after.timestamp {
			continue
		}
		if event.eventType != eventType {
			continue
		}
		if differentSpan && event.spanID == after.spanID {
			continue
		}
		return event, true
	}
	return causalEvent{}, false
}

func (cs *conversionState) addDependency(
	dependencyType trace.DependencyType,
	payload *DependencyPayload,
	origin causalEvent,
	destination causalEvent,
) {
	originSpan := cs.spansByID[origin.spanID]
	destinationSpan := cs.spansByID[destination.spanID]
	if originSpan == nil || destinationSpan == nil {
		cs.diagnostic(origin.spanID, "dependency references an unknown span")
		return
	}
	dependency := cs.trace.NewDependency(dependencyType, payload)
	if err := dependency.SetOriginSpan(
		trace.DurationComparator,
		originSpan,
		cs.durationFromMicros(origin.timestamp),
		trace.DependencyEndpointCanFissionSuspend,
	); err != nil {
		cs.diagnostic(origin.spanID, fmt.Sprintf("could not set dependency origin %s: %v", payload, err))
		return
	}
	if err := dependency.AddDestinationSpan(
		trace.DurationComparator,
		destinationSpan,
		cs.durationFromMicros(destination.timestamp),
		trace.DependencyEndpointCanFissionSuspend,
	); err != nil {
		cs.diagnostic(destination.spanID, fmt.Sprintf("could not add dependency destination %s: %v", payload, err))
	}
}

func (cs *conversionState) durationFromMicros(timestampMicros int64) time.Duration {
	return time.Duration(timestampMicros-cs.originMicros) * time.Microsecond
}

func (cs *conversionState) diagnostic(spanID, message string) {
	cs.diagnostics = append(cs.diagnostics, Diagnostic{
		TraceID: cs.raw.TraceID,
		SpanID:  spanID,
		Message: message,
	})
}

func causalEvents(span RawSpan) []causalEvent {
	var events []causalEvent
	for _, log := range span.Logs {
		eventType := logField(log, "type")
		if eventType == "" {
			continue
		}
		events = append(events, causalEvent{
			spanID:    span.SpanID,
			timestamp: log.Timestamp,
			eventType: eventType,
			fields:    logFields(log),
		})
	}
	return events
}

func logField(log RawLog, key string) string {
	for _, field := range log.Fields {
		if field.Key == key {
			return valueString(field.Value)
		}
	}
	return ""
}

func logFields(log RawLog) map[string]string {
	fields := map[string]string{}
	for _, field := range log.Fields {
		fields[field.Key] = valueString(field.Value)
	}
	return fields
}

func sortedLogs(logs []RawLog) []RawLog {
	ret := append([]RawLog(nil), logs...)
	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].Timestamp < ret[j].Timestamp
	})
	return ret
}

func sortCausalEvents(events []causalEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].timestamp != events[j].timestamp {
			return events[i].timestamp < events[j].timestamp
		}
		if events[i].spanID != events[j].spanID {
			return events[i].spanID < events[j].spanID
		}
		return strings.Compare(events[i].eventType, events[j].eventType) < 0
	})
}
