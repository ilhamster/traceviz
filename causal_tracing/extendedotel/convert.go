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
	raw                RawTrace
	namer              *Namer
	trace              trace.Trace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	originMicros       int64
	diagnostics        []Diagnostic
	spansByID          map[string]trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	rawByID            map[string]RawSpan
	categories         map[trace.HierarchyType]map[string]trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	callReturns        []*traceyCallReturnPair
	callerByChild      map[string]string
	spawnParentByChild map[string]string
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

type traceyCallReturnPair struct {
	call   *causalEvent
	ret    *causalEvent
	child  string
	callID string
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
		raw:                raw,
		namer:              &Namer{},
		spansByID:          map[string]trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{},
		rawByID:            map[string]RawSpan{},
		callerByChild:      map[string]string{},
		spawnParentByChild: map[string]string{},
		categories: map[trace.HierarchyType]map[string]trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]{
			ServiceHierarchyType:      {},
			ProcessHierarchyType:      {},
			SpanHierarchyType:         {},
			ServiceSpawnHierarchyType: {},
		},
	}
	state.originMicros = minStartMicros(raw.Spans)
	state.trace = trace.NewTrace[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload](
		trace.DurationComparator,
		state.namer,
	)
	if err := state.indexRawSpans(); err != nil {
		return nil, err
	}
	state.collectDirectSpawnParents()
	state.collectTraceyCallReturns()
	if err := state.createSpans(); err != nil {
		return nil, err
	}
	state.applySuspends()
	state.applyMarks()
	state.applyTraceyCallReturns()
	state.applyLockDependencies()
	state.applyRPCDependencies()
	state.applyDirectDependencies()
	state.trace.Simplify()
	concurrencyMaps := state.buildConcurrencyMaps()
	return &Trace{
		traceID:      raw.TraceID,
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
		if _, isCalledChild := cs.callerByChild[rawSpan.SpanID]; isCalledChild {
			continue
		}
		if _, err := cs.createSpanTree(rawSpan.SpanID, nil, map[string]struct{}{}); err != nil {
			return err
		}
	}
	for _, rawSpan := range cs.raw.Spans {
		if _, ok := cs.spansByID[rawSpan.SpanID]; !ok {
			if _, err := cs.createSpanTree(rawSpan.SpanID, nil, map[string]struct{}{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cs *conversionState) indexRawSpans() error {
	for _, rawSpan := range cs.raw.Spans {
		if rawSpan.SpanID == "" {
			return fmt.Errorf("trace %q contains a span with empty spanID", cs.raw.TraceID)
		}
		if _, ok := cs.rawByID[rawSpan.SpanID]; ok {
			return fmt.Errorf("trace %q contains duplicate spanID %q", cs.raw.TraceID, rawSpan.SpanID)
		}
		cs.rawByID[rawSpan.SpanID] = rawSpan
	}
	return nil
}

func (cs *conversionState) createSpanTree(
	spanID string,
	parent trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	visiting map[string]struct{},
) (trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload], error) {
	if existing := cs.spansByID[spanID]; existing != nil {
		return existing, nil
	}
	if _, ok := visiting[spanID]; ok {
		return nil, fmt.Errorf("trace %q contains a tracey_call cycle involving span %q", cs.raw.TraceID, spanID)
	}
	rawSpan, ok := cs.rawByID[spanID]
	if !ok {
		return nil, fmt.Errorf("trace %q references unknown child span %q", cs.raw.TraceID, spanID)
	}
	visiting[spanID] = struct{}{}
	defer delete(visiting, spanID)

	start := cs.durationFromMicros(rawSpan.StartTime)
	end := cs.durationFromMicros(rawSpan.StartTime + rawSpan.Duration)
	serviceName := cs.serviceName(rawSpan.ProcessID)
	payload := &SpanPayload{
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
	}
	var span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	if parent == nil {
		rootSpan := cs.trace.NewRootSpan(start, end, payload)
		span = rootSpan
		if err := cs.addSpanCategories(rootSpan, rawSpan, serviceName); err != nil {
			return nil, err
		}
	} else {
		childSpan, err := parent.NewChildSpan(trace.DurationComparator, start, end, payload)
		if err != nil {
			cs.diagnostic(spanID, fmt.Sprintf("could not create tracey_call child span: %v", err))
			rootSpan := cs.trace.NewRootSpan(start, end, payload)
			span = rootSpan
			if err := cs.addSpanCategories(rootSpan, rawSpan, serviceName); err != nil {
				return nil, err
			}
		} else {
			span = childSpan
		}
	}
	cs.spansByID[spanID] = span
	for _, pair := range cs.callReturns {
		if pair.call != nil && pair.call.spanID == spanID {
			if _, err := cs.createSpanTree(pair.child, span, visiting); err != nil {
				return nil, err
			}
		}
	}
	return span, nil
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
	if err := spanCategory.AddRootSpan(rootSpan); err != nil {
		return err
	}

	serviceSpawnCategory := cs.serviceSpawnRootSpanCategory(rawSpan, serviceName, map[string]struct{}{})
	return serviceSpawnCategory.AddRootSpan(rootSpan)
}

func (cs *conversionState) serviceSpawnRootSpanCategory(
	rawSpan RawSpan,
	serviceName string,
	visiting map[string]struct{},
) trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	serviceCategory := cs.serviceSpawnServiceCategory(rawSpan, serviceName, visiting)
	servicePayload := serviceCategory.Payload()
	key := servicePayload.ID + ":root-span:" + rawSpan.SpanID
	return cs.childCategory(ServiceSpawnHierarchyType, serviceCategory, key, &CategoryPayload{
		ID:            key,
		Name:          rootSpanCategoryName(rawSpan),
		HierarchyType: ServiceSpawnHierarchyType,
		ServiceName:   serviceName,
		ProcessID:     rawSpan.ProcessID,
		OperationName: rawSpan.OperationName,
		SpanID:        rawSpan.SpanID,
	})
}

func (cs *conversionState) serviceSpawnServiceCategory(
	rawSpan RawSpan,
	serviceName string,
	visiting map[string]struct{},
) trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	if _, ok := visiting[rawSpan.SpanID]; ok {
		return cs.serviceSpawnRootCategory(serviceName)
	}
	visiting[rawSpan.SpanID] = struct{}{}
	defer delete(visiting, rawSpan.SpanID)

	parentSpanID := cs.serviceSpawnParentSpanID(rawSpan)
	parentRawSpan, ok := cs.rawByID[parentSpanID]
	if !ok || parentRawSpan.TraceID != rawSpan.TraceID {
		return cs.serviceSpawnRootCategory(serviceName)
	}
	parentServiceName := cs.serviceName(parentRawSpan.ProcessID)
	parentCategory := cs.serviceSpawnServiceCategory(parentRawSpan, parentServiceName, visiting)
	if parentServiceName == serviceName {
		return parentCategory
	}
	parentPayload := parentCategory.Payload()
	parentPath := parentPayload.ID
	key := parentPath + ":service:" + serviceName
	return cs.childCategory(ServiceSpawnHierarchyType, parentCategory, key, &CategoryPayload{
		ID:            key,
		Name:          serviceName,
		HierarchyType: ServiceSpawnHierarchyType,
		ServiceName:   serviceName,
	})
}

