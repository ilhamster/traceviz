import { useCallback, useEffect, useMemo, useState, type RefCallback } from 'react';
import {
	Action,
	AppCore,
	Changed,
	Clear,
	ConfigurationError,
	DataQuery,
	DataSeriesQuery,
	Duration,
	DurationValue,
	Equals,
	HttpDataFetcher,
	IntegerValue,
	Interactions,
	LocalValue,
	Predicate,
	Reaction,
	Set as SetAction,
	Severity,
	StringListValue,
	StringSetValue,
	StringValue,
	Trace,
	Update,
	ValueMap,
	Watch,
	type Axis,
	type KeyedValueRef,
	type ResponseNode,
	type Value,
	type ValueRef,
} from '@traceviz/client-core';
import {
  AppCoreContext,
  CALLED_OUT_CATEGORY_ID_KEY,
  CATEGORY_CLICK_ACTION,
  CATEGORY_HEADERS_TARGET,
  CATEGORY_ID_KEY,
  CATEGORY_MOUSEOUT_ACTION,
  CATEGORY_MOUSEOVER_ACTION,
  DEPRESSED_KEY_CODES_KEY,
  DataTable,
  ErrorToast,
  HorizontalTrace,
  KEY_PRESS_ACTION,
  KEY_TARGET,
  KeypressListener,
  RectangularCategoryHierarchyYAxis,
  StandardContinuousXAxis,
  TRACE_BRUSH_ACTION,
  TRACE_CHART_TARGET,
  TRACE_RESET_ZOOM_ACTION,
  TRACE_SPAN_CLICK_ACTION,
  TRACE_SPANS_TARGET,
  TRACE_ZOOM_END_KEY,
  TRACE_ZOOM_START_KEY,
  UPDATE_CALLED_OUT_CATEGORY_WATCH,
  useValue,
} from '@traceviz/client-react';
import type { Subscription } from 'rxjs';

const DEFAULT_TRACE_PATH = 'testdata/compose-post-ct-logs.json';
const LOAD_STATUS_QUERY = 'causal_tracing.load_status';
const CORPUS_TRACES_QUERY = 'causal_tracing.corpus_traces';
const TRACE_STATUS_QUERY = 'causal_tracing.trace_status';
const TRACE_DIAGNOSTICS_QUERY = 'causal_tracing.trace_diagnostics';
const TRACE_QUERY = 'causal_tracing.trace';
const CRITICAL_PATH_TRACE_QUERY = 'causal_tracing.critical_path_trace';
const SPAN_CAUSALITY_QUERY = 'causal_tracing.span_causality';
const VALIDATE_SEARCH_QUERY = 'causal_tracing.validate_search';
const VALIDATE_TRANSFORM_QUERY = 'causal_tracing.validate_transform';
const DEFAULT_CRITICAL_PATH_START = '** earliest';
const DEFAULT_CRITICAL_PATH_END = '** latest';
const DEFAULT_CRITICAL_PATH_STRATEGY = 'temporal_most_work';
const EXPANDED_CATEGORY_IDS_KEY = 'expanded_category_ids';
const FOCUS_SPAN_IDS_KEY = 'focus_span_ids';
const SPAN_ID_KEY = 'span_id';
const OTHER_SPAN_ID_KEY = 'other_span_id';
const TRACE_VIEW_WIDTH_PX_PARAM = 'trace_view_width_px';
const TRACE_ID_KEY = 'trace_id';
const CATEGORY_ID_PROPERTY_KEY = 'category_id';
const CATEGORY_EXPANSION_STATE_KEY = 'category_expansion_state';
const CATEGORY_STATE_COLLAPSED = 'collapsed';
const CATEGORY_STATE_EXPANDED = 'expanded';
const TEMPORAL_DOMAIN_START_KEY = 'temporal_domain_start';
const TEMPORAL_DOMAIN_END_KEY = 'temporal_domain_end';
const CRITICAL_PATH_START_KEY = 'critical_path_start';
const CRITICAL_PATH_END_KEY = 'critical_path_end';
const CRITICAL_PATH_STRATEGY_KEY = 'critical_path_strategy';
const SEARCH_KEY = 'search';
const DRAFT_SEARCH_KEY = 'draft_search';
const TRANSFORM_TEMPLATE_KEY = 'transform_template';
const DRAFT_TRANSFORM_TEMPLATE_KEY = 'draft_transform_template';
const EXPAND_MATCHES_KEY = 'expand_matches';
const SEARCH_VALIDATION_REQUEST_KEY = 'search_validation_request';
const TRANSFORM_VALIDATION_REQUEST_KEY = 'transform_validation_request';
const STATUS_KEY = 'status';
const MESSAGE_KEY = 'message';
const TRACE_FULL_START_KEY = 'trace_full_start';
const TRACE_FULL_END_KEY = 'trace_full_end';
const KEY_ZOOM_IN = 'KeyW';
const KEY_ZOOM_OUT = 'KeyS';
const KEY_PAN_LEFT = 'KeyA';
const KEY_PAN_RIGHT = 'KeyD';
const ZOOM_IN_FACTOR = 0.8;
const ZOOM_OUT_FACTOR = 1.25;
const PAN_FRACTION = 0.2;
const MIN_TEMPORAL_DOMAIN_NS = 100;

type DurationDomain = [number, number];

type CausalTracingState = {
  core: AppCore;
  dataQuery: DataQuery;
  corpusPath: StringValue;
  traceID: StringValue;
  expandedCategoryIDs: StringSetValue;
  focusSpanIDs: StringListValue;
  temporalDomainStart: DurationValue;
  temporalDomainEnd: DurationValue;
  criticalPathStart: StringValue;
  criticalPathEnd: StringValue;
  criticalPathStrategy: StringValue;
  search: StringValue;
  draftSearch: StringValue;
  transformTemplate: StringValue;
  draftTransformTemplate: StringValue;
  expandMatches: StringValue;
  searchValidationRequest: IntegerValue;
  transformValidationRequest: IntegerValue;
  traceViewWidthPx: IntegerValue;
  calledOutCategoryID: StringValue;
};

type CriticalPathStrategyOption = {
  value: string;
  label: string;
};

const CRITICAL_PATH_STRATEGY_OPTIONS: CriticalPathStrategyOption[] = [
  { value: 'temporal_most_work', label: 'Temporal max work' },
  { value: 'most_work', label: 'Max work' },
  { value: 'causal', label: 'Causal' },
  { value: 'predecessor', label: 'Predecessor' },
  { value: 'most_prox', label: 'Latest predecessor' },
  { value: 'least_prox', label: 'Earliest predecessor' },
  { value: 'least_work', label: 'Max dependency delay' },
];

type SeriesResult = {
  data?: ResponseNode;
  loading: boolean;
};

class GlobalValueRef implements KeyedValueRef {
  constructor(private readonly core: AppCore, readonly key: string) {}

  get(): Value | undefined {
    return this.core.globalState.get(this.key);
  }

  label(): string {
    return `global value '${this.key}'`;
  }
}

class DirectValueRef implements ValueRef {
  constructor(private readonly value: Value, private readonly name: string) {}

  get(): Value {
    return this.value;
  }

  label(): string {
    return this.name;
  }
}

class PushNonEmptyLocalStringLeft extends Update {
  constructor(
    private readonly destination: StringListValue,
    private readonly sourceKey: string,
  ) {
    super();
  }

  override update(localState?: ValueMap): void {
    const source = localState?.get(this.sourceKey);
    if (!(source instanceof StringValue) || source.val === '') {
      return;
    }
    this.destination.val = [source.val, ...this.destination.val];
  }

  override get autoDocument(): string {
    return `pushes nonempty local value '${this.sourceKey}'`;
  }
}

class ToggleCategoryExpansion extends Update {
  constructor(private readonly expandedCategoryIDs: StringSetValue) {
    super();
  }

  override update(localState?: ValueMap): void {
    const categoryID = localState?.get(CATEGORY_ID_PROPERTY_KEY);
    const expansionState = localState?.get(CATEGORY_EXPANSION_STATE_KEY);
    if (!(categoryID instanceof StringValue) || !(expansionState instanceof StringValue)) {
      return;
    }
    const nextExpanded = new Set(this.expandedCategoryIDs.val);
    switch (expansionState.val) {
      case CATEGORY_STATE_COLLAPSED:
        nextExpanded.add(categoryID.val);
        break;
      case CATEGORY_STATE_EXPANDED:
        nextExpanded.delete(categoryID.val);
        break;
      default:
        return;
    }
    this.expandedCategoryIDs.val = nextExpanded;
  }

  override get autoDocument(): string {
    return 'toggles category expansion';
  }
}

function setTemporalDomain(
  temporalDomainStart: DurationValue,
  temporalDomainEnd: DurationValue,
  startNanos: number,
  endNanos: number,
  fullDomain?: DurationDomain | null,
): void {
  if (!Number.isFinite(startNanos) || !Number.isFinite(endNanos)) {
    return;
  }
  let nextStart = Math.round(Math.min(startNanos, endNanos));
  let nextEnd = Math.round(Math.max(startNanos, endNanos));
  if (fullDomain) {
    const clamped = clampTemporalDomain(nextStart, nextEnd, fullDomain);
    if (clamped === null) {
      return;
    }
    [nextStart, nextEnd] = clamped;
  } else {
    nextStart = Math.max(0, nextStart);
    nextEnd = Math.max(0, nextEnd);
  }
  if (nextEnd - nextStart < MIN_TEMPORAL_DOMAIN_NS) {
    return;
  }
  temporalDomainStart.val = new Duration(nextStart);
  temporalDomainEnd.val = new Duration(nextEnd);
}

function clampTemporalDomain(
  startNanos: number,
  endNanos: number,
  fullDomain: DurationDomain,
): DurationDomain | null {
  const [fullStart, fullEnd] = fullDomain;
  if (fullEnd <= fullStart || endNanos <= startNanos) {
    return null;
  }
  const requestedWidth = endNanos - startNanos;
  const fullWidth = fullEnd - fullStart;
  if (requestedWidth >= fullWidth) {
    return [fullStart, fullEnd];
  }
  if (startNanos < fullStart) {
    return [fullStart, fullStart + requestedWidth];
  }
  if (endNanos > fullEnd) {
    return [fullEnd - requestedWidth, fullEnd];
  }
  return [startNanos, endNanos];
}

function resetTemporalDomain(
  temporalDomainStart: DurationValue,
  temporalDomainEnd: DurationValue,
): void {
  temporalDomainStart.val = new Duration(0);
  temporalDomainEnd.val = new Duration(0);
}

function renderedTraceDurationDomain(trace: Trace<unknown> | undefined): DurationDomain | null {
  if (!trace || !(trace.axis.min instanceof Duration) || !(trace.axis.max instanceof Duration)) {
    return null;
  }
  return [trace.axis.min.nanos, trace.axis.max.nanos];
}

function renderedTraceFullDurationDomain(trace: Trace<unknown> | undefined): DurationDomain | null {
  if (!trace) {
    return null;
  }
  if (
    !trace.properties.has(TRACE_FULL_START_KEY) ||
    !trace.properties.has(TRACE_FULL_END_KEY)
  ) {
    return renderedTraceDurationDomain(trace);
  }
  return [
    trace.properties.expectDuration(TRACE_FULL_START_KEY).nanos,
    trace.properties.expectDuration(TRACE_FULL_END_KEY).nanos,
  ];
}