func (cs *conversionState) serviceSpawnParentSpanID(rawSpan RawSpan) string {
	if parentSpanID := cs.spawnParentByChild[rawSpan.SpanID]; parentSpanID != "" {
		return parentSpanID
	}
	return rawSpanParentSpanID(rawSpan)
}

func (cs *conversionState) serviceSpawnRootCategory(
	serviceName string,
) trace.Category[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload] {
	key := "service-spawn:" + serviceName
	return cs.category(ServiceSpawnHierarchyType, key, &CategoryPayload{
		ID:            key,
		Name:          serviceName,
		HierarchyType: ServiceSpawnHierarchyType,
		ServiceName:   serviceName,
	})
}

func rawSpanParentSpanID(rawSpan RawSpan) string {
	for _, ref := range rawSpan.References {
		if ref.RefType == "CHILD_OF" && ref.SpanID != "" {
			return ref.SpanID
		}
	}
	return ""
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

func rootSpanCategoryName(span RawSpan) string {
	if span.OperationName != "" {
		return span.OperationName
	}
	return span.SpanID
}

func (cs *conversionState) collectDirectSpawnParents() {
	originsByDependencyID := map[string]string{}
	type destination struct {
		dependencyID string
		spanID       string
	}
	var destinations []destination
	for _, rawSpan := range cs.raw.Spans {
		for _, event := range causalEvents(rawSpan) {
			if event.eventType != "tracey_dependency_origin" && event.eventType != "tracey_dependency_destination" {
				continue
			}
			if event.fields["dependency_type"] != "spawn" {
				continue
			}
			dependencyID := event.fields["dependency_id"]
			if dependencyID == "" {
				continue
			}
			switch event.eventType {
			case "tracey_dependency_origin":
				if existing := originsByDependencyID[dependencyID]; existing == "" {
					originsByDependencyID[dependencyID] = event.spanID
				}
			case "tracey_dependency_destination":
				destinations = append(destinations, destination{
					dependencyID: dependencyID,
					spanID:       event.spanID,
				})
			}
		}
	}
	for _, destination := range destinations {
		originSpanID := originsByDependencyID[destination.dependencyID]
		if originSpanID == "" || originSpanID == destination.spanID {
			continue
		}
		if _, ok := cs.spawnParentByChild[destination.spanID]; !ok {
			cs.spawnParentByChild[destination.spanID] = originSpanID
		}
	}
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

func (cs *conversionState) applyMarks() {
	for _, rawSpan := range cs.raw.Spans {
		span := cs.spansByID[rawSpan.SpanID]
		for _, event := range causalEvents(rawSpan) {
			if event.eventType != "mark" {
				continue
			}
			label := event.fields["label"]
			if label == "" {
				cs.diagnostic(rawSpan.SpanID, "mark event without label")
				continue
			}
			if err := span.Mark(
				trace.DurationComparator,
				label,
				cs.durationFromMicros(event.timestamp),
				trace.MarkCanFissionSuspend,
			); err != nil {
				cs.diagnostic(rawSpan.SpanID, fmt.Sprintf("could not add mark %q: %v", label, err))
			}
		}
	}
}

func (cs *conversionState) collectTraceyCallReturns() {
	pairs := map[string]*traceyCallReturnPair{}
	for _, rawSpan := range cs.raw.Spans {
		for _, event := range causalEvents(rawSpan) {
			switch event.eventType {
			case "tracey_call", "tracey_return":
				childSpanID := event.fields["child_span_id"]
				if childSpanID == "" {
					cs.diagnostic(rawSpan.SpanID, event.eventType+" event without child_span_id")
					continue
				}
				callID := event.fields["call_id"]
				if callID == "" {
					callID = rawSpan.SpanID + "->" + childSpanID
				}
				key := rawSpan.SpanID + "\x00" + callID
				pair := pairs[key]
				if pair == nil {
					pair = &traceyCallReturnPair{
						child:  childSpanID,
						callID: callID,
					}
					pairs[key] = pair
				}
				if pair.child != childSpanID {
					cs.diagnostic(rawSpan.SpanID, "tracey call_id "+callID+" references multiple child spans")
					continue
				}
				localEvent := event
				switch event.eventType {
				case "tracey_call":
					if pair.call != nil {
						cs.diagnostic(rawSpan.SpanID, "duplicate tracey_call for call_id "+callID)
						continue
					}
					pair.call = &localEvent
				case "tracey_return":
					if pair.ret != nil {
						cs.diagnostic(rawSpan.SpanID, "duplicate tracey_return for call_id "+callID)
						continue
					}
					pair.ret = &localEvent
				}
			}
		}
	}

	keys := make([]string, 0, len(pairs))
	for key := range pairs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pair := pairs[key]
		if pair.call == nil {
			cs.diagnostic(pair.ret.spanID, "tracey call_id "+pair.callID+" has return without call")
			continue
		}
		if pair.ret == nil {
			cs.diagnostic(pair.call.spanID, "tracey call_id "+pair.callID+" has call without return")
			continue
		}
		if _, ok := cs.rawByID[pair.call.spanID]; !ok {
			cs.diagnostic(pair.call.spanID, "tracey call references unknown parent span")
			continue
		}
		childRaw, ok := cs.rawByID[pair.child]
		if !ok {
			cs.diagnostic(pair.call.spanID, "tracey call references unknown child span "+pair.child)
			continue
		}
		parentRaw := cs.rawByID[pair.call.spanID]
		parentServiceName := cs.serviceName(parentRaw.ProcessID)
		childServiceName := cs.serviceName(childRaw.ProcessID)
		if parentServiceName != childServiceName {
			cs.diagnostic(
				pair.call.spanID,
				fmt.Sprintf(
					"tracey call_id %s child span %s is in service %q, not caller service %q",
					pair.callID,
					pair.child,
					childServiceName,
					parentServiceName,
				),
			)
			continue
		}
		if parentRaw.ProcessID != childRaw.ProcessID {
			cs.diagnostic(
				pair.call.spanID,
				fmt.Sprintf(
					"tracey call_id %s child span %s is in process %q, not caller process %q",
					pair.callID,
					pair.child,
					childRaw.ProcessID,
					parentRaw.ProcessID,
				),
			)
			continue
		}
		callTime := cs.durationFromMicros(pair.call.timestamp)
		returnTime := cs.durationFromMicros(pair.ret.timestamp)
		childStart := cs.durationFromMicros(childRaw.StartTime)
		childEnd := cs.durationFromMicros(childRaw.StartTime + childRaw.Duration)
		if trace.DurationComparator.Greater(callTime, returnTime) {
			cs.diagnostic(pair.call.spanID, "tracey call_id "+pair.callID+" returns before it calls")
			continue
		}
		if trace.DurationComparator.Less(childStart, callTime) || trace.DurationComparator.Greater(childEnd, returnTime) {
			cs.diagnostic(pair.call.spanID, "tracey call_id "+pair.callID+" child span is not contained in call interval")
			continue
		}
		if _, ok := cs.callerByChild[pair.child]; ok {
			cs.diagnostic(pair.call.spanID, "tracey child span "+pair.child+" is called by multiple call pairs")
			continue
		}
		cs.callerByChild[pair.child] = pair.call.spanID
		cs.callReturns = append(cs.callReturns, pair)
	}
}

func (cs *conversionState) applyTraceyCallReturns() {
	for _, pair := range cs.callReturns {
		cs.applyTraceyCallReturn(pair)
	}
}

func (cs *conversionState) applyTraceyCallReturn(pair *traceyCallReturnPair) {
	if pair.call == nil {
		cs.diagnostic("", "tracey call_id "+pair.callID+" has return without call")
		return
	}
	if pair.ret == nil {
		cs.diagnostic(pair.call.spanID, "tracey call_id "+pair.callID+" has call without return")
		return
	}
	parentSpan := cs.spansByID[pair.call.spanID]
	childSpan := cs.spansByID[pair.child]
	if parentSpan == nil {
		cs.diagnostic(pair.call.spanID, "tracey call references unknown parent span")
		return
	}
	if childSpan == nil {
		cs.diagnostic(pair.call.spanID, "tracey call references unknown child span "+pair.child)
		return
	}
	callTime := cs.durationFromMicros(pair.call.timestamp)
	returnTime := cs.durationFromMicros(pair.ret.timestamp)
	if trace.DurationComparator.Greater(callTime, returnTime) {
		cs.diagnostic(pair.call.spanID, "tracey call_id "+pair.callID+" returns before it calls")
		return
	}
	if trace.DurationComparator.Less(childSpan.Start(), callTime) || trace.DurationComparator.Greater(childSpan.End(), returnTime) {
		cs.diagnostic(pair.call.spanID, "tracey call_id "+pair.callID+" child span is not contained in call interval")
		return
	}
	childRaw := cs.rawByID[pair.child]
	if !cs.updateTraceyCallDependencyPayload(parentSpan, childSpan, pair, childRaw) {
		cs.diagnostic(pair.call.spanID, "tracey call_id "+pair.callID+" could not find implicit call dependency")
	}
	if !cs.updateTraceyReturnDependencyPayload(parentSpan, childSpan, pair, childRaw) {
		cs.diagnostic(pair.call.spanID, "tracey call_id "+pair.callID+" could not find implicit return dependency")
	}
}

func (cs *conversionState) updateTraceyCallDependencyPayload(
	parentSpan trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	childSpan trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	pair *traceyCallReturnPair,
	childRaw RawSpan,
) bool {
	for _, elementarySpan := range childSpan.ElementarySpans() {
		dependency := elementarySpan.Incoming()
		if dependency == nil ||
			dependency.DependencyType() != trace.Call ||
			dependency.TriggeringOrigin() == nil ||
			dependency.TriggeringOrigin().Span() != parentSpan {
			continue
		}
		return updateDependencyPayload(dependency, &DependencyPayload{
			Kind:             "call",
			Key:              pair.callID,
			OriginSpanID:     pair.call.spanID,
			OriginTimestamp:  pair.call.timestamp,
			DestSpanID:       pair.child,
			DestTimestamp:    childRaw.StartTime,
			OriginEvent:      pair.call.eventType,
			DestinationEvent: "tracey_call_child_start",
		})
	}
	return false
}

func (cs *conversionState) updateTraceyReturnDependencyPayload(
	parentSpan trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	childSpan trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	pair *traceyCallReturnPair,
	childRaw RawSpan,
) bool {
	for _, elementarySpan := range childSpan.ElementarySpans() {
		dependency := elementarySpan.Outgoing()
		if dependency == nil ||
			dependency.DependencyType() != trace.Return ||
			!dependencyHasDestinationSpan(dependency, parentSpan) {
			continue
		}
		return updateDependencyPayload(dependency, &DependencyPayload{
			Kind:             "return",
			Key:              pair.callID,
			OriginSpanID:     pair.child,
			OriginTimestamp:  childRaw.StartTime + childRaw.Duration,
			DestSpanID:       pair.ret.spanID,
			DestTimestamp:    pair.ret.timestamp,
			OriginEvent:      "tracey_call_child_end",
			DestinationEvent: pair.ret.eventType,
		})
	}
	return false
}

func dependencyHasDestinationSpan(
	dependency trace.Dependency[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	span trace.Span[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
) bool {
	for _, destination := range dependency.Destinations() {
		if destination.Span() == span {
			return true
		}
	}
	return false
}

func updateDependencyPayload(
	dependency trace.Dependency[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload],
	payload *DependencyPayload,
) bool {
	payloadUpdater, ok := dependency.(interface {
		WithPayload(*DependencyPayload) trace.MutableDependency[time.Duration, *CategoryPayload, *SpanPayload, *DependencyPayload]
	})
	if !ok {
		return false
	}
	payloadUpdater.WithPayload(payload)
	return true
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

func (cs *conversionState) applyDirectDependencies() {
	eventsByDependencyID := map[string][]causalEvent{}
	for _, rawSpan := range cs.raw.Spans {
		for _, event := range causalEvents(rawSpan) {
			switch event.eventType {
			case "tracey_dependency_origin", "tracey_dependency_destination":
				dependencyID := event.fields["dependency_id"]
				if dependencyID == "" {
					cs.diagnostic(rawSpan.SpanID, event.eventType+" event without dependency_id")
					continue
				}
				eventsByDependencyID[dependencyID] = append(eventsByDependencyID[dependencyID], event)
			}
		}
	}
	for dependencyID, events := range eventsByDependencyID {
		sortCausalEvents(events)
		var origin *causalEvent
		var destinations []causalEvent
		dependencyTypeName := ""
		for idx := range events {
			event := events[idx]
			if event.fields["dependency_type"] != "" {
				dependencyTypeName = event.fields["dependency_type"]
			}
			switch event.eventType {
			case "tracey_dependency_origin":
				if origin != nil {
					cs.diagnostic(event.spanID, "direct dependency "+dependencyID+" has multiple origins")
					continue
				}
				origin = &event
			case "tracey_dependency_destination":
				destinations = append(destinations, event)
			}
		}
		if origin == nil {
			cs.diagnostic("", "direct dependency "+dependencyID+" has no origin")
			continue
		}
		if len(destinations) == 0 {
			cs.diagnostic(origin.spanID, "direct dependency "+dependencyID+" has no destinations")
			continue
		}
		dependencyType, ok := directDependencyType(dependencyTypeName)
		if !ok {
			cs.diagnostic(origin.spanID, "direct dependency "+dependencyID+" has unknown dependency_type "+dependencyTypeName)
			continue
		}
		dependency := cs.trace.NewDependency(dependencyType, &DependencyPayload{
			Kind:            dependencyTypeName,
			Key:             dependencyID,
			OriginSpanID:    origin.spanID,
			OriginTimestamp: origin.timestamp,
			OriginEvent:     origin.eventType,
		})
		if err := dependency.SetOriginSpan(
			trace.DurationComparator,
			cs.spansByID[origin.spanID],
			cs.durationFromMicros(origin.timestamp),
			trace.DependencyEndpointCanFissionSuspend,
		); err != nil {
			cs.diagnostic(origin.spanID, fmt.Sprintf("could not set direct dependency origin %q: %v", dependencyID, err))
			continue
		}
		for _, destination := range destinations {
			if err := dependency.AddDestinationSpan(
				trace.DurationComparator,
				cs.spansByID[destination.spanID],
				cs.durationFromMicros(destination.timestamp),
				trace.DependencyEndpointCanFissionSuspend,
			); err != nil {
				cs.diagnostic(destination.spanID, fmt.Sprintf("could not add direct dependency destination %q: %v", dependencyID, err))
			}
		}
	}
}

func directDependencyType(name string) (trace.DependencyType, bool) {
	switch name {
	case "spawn":
		return DependencySpawn, true
	case "send":
		return DependencySend, true
	case "signal":
		return DependencySignal, true
	default:
		return 0, false
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