function parseRenderedTrace(
  data: ResponseNode | undefined,
  selectedTraceID: string,
  core: AppCore,
  source: string,
): Trace<unknown> | undefined {
  if (!data || selectedTraceID === '') {
    return undefined;
  }
  if (!data.properties.has(TRACE_ID_KEY)) {
    return undefined;
  }
  try {
    const responseTraceID = data.properties.expectString(TRACE_ID_KEY);
    if (responseTraceID !== selectedTraceID) {
      return undefined;
    }
    return Trace.fromNode(data);
  } catch (err: unknown) {
    core.err(
      err instanceof ConfigurationError
        ? err
        : new ConfigurationError(String(err)).from(source).at(Severity.ERROR),
    );
    return undefined;
  }
}

class SetTemporalDomainFromLocal extends Update {
  constructor(
    private readonly temporalDomainStart: DurationValue,
    private readonly temporalDomainEnd: DurationValue,
  ) {
    super();
  }

  override update(localState?: ValueMap): void {
    const start = localState?.get(TRACE_ZOOM_START_KEY);
    const end = localState?.get(TRACE_ZOOM_END_KEY);
    if (!(start instanceof DurationValue) || !(end instanceof DurationValue)) {
      return;
    }
    setTemporalDomain(
      this.temporalDomainStart,
      this.temporalDomainEnd,
      start.val.nanos,
      end.val.nanos,
    );
  }

  override get autoDocument(): string {
    return 'sets the committed temporal domain from a brushed trace range';
  }
}

class ResetTemporalDomain extends Update {
  constructor(
    private readonly temporalDomainStart: DurationValue,
    private readonly temporalDomainEnd: DurationValue,
  ) {
    super();
  }

  override update(): void {
    resetTemporalDomain(this.temporalDomainStart, this.temporalDomainEnd);
  }

  override get autoDocument(): string {
    return 'resets the committed temporal domain to the whole trace';
  }
}

class KeyboardTemporalZoom extends Update {
  constructor(
    private readonly temporalDomainStart: DurationValue,
    private readonly temporalDomainEnd: DurationValue,
    private readonly currentDomain: () => DurationDomain | null,
    private readonly fullDomain: () => DurationDomain | null,
  ) {
    super();
  }

  override update(localState?: ValueMap): void {
    const depressedKeys = localState?.get(DEPRESSED_KEY_CODES_KEY);
    if (!(depressedKeys instanceof StringSetValue)) {
      return;
    }
    const domain = this.currentDomain();
    if (domain === null) {
      return;
    }
    const [start, end] = domain;
    const width = end - start;
    const fullDomain = this.fullDomain();
    if (width < MIN_TEMPORAL_DOMAIN_NS) {
      return;
    }
    if (depressedKeys.val.has(KEY_ZOOM_IN) || depressedKeys.val.has(KEY_ZOOM_OUT)) {
      const factor = depressedKeys.val.has(KEY_ZOOM_IN)
        ? ZOOM_IN_FACTOR
        : ZOOM_OUT_FACTOR;
      const center = start + width / 2;
      const nextWidth = width * factor;
      setTemporalDomain(
        this.temporalDomainStart,
        this.temporalDomainEnd,
        center - nextWidth / 2,
        center + nextWidth / 2,
        fullDomain,
      );
      return;
    }
    if (depressedKeys.val.has(KEY_PAN_LEFT) || depressedKeys.val.has(KEY_PAN_RIGHT)) {
      const direction = depressedKeys.val.has(KEY_PAN_RIGHT) ? 1 : -1;
      const delta = width * PAN_FRACTION * direction;
      const nextStart = start + delta;
      const nextEnd = end + delta;
      setTemporalDomain(
        this.temporalDomainStart,
        this.temporalDomainEnd,
        nextStart,
        nextEnd,
        fullDomain,
      );
    }
  }

  override get autoDocument(): string {
    return 'updates the committed temporal domain from WASD key state';
  }
}

function createCausalTracingState(): CausalTracingState {
  const corpusPath = new StringValue(DEFAULT_TRACE_PATH);
  const traceID = new StringValue('');
  const expandedCategoryIDs = new StringSetValue(new Set<string>());
  const focusSpanIDs = new StringListValue([]);
  const temporalDomainStart = new DurationValue(new Duration(0));
  const temporalDomainEnd = new DurationValue(new Duration(0));
  const criticalPathStart = new StringValue(DEFAULT_CRITICAL_PATH_START);
  const criticalPathEnd = new StringValue(DEFAULT_CRITICAL_PATH_END);
  const criticalPathStrategy = new StringValue(DEFAULT_CRITICAL_PATH_STRATEGY);
  const search = new StringValue('');
  const draftSearch = new StringValue('');
  const transformTemplate = new StringValue('');
  const draftTransformTemplate = new StringValue('');
  const expandMatches = new StringValue('false');
  const searchValidationRequest = new IntegerValue(0);
  const transformValidationRequest = new IntegerValue(0);
  const traceViewWidthPx = new IntegerValue(0);
  const calledOutCategoryID = new StringValue('');
  const core = new AppCore();
  core.globalState.set('corpus_path', corpusPath);
  core.globalState.set('trace_id', traceID);
  core.globalState.set(EXPANDED_CATEGORY_IDS_KEY, expandedCategoryIDs);
  core.globalState.set(FOCUS_SPAN_IDS_KEY, focusSpanIDs);
  core.globalState.set(TEMPORAL_DOMAIN_START_KEY, temporalDomainStart);
  core.globalState.set(TEMPORAL_DOMAIN_END_KEY, temporalDomainEnd);
  core.globalState.set(CRITICAL_PATH_START_KEY, criticalPathStart);
  core.globalState.set(CRITICAL_PATH_END_KEY, criticalPathEnd);
  core.globalState.set(CRITICAL_PATH_STRATEGY_KEY, criticalPathStrategy);
  core.globalState.set(SEARCH_KEY, search);
  core.globalState.set(DRAFT_SEARCH_KEY, draftSearch);
  core.globalState.set(TRANSFORM_TEMPLATE_KEY, transformTemplate);
  core.globalState.set(DRAFT_TRANSFORM_TEMPLATE_KEY, draftTransformTemplate);
  core.globalState.set(EXPAND_MATCHES_KEY, expandMatches);

  const dataQuery = core.addDataQuery();
  dataQuery.connect(new HttpDataFetcher(core));
  dataQuery.setGlobalFilters(
    new ValueMap(
      new Map<string, Value>([
        ['corpus_path', corpusPath],
        ['trace_id', traceID],
        [EXPANDED_CATEGORY_IDS_KEY, expandedCategoryIDs],
        [FOCUS_SPAN_IDS_KEY, focusSpanIDs],
        [TEMPORAL_DOMAIN_START_KEY, temporalDomainStart],
        [TEMPORAL_DOMAIN_END_KEY, temporalDomainEnd],
        [CRITICAL_PATH_START_KEY, criticalPathStart],
        [CRITICAL_PATH_END_KEY, criticalPathEnd],
        [CRITICAL_PATH_STRATEGY_KEY, criticalPathStrategy],
        [SEARCH_KEY, search],
        [DRAFT_SEARCH_KEY, draftSearch],
        [TRANSFORM_TEMPLATE_KEY, transformTemplate],
        [DRAFT_TRANSFORM_TEMPLATE_KEY, draftTransformTemplate],
        [EXPAND_MATCHES_KEY, expandMatches],
      ]),
    ),
  );
  dataQuery.debounceUpdates(50);
  core.publish();

  return {
    core,
    dataQuery,
    corpusPath,
    traceID,
    expandedCategoryIDs,
    focusSpanIDs,
    temporalDomainStart,
    temporalDomainEnd,
    criticalPathStart,
    criticalPathEnd,
    criticalPathStrategy,
    search,
    draftSearch,
    transformTemplate,
    draftTransformTemplate,
    expandMatches,
    searchValidationRequest,
    transformValidationRequest,
    traceViewWidthPx,
    calledOutCategoryID,
  };
}

function useDataSeries(
  dataQuery: DataQuery,
  queryName: string,
  parameters: ValueMap,
  fetch: Predicate,
): SeriesResult {
  const [data, setData] = useState<ResponseNode | undefined>(undefined);
  const [loading, setLoading] = useState<boolean>(false);

  useEffect(() => {
    const seriesQuery = new DataSeriesQuery(
      dataQuery,
      new StringValue(queryName),
      parameters,
      fetch.match()(),
    );
    const subscriptions: Subscription[] = [
      seriesQuery.response.subscribe(setData),
      seriesQuery.loading.subscribe(setLoading),
    ];
    return () => {
      subscriptions.forEach((sub) => sub.unsubscribe());
      seriesQuery.dispose();
    };
  }, [dataQuery, fetch, parameters, queryName]);

  return { data, loading };
}

function useMeasuredWidth(): [RefCallback<HTMLDivElement>, number] {
	const [element, setElement] = useState<HTMLDivElement | null>(null);
	const [widthPx, setWidthPx] = useState<number>(0);

	useEffect(() => {
		if (!element) {
			setWidthPx(0);
			return;
		}
		const updateWidth = (nextWidthPx: number): void => {
			setWidthPx(Math.max(0, Math.round(nextWidthPx)));
    };
    updateWidth(element.getBoundingClientRect().width);
    const observer = new ResizeObserver((entries) => {
      const [entry] = entries;
      if (entry) {
        updateWidth(entry.contentRect.width);
      }
    });
    observer.observe(element);
		return () => {
			observer.disconnect();
		};
	}, [element]);

	const ref = useCallback((node: HTMLDivElement | null): void => {
		setElement(node);
	}, []);

	return [ref, widthPx];
}

export default function App(): JSX.Element {
  const state = useMemo(() => createCausalTracingState(), []);
  const corpusPath = useValue(state.corpusPath, DEFAULT_TRACE_PATH) ?? '';
  const selectedTraceID = useValue(state.traceID, '') ?? '';
  const focusSpanIDs = useValue<string[]>(state.focusSpanIDs, []) ?? [];
  const focusSpanID = focusSpanIDs.length > 0 ? focusSpanIDs[0] : '';
  const criticalPathStart = useValue(
    state.criticalPathStart,
    DEFAULT_CRITICAL_PATH_START,
  ) ?? DEFAULT_CRITICAL_PATH_START;
  const criticalPathEnd = useValue(
    state.criticalPathEnd,
    DEFAULT_CRITICAL_PATH_END,
  ) ?? DEFAULT_CRITICAL_PATH_END;
  const criticalPathStrategy = useValue(
    state.criticalPathStrategy,
    DEFAULT_CRITICAL_PATH_STRATEGY,
  ) ?? DEFAULT_CRITICAL_PATH_STRATEGY;
  const draftSearch = useValue(state.draftSearch, '') ?? '';
  const transformTemplate = useValue(state.transformTemplate, '') ?? '';
  const draftTransformTemplate = useValue(
    state.draftTransformTemplate,
    '',
  ) ?? '';
  const expandMatches = useValue(state.expandMatches, 'false') === 'true';
  const [searchValidationError, setSearchValidationError] = useState<string>('');
  const [transformModalOpen, setTransformModalOpen] = useState<boolean>(false);
  const [transformValidationError, setTransformValidationError] =
    useState<string>('');
  const [tracePanelRef, measuredTraceWidthPx] = useMeasuredWidth();
  useEffect(() => {
    if (measuredTraceWidthPx > 0) {
      state.traceViewWidthPx.val = measuredTraceWidthPx;
    }
  }, [measuredTraceWidthPx, state.traceViewWidthPx]);
  const emptyParams = useMemo(() => new ValueMap(), []);
  const traceParams = useMemo(
    () =>
      new ValueMap(
        new Map<string, Value>([
          [TRACE_VIEW_WIDTH_PX_PARAM, state.traceViewWidthPx],
        ]),
      ),
    [state.traceViewWidthPx],
  );
  const globalRef = (key: string): GlobalValueRef =>
    new GlobalValueRef(state.core, key);
  const corpusChanged = useMemo(
    () => new Changed([globalRef('corpus_path')]),
    [state.core],
  );
  const traceChanged = useMemo(
    () =>
      new Changed([
        globalRef('corpus_path'),
        globalRef('trace_id'),
        globalRef(TRANSFORM_TEMPLATE_KEY),
      ]),
    [state.core],
  );
	const traceRenderChanged = useMemo(
		() =>
			new Changed([
				globalRef('corpus_path'),
				globalRef('trace_id'),
        globalRef(EXPANDED_CATEGORY_IDS_KEY),
        globalRef(FOCUS_SPAN_IDS_KEY),
        globalRef(TEMPORAL_DOMAIN_START_KEY),
        globalRef(TEMPORAL_DOMAIN_END_KEY),
        globalRef(CRITICAL_PATH_START_KEY),
        globalRef(CRITICAL_PATH_END_KEY),
        globalRef(CRITICAL_PATH_STRATEGY_KEY),
        globalRef(SEARCH_KEY),
        globalRef(TRANSFORM_TEMPLATE_KEY),
        globalRef(EXPAND_MATCHES_KEY),
				new DirectValueRef(state.traceViewWidthPx, 'trace view width'),
      ]),
    [state.core, state.traceViewWidthPx],
  );
  const spanFocusChanged = useMemo(
    () =>
      new Changed([
        globalRef('corpus_path'),
        globalRef('trace_id'),
        globalRef(TRANSFORM_TEMPLATE_KEY),
        globalRef(FOCUS_SPAN_IDS_KEY),
      ]),
    [state.core],
  );
  const searchValidationChanged = useMemo(
    () =>
      new Changed([
        new DirectValueRef(
          state.searchValidationRequest,
          'search validation request',
        ),
      ]),
    [state.searchValidationRequest],
  );
  const transformValidationChanged = useMemo(
    () =>
      new Changed([
        new DirectValueRef(
          state.transformValidationRequest,
          'transform validation request',
        ),
      ]),
    [state.transformValidationRequest],
  );
  const status = useDataSeries(
    state.dataQuery,
    LOAD_STATUS_QUERY,
    emptyParams,
    corpusChanged,
  );
  const corpusTraces = useDataSeries(
    state.dataQuery,
    CORPUS_TRACES_QUERY,
    emptyParams,
    corpusChanged,
  );
  const traceStatus = useDataSeries(
    state.dataQuery,
    TRACE_STATUS_QUERY,
    emptyParams,
    traceChanged,
  );
  const traceDiagnostics = useDataSeries(
    state.dataQuery,
    TRACE_DIAGNOSTICS_QUERY,
    emptyParams,
    traceChanged,
  );
  const renderedTraceResult = useDataSeries(
    state.dataQuery,
    TRACE_QUERY,
    traceParams,
    traceRenderChanged,
  );
  const renderedCriticalPathResult = useDataSeries(
    state.dataQuery,
    CRITICAL_PATH_TRACE_QUERY,
    traceParams,
    traceRenderChanged,
  );
  const spanCausality = useDataSeries(
    state.dataQuery,
    SPAN_CAUSALITY_QUERY,
    emptyParams,
    spanFocusChanged,
  );
  const searchValidation = useDataSeries(
    state.dataQuery,
    VALIDATE_SEARCH_QUERY,
    emptyParams,
    searchValidationChanged,
  );
  const transformValidation = useDataSeries(
    state.dataQuery,
    VALIDATE_TRANSFORM_QUERY,
    emptyParams,
    transformValidationChanged,
  );
  useEffect(() => {
    const data = searchValidation.data;
    if (!data) {
      return;
    }
    try {
      const validatedDraft = data.properties.expectString(DRAFT_SEARCH_KEY);
      if (validatedDraft !== state.draftSearch.val) {
        return;
      }
      const status = data.properties.expectString(STATUS_KEY);
      const message = data.properties.has(MESSAGE_KEY)
        ? data.properties.expectString(MESSAGE_KEY)
        : '';
      if (status === 'ok') {
        setSearchValidationError('');
        state.search.val = validatedDraft;
        return;
      }
      setSearchValidationError(message || 'Invalid search pattern');
      state.search.val = '';
    } catch (err: unknown) {
      state.core.err(
        err instanceof ConfigurationError
          ? err
          : new ConfigurationError(String(err))
              .from('causal-tracing.search-validation')
              .at(Severity.ERROR),
      );
    }
  }, [searchValidation.data, state.core, state.draftSearch, state.search]);
  useEffect(() => {
    const data = transformValidation.data;
    if (!data) {
      return;
    }
    try {
      const validatedDraft = data.properties.expectString(
        DRAFT_TRANSFORM_TEMPLATE_KEY,
      );
      if (validatedDraft !== state.draftTransformTemplate.val) {
        return;
      }
      const status = data.properties.expectString(STATUS_KEY);
      const message = data.properties.has(MESSAGE_KEY)
        ? data.properties.expectString(MESSAGE_KEY)
        : '';
      if (status === 'ok') {
        setTransformValidationError('');
        state.transformTemplate.val = validatedDraft;
        state.focusSpanIDs.val = [];
        state.calledOutCategoryID.val = '';
        resetTemporalDomain(state.temporalDomainStart, state.temporalDomainEnd);
        setTransformModalOpen(false);
        return;
      }
      setTransformValidationError(message || 'Invalid transform template');
    } catch (err: unknown) {
      state.core.err(
        err instanceof ConfigurationError
          ? err
          : new ConfigurationError(String(err))
              .from('causal-tracing.transform-validation')
              .at(Severity.ERROR),
      );
    }
  }, [
    transformValidation.data,
    state.calledOutCategoryID,
    state.core,
    state.draftTransformTemplate,
    state.focusSpanIDs,
    state.temporalDomainEnd,
    state.temporalDomainStart,
    state.transformTemplate,
  ]);
  const renderedTrace = useMemo(
    () =>
      parseRenderedTrace(
        renderedTraceResult.data,
        selectedTraceID,
        state.core,
        'causal-tracing.trace-view',
      ),
    [renderedTraceResult.data, selectedTraceID, state.core],
  );
  const renderedCriticalPathTrace = useMemo(
    () =>
      parseRenderedTrace(
        renderedCriticalPathResult.data,
        selectedTraceID,
        state.core,
        'causal-tracing.critical-path-view',
      ),
    [renderedCriticalPathResult.data, selectedTraceID, state.core],
  );
  const corpusInteractions = useMemo(() => {
    const traceID = new LocalValue('trace_id');
    return new Interactions()
      .withAction(
        new Action('rows', 'click', [
          new SetAction(globalRef('trace_id'), traceID),
          new Clear([new DirectValueRef(state.focusSpanIDs, 'focus span stack')]),
          new ResetTemporalDomain(
            state.temporalDomainStart,
            state.temporalDomainEnd,
          ),
        ]),
      )
      .withReaction(
        new Reaction(
          'rows',
          'highlight',
          new Equals(globalRef('trace_id'), traceID),
        ),
      );
  }, [
    state.core,
    state.focusSpanIDs,
    state.temporalDomainEnd,
    state.temporalDomainStart,
  ]);
  const traceInteractions = useMemo(() => {
    return new Interactions()
      .withAction(
        new Action(TRACE_SPANS_TARGET, TRACE_SPAN_CLICK_ACTION, [
          new PushNonEmptyLocalStringLeft(state.focusSpanIDs, SPAN_ID_KEY),
        ]),
      )
      .withAction(
        new Action(TRACE_CHART_TARGET, TRACE_BRUSH_ACTION, [
          new SetTemporalDomainFromLocal(
            state.temporalDomainStart,
            state.temporalDomainEnd,
          ),
        ]),
      )
      .withAction(
        new Action(TRACE_CHART_TARGET, TRACE_RESET_ZOOM_ACTION, [
          new ResetTemporalDomain(
            state.temporalDomainStart,
            state.temporalDomainEnd,
          ),
        ]),
      )
      .withWatch(
        new Watch(
          UPDATE_CALLED_OUT_CATEGORY_WATCH,
          new ValueMap(
            new Map<string, Value>([
              [CALLED_OUT_CATEGORY_ID_KEY, state.calledOutCategoryID],
              [CATEGORY_ID_KEY, new StringValue(CATEGORY_ID_PROPERTY_KEY)],
            ]),
          ),
        ),
      );
  }, [
    state.calledOutCategoryID,
    state.focusSpanIDs,
    state.temporalDomainEnd,
    state.temporalDomainStart,
  ]);
  const keypressInteractions = useMemo(() => {
    return new Interactions().withAction(
      new Action(KEY_TARGET, KEY_PRESS_ACTION, [
        new KeyboardTemporalZoom(
          state.temporalDomainStart,
          state.temporalDomainEnd,
          () => renderedTraceDurationDomain(renderedTrace),
          () => renderedTraceFullDurationDomain(renderedTrace),
        ),
      ]),
    );
  }, [renderedTrace, state.temporalDomainEnd, state.temporalDomainStart]);
  const categoryAxisInteractions = useMemo(() => {
    const categoryID = new LocalValue(CATEGORY_ID_PROPERTY_KEY);
    const calledOutCategoryID = new DirectValueRef(
      state.calledOutCategoryID,
      'called-out category ID',
    );
    return new Interactions()
      .withAction(
        new Action(CATEGORY_HEADERS_TARGET, CATEGORY_MOUSEOVER_ACTION, [
          new SetAction(calledOutCategoryID, categoryID),
        ]),
      )
      .withAction(
        new Action(CATEGORY_HEADERS_TARGET, CATEGORY_CLICK_ACTION, [
          new ToggleCategoryExpansion(state.expandedCategoryIDs),
        ]),
      )
      .withAction(
        new Action(CATEGORY_HEADERS_TARGET, CATEGORY_MOUSEOUT_ACTION, [
          new Clear([calledOutCategoryID]),
        ]),
      );
  }, [state.calledOutCategoryID, state.expandedCategoryIDs]);
  const spanCausalityInteractions = useMemo(() => {
    return new Interactions().withAction(
      new Action('rows', 'click', [
        new PushNonEmptyLocalStringLeft(state.focusSpanIDs, OTHER_SPAN_ID_KEY),
      ]),
    );
  }, [state.focusSpanIDs]);

  const clearSelectedTrace = (): void => {
    state.focusSpanIDs.val = [];
    state.search.val = '';
    state.draftSearch.val = '';
    state.transformTemplate.val = '';
    state.draftTransformTemplate.val = '';
    setSearchValidationError('');
    setTransformValidationError('');
    setTransformModalOpen(false);
    resetTemporalDomain(state.temporalDomainStart, state.temporalDomainEnd);
    state.traceID.val = '';
  };

  const setCorpusPath = (nextCorpusPath: string): void => {
    state.focusSpanIDs.val = [];
    state.search.val = '';
    state.draftSearch.val = '';
    state.transformTemplate.val = '';
    state.draftTransformTemplate.val = '';
    setSearchValidationError('');
    setTransformValidationError('');
    setTransformModalOpen(false);
    resetTemporalDomain(state.temporalDomainStart, state.temporalDomainEnd);
    state.traceID.val = '';
    state.corpusPath.val = nextCorpusPath;
  };

  const popFocusSpan = (): void => {
    state.focusSpanIDs.val = focusSpanIDs.slice(1);
  };

  const clearFocusSpans = (): void => {
    state.focusSpanIDs.val = [];
  };

  const resetZoom = (): void => {
    resetTemporalDomain(state.temporalDomainStart, state.temporalDomainEnd);
  };

  const applySearch = (): void => {
    state.searchValidationRequest.val = state.searchValidationRequest.val + 1;
  };

  const clearSearch = (): void => {
    state.draftSearch.val = '';
    state.search.val = '';
    setSearchValidationError('');
  };

  const openTransformModal = (): void => {
    state.draftTransformTemplate.val = state.transformTemplate.val;
    setTransformValidationError('');
    setTransformModalOpen(true);
  };

  const closeTransformModal = (): void => {
    state.draftTransformTemplate.val = state.transformTemplate.val;
    setTransformValidationError('');
    setTransformModalOpen(false);
  };

  const commitTransform = (): void => {
    state.transformValidationRequest.val = state.transformValidationRequest.val + 1;
  };

  const clearTransform = (): void => {
    state.draftTransformTemplate.val = '';
    state.transformTemplate.val = '';
    state.focusSpanIDs.val = [];
    state.calledOutCategoryID.val = '';
    setTransformValidationError('');
    resetTemporalDomain(state.temporalDomainStart, state.temporalDomainEnd);
    setTransformModalOpen(false);
  };

  const loading = selectedTraceID === ''
    ? status.loading || corpusTraces.loading
    : traceStatus.loading || traceDiagnostics.loading || renderedTraceResult.loading || renderedCriticalPathResult.loading || spanCausality.loading || searchValidation.loading || transformValidation.loading;

  const corpusView = (
    <>
      <section className="status-panel" aria-label="Corpus load status">
        <div className="panel-heading">
          <h1>Causal trace corpus</h1>
          <span className={loading ? 'status-badge busy' : 'status-badge'}>
            {loading ? 'Loading' : 'Ready'}
          </span>
        </div>
        <DataTable
          data={status.data}
          loading={status.loading}
          withPagination={false}
          scrollable={false}
        />
      </section>
      <section className="table-panel" aria-label="Corpus traces">
        <div className="panel-heading">
          <h2>Traces</h2>
        </div>
        <DataTable
          data={corpusTraces.data}
          loading={corpusTraces.loading}
          interactions={corpusInteractions}
        />
      </section>
    </>
  );

  const traceView = (
    <>
      <section className="status-panel" aria-label="Selected trace status">
        <div className="panel-heading">
          <h1>Selected trace</h1>
          <button
            className="plain-button"
            type="button"
            onClick={clearSelectedTrace}
          >
            Back to corpus
          </button>
        </div>
        <DataTable
          data={traceStatus.data}
          loading={traceStatus.loading}
          withPagination={false}
          scrollable={false}
        />
      </section>
      <section
        className="trace-panel"
        ref={tracePanelRef}
        aria-label="Selected trace timeline"
      >
        <div className="panel-heading">
          <h2>{focusSpanIDs.length > 0 ? 'Focused span stack' : 'Timeline'}</h2>
          <div className="panel-actions">
            <button className="plain-button" type="button" onClick={resetZoom}>
              Reset zoom
            </button>
            <span
              className={
                renderedTraceResult.loading ? 'status-badge busy' : 'status-badge'
              }
            >
              {renderedTraceResult.loading ? 'Loading' : 'Ready'}
            </span>
          </div>
        </div>
        {focusSpanIDs.length === 0 ? (
          <>
            <div className="search-controls" aria-label="Trace search controls">
              <label
                className={
                  searchValidationError === ''
                    ? 'field search-field'
                    : 'field search-field invalid'
                }
              >
                <span>Search</span>
                <input
                  value={draftSearch}
                  onChange={(event) => {
                    state.draftSearch.val = event.target.value;
                    state.search.val = '';
                    setSearchValidationError('');
                  }}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault();
                      applySearch();
                    }
                  }}
                />
                {searchValidationError !== '' ? (
                  <span className="field-error">{searchValidationError}</span>
                ) : null}
              </label>
              <button
                className="plain-button"
                type="button"
                disabled={searchValidation.loading}
                onClick={applySearch}
              >
                {searchValidation.loading ? 'Validating' : 'Search'}
              </button>
              <button className="plain-button" type="button" onClick={clearSearch}>
                Clear
              </button>
              <label className="checkbox-field">
                <input
                  type="checkbox"
                  checked={expandMatches}
                  onChange={(event) => {
                    state.expandMatches.val = event.target.checked ? 'true' : 'false';
                  }}
                />
                <span>Expand matches</span>
              </label>
            </div>
            <div className="critical-path-controls" aria-label="Critical path controls">
              <label className="field compact">
                <span>CP start</span>
                <input
                  value={criticalPathStart}
                  onChange={(event) => {
                    state.criticalPathStart.val = event.target.value;
                  }}
                />
              </label>
              <label className="field compact">
                <span>CP end</span>
                <input
                  value={criticalPathEnd}
                  onChange={(event) => {
                    state.criticalPathEnd.val = event.target.value;
                  }}
                />
              </label>
              <label className="field compact">
                <span>Pathing</span>
                <select
                  value={criticalPathStrategy}
                  onChange={(event) => {
                    state.criticalPathStrategy.val = event.target.value;
                  }}
                >
                  {CRITICAL_PATH_STRATEGY_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          </>
        ) : null}
        {focusSpanIDs.length > 0 ? (
          <div className="focus-bar" aria-label="Focused span stack controls">
            <span className="focus-head">Head: {focusSpanID}</span>
            <span>{focusSpanIDs.length} span{focusSpanIDs.length === 1 ? '' : 's'}</span>
            <button className="plain-button" type="button" onClick={popFocusSpan}>
              Pop
            </button>
            <button className="plain-button" type="button" onClick={clearFocusSpans}>
              Close focus
            </button>
          </div>
        ) : null}
        <div className="trace-viewport">
          {renderedTrace && measuredTraceWidthPx > 0 ? (
            <HorizontalTrace
              trace={renderedTrace}
              widthPx={measuredTraceWidthPx}
              interactions={traceInteractions}
              yAxisInteractions={categoryAxisInteractions}
              transitionDurationMs={0}
              renderYAxis={({ renderedCategories, interactions }) => (
                <RectangularCategoryHierarchyYAxis
                  renderedCategories={renderedCategories}
                  interactions={interactions}
                  transitionDurationMs={0}
                />
              )}
              renderXAxis={({ trace, renderSettings, widthPx }) => (
                <StandardContinuousXAxis
                  axis={trace.axis as Axis<unknown>}
                  renderSettings={renderSettings}
                  widthPx={widthPx}
                />
              )}
            />
          ) : (
            <div className="empty-trace">
              {renderedTraceResult.loading ? 'Loading trace' : 'No trace loaded'}
            </div>
          )}
        </div>
      </section>
      {focusSpanIDs.length === 0 ? (
        <section
          className="trace-panel compact-trace-panel"
          aria-label="Overtime critical path"
        >
          <div className="panel-heading">
            <h2>Critical path overtime</h2>
            <span
              className={
                renderedCriticalPathResult.loading ? 'status-badge busy' : 'status-badge'
              }
            >
              {renderedCriticalPathResult.loading ? 'Loading' : 'Ready'}
            </span>
          </div>
          <div className="trace-viewport critical-path-viewport">
            {renderedCriticalPathTrace && measuredTraceWidthPx > 0 ? (
              <HorizontalTrace
                trace={renderedCriticalPathTrace}
                widthPx={measuredTraceWidthPx}
                interactions={traceInteractions}
                yAxisInteractions={categoryAxisInteractions}
                transitionDurationMs={0}
                renderYAxis={({ renderedCategories, interactions }) => (
                  <RectangularCategoryHierarchyYAxis
                    renderedCategories={renderedCategories}
                    interactions={interactions}
                    transitionDurationMs={0}
                  />
                )}
                renderXAxis={({ trace, renderSettings, widthPx }) => (
                  <StandardContinuousXAxis
                    axis={trace.axis as Axis<unknown>}
                    renderSettings={renderSettings}
                    widthPx={widthPx}
                  />
                )}
              />
            ) : (
              <div className="empty-trace">
                {renderedCriticalPathResult.loading
                  ? 'Loading critical path'
                  : 'No critical path loaded'}
              </div>
            )}
          </div>
        </section>
      ) : null}
      {focusSpanIDs.length > 0 ? (
        <section className="table-panel" aria-label="Focused span causality">
          <div className="panel-heading">
            <h2>Span causality</h2>
            <span
              className={
                spanCausality.loading ? 'status-badge busy' : 'status-badge'
              }
            >
              {spanCausality.loading ? 'Loading' : 'Ready'}
            </span>
          </div>
          <DataTable
            data={spanCausality.data}
            loading={spanCausality.loading}
            interactions={spanCausalityInteractions}
          />
        </section>
      ) : null}
      <section className="table-panel" aria-label="Selected trace diagnostics">
        <div className="panel-heading">
          <h2>Nonfatal diagnostics</h2>
          <span
            className={
              traceDiagnostics.loading ? 'status-badge busy' : 'status-badge'
            }
          >
            {traceDiagnostics.loading ? 'Loading' : 'Ready'}
          </span>
        </div>
        <DataTable
          data={traceDiagnostics.data}
          loading={traceDiagnostics.loading}
        />
      </section>
    </>
  );

  return (
    <AppCoreContext.Provider value={state.core}>
      <KeypressListener interactions={keypressInteractions} />
      <main className="app-shell">
        <section className="toolbar" aria-label="Trace source">
          <label className="field">
            <span>Corpus file</span>
            <input
              value={corpusPath}
              onChange={(event) => {
                setCorpusPath(event.target.value);
              }}
            />
          </label>
          {selectedTraceID !== '' ? (
            <label className="field compact">
              <span>Trace ID</span>
              <input
                value={selectedTraceID}
                onChange={(event) => {
                  state.focusSpanIDs.val = [];
                  state.transformTemplate.val = '';
                  state.draftTransformTemplate.val = '';
                  setTransformValidationError('');
                  resetTemporalDomain(
                    state.temporalDomainStart,
                    state.temporalDomainEnd,
                  );
                  state.traceID.val = event.target.value;
                }}
              />
            </label>
          ) : null}
          {selectedTraceID !== '' ? (
            <div className="toolbar-actions">
              <button
                className="plain-button"
                type="button"
                onClick={openTransformModal}
              >
                Transform
              </button>
              {transformTemplate !== '' ? (
                <span className="status-badge transformed">Transformed</span>
              ) : null}
            </div>
          ) : null}
        </section>
        {selectedTraceID === '' ? corpusView : traceView}
      </main>
      {transformModalOpen ? (
        <div className="modal-backdrop" role="presentation">
          <section
            className="modal-panel"
            role="dialog"
            aria-modal="true"
            aria-label="Trace transform"
          >
            <div className="panel-heading">
              <h2>Trace transform</h2>
              <button
                className="plain-button"
                type="button"
                onClick={closeTransformModal}
              >
                Close
              </button>
            </div>
            <label
              className={
                transformValidationError === ''
                  ? 'field transform-field'
                  : 'field transform-field invalid'
              }
            >
              <span>Transform template</span>
              <textarea
                value={draftTransformTemplate}
                onChange={(event) => {
                  state.draftTransformTemplate.val = event.target.value;
                  setTransformValidationError('');
                }}
              />
              {transformValidationError !== '' ? (
                <span className="field-error">{transformValidationError}</span>
              ) : null}
            </label>
            <div className="modal-actions">
              <button
                className="plain-button"
                type="button"
                disabled={transformValidation.loading}
                onClick={commitTransform}
              >
                {transformValidation.loading ? 'Transforming' : 'Transform'}
              </button>
              <button
                className="plain-button"
                type="button"
                onClick={clearTransform}
              >
                Clear transform
              </button>
            </div>
          </section>
        </div>
      ) : null}
      <ErrorToast />
    </AppCoreContext.Provider>
  );
}
