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
	UrlHash,
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

const DEFAULT_TRACE_PATH = '';
const DEFAULT_THEME = 'light';
const LOAD_STATUS_QUERY = 'causal_tracing.load_status';
const CORPUS_TRACES_QUERY = 'causal_tracing.corpus_traces';
const TRACE_STATUS_QUERY = 'causal_tracing.trace_status';
const HIERARCHY_TYPES_QUERY = 'causal_tracing.hierarchy_types';
const CRITICAL_PATH_STRATEGIES_QUERY =
  'causal_tracing.critical_path_strategies';
const TRACE_DIAGNOSTICS_QUERY = 'causal_tracing.trace_diagnostics';
const TRACE_QUERY = 'causal_tracing.trace';
const CRITICAL_PATH_TRACE_QUERY = 'causal_tracing.critical_path_trace';
const SPAN_CAUSALITY_QUERY = 'causal_tracing.span_causality';
const VALIDATE_SEARCH_QUERY = 'causal_tracing.validate_search';
const VALIDATE_TRANSFORM_QUERY = 'causal_tracing.validate_transform';
const VALIDATE_CRITICAL_PATH_QUERY = 'causal_tracing.validate_critical_path';
const DEFAULT_CRITICAL_PATH_START = '';
const DEFAULT_CRITICAL_PATH_END = '';
const DEFAULT_CRITICAL_PATH_STRATEGY = 'Temporal Max work (non causal)';
const EXPANDED_CATEGORY_IDS_KEY = 'expanded_category_ids';
const FOCUS_SPAN_IDS_KEY = 'focus_span_ids';
const SPAN_ID_KEY = 'span_id';
const OTHER_SPAN_ID_KEY = 'other_span_id';
const TRACE_VIEW_WIDTH_PX_PARAM = 'trace_view_width_px';
const TRACE_ID_KEY = 'trace_id';
const HIERARCHY_TYPE_KEY = 'hierarchy_type';
const HIERARCHY_NAME_KEY = 'hierarchy_name';
const HIERARCHY_DESCRIPTION_KEY = 'hierarchy_description';
const DEFAULT_HIERARCHY_TYPE = 'service';
const CATEGORY_ID_PROPERTY_KEY = 'category_id';
const CATEGORY_EXPANSION_STATE_KEY = 'category_expansion_state';
const CATEGORY_STATE_COLLAPSED = 'collapsed';
const CATEGORY_STATE_EXPANDED = 'expanded';
const TEMPORAL_DOMAIN_START_KEY = 'temporal_domain_start';
const TEMPORAL_DOMAIN_END_KEY = 'temporal_domain_end';
const CRITICAL_PATH_START_KEY = 'critical_path_start';
const CRITICAL_PATH_END_KEY = 'critical_path_end';
const CRITICAL_PATH_STRATEGY_KEY = 'critical_path_strategy';
const CRITICAL_PATH_STRATEGY_DESCRIPTION_KEY =
  'critical_path_strategy_description';
const DRAFT_CRITICAL_PATH_START_KEY = 'draft_critical_path_start';
const DRAFT_CRITICAL_PATH_END_KEY = 'draft_critical_path_end';
const DRAFT_CRITICAL_PATH_STRATEGY_KEY = 'draft_critical_path_strategy';
const SEARCH_KEY = 'search';
const DRAFT_SEARCH_KEY = 'draft_search';
const TRANSFORM_TEMPLATE_KEY = 'transform_template';
const DRAFT_TRANSFORM_TEMPLATE_KEY = 'draft_transform_template';
const EXPAND_MATCHES_KEY = 'expand_matches';
const HIDE_NON_MATCHING_KEY = 'hide_non_matching';
const HIDE_EMPTY_KEY = 'hide_empty';
const SHOW_ONLY_CRITICAL_PATH_KEY = 'show_only_critical_path';
const THEME_KEY = 'theme';
const SEARCH_VALIDATION_REQUEST_KEY = 'search_validation_request';
const TRANSFORM_VALIDATION_REQUEST_KEY = 'transform_validation_request';
const CRITICAL_PATH_VALIDATION_REQUEST_KEY = 'critical_path_validation_request';
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
  hierarchyType: StringValue;
  expandedCategoryIDs: StringSetValue;
  focusSpanIDs: StringListValue;
  temporalDomainStart: DurationValue;
  temporalDomainEnd: DurationValue;
  criticalPathStart: StringValue;
  criticalPathEnd: StringValue;
  criticalPathStrategy: StringValue;
  draftCriticalPathStart: StringValue;
  draftCriticalPathEnd: StringValue;
  draftCriticalPathStrategy: StringValue;
  search: StringValue;
  draftSearch: StringValue;
  transformTemplate: StringValue;
  draftTransformTemplate: StringValue;
  expandMatches: StringValue;
  hideNonMatching: StringValue;
  hideEmpty: StringValue;
  showOnlyCriticalPath: StringValue;
  theme: StringValue;
  searchValidationRequest: IntegerValue;
  transformValidationRequest: IntegerValue;
  criticalPathValidationRequest: IntegerValue;
  traceViewWidthPx: IntegerValue;
  calledOutCategoryID: StringValue;
};

type CriticalPathStrategyOption = {
  value: string;
  label: string;
};

type HierarchyOption = {
  value: string;
  label: string;
  description: string;
};

const FALLBACK_CRITICAL_PATH_STRATEGY_OPTIONS: CriticalPathStrategyOption[] = [
  {
    value: 'Temporal Max work (non causal)',
    label: 'Temporal Max work (non causal)',
  },
  { value: 'Maximize work', label: 'Maximize work' },
  {
    value: 'Prefer traversing causal dependencies',
    label: 'Prefer traversing causal dependencies',
  },
  {
    value: 'Prefer traversing sequential dependencies',
    label: 'Prefer traversing sequential dependencies',
  },
  {
    value: 'Traverse latest-resolving dependencies',
    label: 'Traverse latest-resolving dependencies',
  },
  {
    value: 'Traverse earliest-resolving dependencies',
    label: 'Traverse earliest-resolving dependencies',
  },
  { value: 'Maximize dependency delay', label: 'Maximize dependency delay' },
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

function parseHierarchyOptions(
  data: ResponseNode | undefined,
  core: AppCore,
): HierarchyOption[] {
  if (!data) {
    return [];
  }
  const options: HierarchyOption[] = [];
  for (const row of data.children.slice(1)) {
    try {
      if (!row.properties.has(HIERARCHY_TYPE_KEY)) {
        continue;
      }
      const value = row.properties.expectString(HIERARCHY_TYPE_KEY);
      const description = row.properties.has(HIERARCHY_DESCRIPTION_KEY)
        ? row.properties.expectString(HIERARCHY_DESCRIPTION_KEY)
        : value;
      const label = description || value;
      options.push({ value, label, description });
    } catch (err: unknown) {
      core.err(
        err instanceof ConfigurationError
          ? err
          : new ConfigurationError(String(err))
              .from('causal-tracing.hierarchy-options')
              .at(Severity.ERROR),
      );
    }
  }
  return options;
}

function parseCriticalPathStrategyOptions(
  data: ResponseNode | undefined,
  core: AppCore,
): CriticalPathStrategyOption[] {
  if (!data) {
    return [];
  }
  const options: CriticalPathStrategyOption[] = [];
  for (const row of data.children.slice(1)) {
    try {
      if (!row.properties.has(CRITICAL_PATH_STRATEGY_KEY)) {
        continue;
      }
      const value = row.properties.expectString(CRITICAL_PATH_STRATEGY_KEY);
      const description = row.properties.has(
        CRITICAL_PATH_STRATEGY_DESCRIPTION_KEY,
      )
        ? row.properties.expectString(CRITICAL_PATH_STRATEGY_DESCRIPTION_KEY)
        : value;
      options.push({ value, label: description || value });
    } catch (err: unknown) {
      core.err(
        err instanceof ConfigurationError
          ? err
          : new ConfigurationError(String(err))
              .from('causal-tracing.critical-path-strategy-options')
              .at(Severity.ERROR),
      );
    }
  }
  return options;
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
  const hierarchyType = new StringValue(DEFAULT_HIERARCHY_TYPE);
  const expandedCategoryIDs = new StringSetValue(new Set<string>());
  const focusSpanIDs = new StringListValue([]);
  const temporalDomainStart = new DurationValue(new Duration(0));
  const temporalDomainEnd = new DurationValue(new Duration(0));
  const criticalPathStart = new StringValue(DEFAULT_CRITICAL_PATH_START);
  const criticalPathEnd = new StringValue(DEFAULT_CRITICAL_PATH_END);
  const criticalPathStrategy = new StringValue(DEFAULT_CRITICAL_PATH_STRATEGY);
  const draftCriticalPathStart = new StringValue(DEFAULT_CRITICAL_PATH_START);
  const draftCriticalPathEnd = new StringValue(DEFAULT_CRITICAL_PATH_END);
  const draftCriticalPathStrategy = new StringValue(DEFAULT_CRITICAL_PATH_STRATEGY);
  const search = new StringValue('');
  const draftSearch = new StringValue('');
  const transformTemplate = new StringValue('');
  const draftTransformTemplate = new StringValue('');
  const expandMatches = new StringValue('false');
  const hideNonMatching = new StringValue('false');
  const hideEmpty = new StringValue('false');
  const showOnlyCriticalPath = new StringValue('false');
  const theme = new StringValue(DEFAULT_THEME);
  const searchValidationRequest = new IntegerValue(0);
  const transformValidationRequest = new IntegerValue(0);
  const criticalPathValidationRequest = new IntegerValue(0);
  const traceViewWidthPx = new IntegerValue(0);
  const calledOutCategoryID = new StringValue('');
  const core = new AppCore();
  core.globalState.set('corpus_path', corpusPath);
  core.globalState.set('trace_id', traceID);
  core.globalState.set(HIERARCHY_TYPE_KEY, hierarchyType);
  core.globalState.set(EXPANDED_CATEGORY_IDS_KEY, expandedCategoryIDs);
  core.globalState.set(FOCUS_SPAN_IDS_KEY, focusSpanIDs);
  core.globalState.set(TEMPORAL_DOMAIN_START_KEY, temporalDomainStart);
  core.globalState.set(TEMPORAL_DOMAIN_END_KEY, temporalDomainEnd);
  core.globalState.set(CRITICAL_PATH_START_KEY, criticalPathStart);
  core.globalState.set(CRITICAL_PATH_END_KEY, criticalPathEnd);
  core.globalState.set(CRITICAL_PATH_STRATEGY_KEY, criticalPathStrategy);
  core.globalState.set(DRAFT_CRITICAL_PATH_START_KEY, draftCriticalPathStart);
  core.globalState.set(DRAFT_CRITICAL_PATH_END_KEY, draftCriticalPathEnd);
  core.globalState.set(DRAFT_CRITICAL_PATH_STRATEGY_KEY, draftCriticalPathStrategy);
  core.globalState.set(SEARCH_KEY, search);
  core.globalState.set(DRAFT_SEARCH_KEY, draftSearch);
  core.globalState.set(TRANSFORM_TEMPLATE_KEY, transformTemplate);
  core.globalState.set(DRAFT_TRANSFORM_TEMPLATE_KEY, draftTransformTemplate);
  core.globalState.set(EXPAND_MATCHES_KEY, expandMatches);
  core.globalState.set(HIDE_NON_MATCHING_KEY, hideNonMatching);
  core.globalState.set(HIDE_EMPTY_KEY, hideEmpty);
  core.globalState.set(SHOW_ONLY_CRITICAL_PATH_KEY, showOnlyCriticalPath);
  core.globalState.set(THEME_KEY, theme);

  const dataQuery = core.addDataQuery();
  dataQuery.connect(new HttpDataFetcher(core));
  dataQuery.setGlobalFilters(
    new ValueMap(
      new Map<string, Value>([
        ['corpus_path', corpusPath],
        ['trace_id', traceID],
        [HIERARCHY_TYPE_KEY, hierarchyType],
        [EXPANDED_CATEGORY_IDS_KEY, expandedCategoryIDs],
        [FOCUS_SPAN_IDS_KEY, focusSpanIDs],
        [TEMPORAL_DOMAIN_START_KEY, temporalDomainStart],
        [TEMPORAL_DOMAIN_END_KEY, temporalDomainEnd],
        [CRITICAL_PATH_START_KEY, criticalPathStart],
        [CRITICAL_PATH_END_KEY, criticalPathEnd],
        [CRITICAL_PATH_STRATEGY_KEY, criticalPathStrategy],
        [DRAFT_CRITICAL_PATH_START_KEY, draftCriticalPathStart],
        [DRAFT_CRITICAL_PATH_END_KEY, draftCriticalPathEnd],
        [DRAFT_CRITICAL_PATH_STRATEGY_KEY, draftCriticalPathStrategy],
        [SEARCH_KEY, search],
        [DRAFT_SEARCH_KEY, draftSearch],
        [TRANSFORM_TEMPLATE_KEY, transformTemplate],
        [DRAFT_TRANSFORM_TEMPLATE_KEY, draftTransformTemplate],
        [EXPAND_MATCHES_KEY, expandMatches],
        [HIDE_NON_MATCHING_KEY, hideNonMatching],
        [HIDE_EMPTY_KEY, hideEmpty],
        [SHOW_ONLY_CRITICAL_PATH_KEY, showOnlyCriticalPath],
        [THEME_KEY, theme],
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
    hierarchyType,
    expandedCategoryIDs,
    focusSpanIDs,
    temporalDomainStart,
    temporalDomainEnd,
    criticalPathStart,
    criticalPathEnd,
    criticalPathStrategy,
    draftCriticalPathStart,
    draftCriticalPathEnd,
    draftCriticalPathStrategy,
    search,
    draftSearch,
    transformTemplate,
    draftTransformTemplate,
    expandMatches,
    hideNonMatching,
    hideEmpty,
    showOnlyCriticalPath,
    theme,
    searchValidationRequest,
    transformValidationRequest,
    criticalPathValidationRequest,
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

  // Maintains one TraceViz DataSeriesQuery for the provided query identity,
  // replacing it only when the query object, name, parameters, or fetch predicate changes.
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

  // Tracks the current DOM element's content width, rebuilding the observer when
  // the ref target changes.
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

  // Provides a stable React ref callback that publishes the latest measured element.
  const ref = useCallback(
    (node: HTMLDivElement | null): void => {
      setElement(node);
    },
    [],
  );

  return [ref, widthPx];
}

export default function App(): JSX.Element {
  // Creates the TraceViz AppCore, Values, and DataQuery exactly once for this app instance.
  const state = useMemo(() => createCausalTracingState(), []);
  const corpusPath = useValue(state.corpusPath, DEFAULT_TRACE_PATH) ?? '';
  const selectedTraceID = useValue(state.traceID, '') ?? '';
  const selectedHierarchyType =
    useValue(state.hierarchyType, DEFAULT_HIERARCHY_TYPE) ??
    DEFAULT_HIERARCHY_TYPE;
  const focusSpanIDs = useValue<string[]>(state.focusSpanIDs, []) ?? [];
  const focusSpanID = focusSpanIDs.length > 0 ? focusSpanIDs[0] : '';
  const focusSpanKey = focusSpanIDs.join('\x00');
  const criticalPathStrategy = useValue(
    state.criticalPathStrategy,
    DEFAULT_CRITICAL_PATH_STRATEGY,
  ) ?? DEFAULT_CRITICAL_PATH_STRATEGY;
  const draftCriticalPathStart = useValue(
    state.draftCriticalPathStart,
    DEFAULT_CRITICAL_PATH_START,
  ) ?? DEFAULT_CRITICAL_PATH_START;
  const draftCriticalPathEnd = useValue(
    state.draftCriticalPathEnd,
    DEFAULT_CRITICAL_PATH_END,
  ) ?? DEFAULT_CRITICAL_PATH_END;
  const draftCriticalPathStrategy = useValue(
    state.draftCriticalPathStrategy,
    DEFAULT_CRITICAL_PATH_STRATEGY,
  ) ?? DEFAULT_CRITICAL_PATH_STRATEGY;
  const draftSearch = useValue(state.draftSearch, '') ?? '';
  const committedSearch = useValue(state.search, '') ?? '';
  const transformTemplate = useValue(state.transformTemplate, '') ?? '';
  const draftTransformTemplate = useValue(
    state.draftTransformTemplate,
    '',
  ) ?? '';
  const expandMatches = useValue(state.expandMatches, 'false') === 'true';
  const hideNonMatching = useValue(state.hideNonMatching, 'false') === 'true';
  const hideEmpty = useValue(state.hideEmpty, 'false') === 'true';
  const showOnlyCriticalPath =
    useValue(state.showOnlyCriticalPath, 'false') === 'true';
  const theme = useValue(state.theme, DEFAULT_THEME) ?? DEFAULT_THEME;
  const [searchValidationError, setSearchValidationError] = useState<string>('');
  const [criticalPathValidationError, setCriticalPathValidationError] =
    useState<string>('');
  const [transformModalOpen, setTransformModalOpen] = useState<boolean>(false);
  const [transformValidationError, setTransformValidationError] =
    useState<string>('');
  const [focusStackOpen, setFocusStackOpen] = useState<boolean>(false);
  // Binds URL-hash state to the stable TraceViz Value objects that identify the
  // selected corpus, trace, hierarchy, and theme.
  useEffect(() => {
    const urlHash = new UrlHash({
      unencoded: new ValueMap(
        new Map<string, Value>([
          ['corpus_path', state.corpusPath],
          [TRACE_ID_KEY, state.traceID],
          [HIERARCHY_TYPE_KEY, state.hierarchyType],
          [THEME_KEY, state.theme],
        ]),
      ),
      stateful: [TRACE_ID_KEY, HIERARCHY_TYPE_KEY, THEME_KEY],
      onError: (err: unknown) => {
        state.core.err(
          err instanceof ConfigurationError
            ? err
            : new ConfigurationError(String(err))
                .from('causal-tracing.url-hash')
                .at(Severity.ERROR),
        );
      },
    });
    urlHash.start();
    return () => {
      urlHash.stop();
    };
  }, [
    state.core,
    state.corpusPath,
    state.hierarchyType,
    state.theme,
    state.traceID,
  ]);
  const [tracePanelRef, measuredTraceWidthPx] = useMeasuredWidth();
  // Publishes measured trace panel width into TraceViz state so backend render
  // queries can downsample to the actual viewport.
  useEffect(() => {
    if (measuredTraceWidthPx > 0) {
      state.traceViewWidthPx.val = measuredTraceWidthPx;
    }
  }, [measuredTraceWidthPx, state.traceViewWidthPx]);
  // Closes the focus-stack popover whenever focus mode exits.
  useEffect(() => {
    if (focusSpanIDs.length === 0) {
      setFocusStackOpen(false);
    }
  }, [focusSpanIDs.length]);
  // Reuses an empty parameter map for queries whose inputs are all global filters.
  const emptyParams = useMemo(() => new ValueMap(), []);
  // Supplies only the trace viewport width as a per-query parameter; trace identity
  // and render policy stay in global filters.  The focus stack remains a global
  // filter, but it is included in this memo's dependency list so focus-mode
  // table interactions rebuild this query subscription and cannot leave the
  // timeline rendering on the previous stack.
  const traceParams = useMemo(
    () =>
      new ValueMap(
        new Map<string, Value>([
          [TRACE_VIEW_WIDTH_PX_PARAM, state.traceViewWidthPx],
        ]),
      ),
    [focusSpanKey, state.traceViewWidthPx],
  );
  const globalRef = (key: string): GlobalValueRef =>
    new GlobalValueRef(state.core, key);
  // Refetches corpus-level queries when the corpus path Value publishes a change.
  const corpusChanged = useMemo(
    () => new Changed([globalRef('corpus_path')]),
    [state.core],
  );
  // Refetches trace metadata queries when the selected corpus, trace, or transform changes.
  const traceChanged = useMemo(
    () =>
      new Changed([
        globalRef('corpus_path'),
        globalRef('trace_id'),
        globalRef(TRANSFORM_TEMPLATE_KEY),
      ]),
    [state.core],
  );
  // Refetches rendered trace views when trace identity, render policy, viewport
  // domain, critical path policy, search/filter state, or measured width changes.
  const traceRenderChanged = useMemo(
    () =>
      new Changed([
        globalRef('corpus_path'),
        globalRef('trace_id'),
        globalRef(HIERARCHY_TYPE_KEY),
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
        globalRef(HIDE_NON_MATCHING_KEY),
        globalRef(HIDE_EMPTY_KEY),
        globalRef(SHOW_ONLY_CRITICAL_PATH_KEY),
        globalRef(THEME_KEY),
        new DirectValueRef(state.traceViewWidthPx, 'trace view width'),
      ]),
    [state.core, state.traceViewWidthPx],
  );
  // Refetches the span focus table when the focused stack or selected trace context changes.
  const spanFocusChanged = useMemo(
    () =>
      new Changed([
        globalRef('corpus_path'),
        globalRef('trace_id'),
        globalRef(TRANSFORM_TEMPLATE_KEY),
        globalRef(HIERARCHY_TYPE_KEY),
        globalRef(FOCUS_SPAN_IDS_KEY),
      ]),
    [state.core],
  );
  // Refetches search validation only when the explicit validation counter changes.
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
  // Refetches transform validation only when the explicit validation counter changes.
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
  // Refetches critical path validation only when the explicit validation counter changes.
  const criticalPathValidationChanged = useMemo(
    () =>
      new Changed([
        new DirectValueRef(
          state.criticalPathValidationRequest,
          'critical path validation request',
        ),
      ]),
    [state.criticalPathValidationRequest],
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
  const hierarchyTypes = useDataSeries(
    state.dataQuery,
    HIERARCHY_TYPES_QUERY,
    emptyParams,
    traceChanged,
  );
  const criticalPathStrategies = useDataSeries(
    state.dataQuery,
    CRITICAL_PATH_STRATEGIES_QUERY,
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
  const criticalPathValidation = useDataSeries(
    state.dataQuery,
    VALIDATE_CRITICAL_PATH_QUERY,
    emptyParams,
    criticalPathValidationChanged,
  );
  // Commits a validated search draft, or reports validation failure without changing
  // the active search.
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
  // Commits validated critical path endpoints and strategy, or keeps the previous
  // critical path policy while surfacing validation failure.
  useEffect(() => {
    const data = criticalPathValidation.data;
    if (!data) {
      return;
    }
    try {
      const validatedStart = data.properties.expectString(
        DRAFT_CRITICAL_PATH_START_KEY,
      );
      const validatedEnd = data.properties.expectString(
        DRAFT_CRITICAL_PATH_END_KEY,
      );
      const validatedStrategy = data.properties.expectString(
        DRAFT_CRITICAL_PATH_STRATEGY_KEY,
      );
      if (
        validatedStart !== state.draftCriticalPathStart.val ||
        validatedEnd !== state.draftCriticalPathEnd.val ||
        validatedStrategy !== state.draftCriticalPathStrategy.val
      ) {
        return;
      }
      const status = data.properties.expectString(STATUS_KEY);
      const message = data.properties.has(MESSAGE_KEY)
        ? data.properties.expectString(MESSAGE_KEY)
        : '';
      if (status === 'ok') {
        setCriticalPathValidationError('');
        state.criticalPathStart.val = validatedStart;
        state.criticalPathEnd.val = validatedEnd;
        state.criticalPathStrategy.val = validatedStrategy;
        return;
      }
      setCriticalPathValidationError(message || 'Invalid critical path');
    } catch (err: unknown) {
      state.core.err(
        err instanceof ConfigurationError
          ? err
          : new ConfigurationError(String(err))
              .from('causal-tracing.critical-path-validation')
              .at(Severity.ERROR),
      );
    }
  }, [
    criticalPathValidation.data,
    state.core,
    state.criticalPathEnd,
    state.criticalPathStart,
    state.criticalPathStrategy,
    state.draftCriticalPathEnd,
    state.draftCriticalPathStart,
    state.draftCriticalPathStrategy,
  ]);
  // Commits a validated transform template as a new trace identity, resetting view
  // state that depends on the concrete rendered trace.
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
  // Parses the backend trace response into a TraceViz Trace for the main overtime view.
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
  // Parses available hierarchy types for the display-options dropdown.
  const hierarchyOptions = useMemo(
    () => parseHierarchyOptions(hierarchyTypes.data, state.core),
    [hierarchyTypes.data, state.core],
  );
  // Parses available critical path strategies, falling back to known descriptions
  // until the backend query has responded.
  const criticalPathStrategyOptions = useMemo(() => {
    const options = parseCriticalPathStrategyOptions(
      criticalPathStrategies.data,
      state.core,
    );
    if (options.length === 0) {
      return FALLBACK_CRITICAL_PATH_STRATEGY_OPTIONS;
    }
    return options;
  }, [criticalPathStrategies.data, state.core]);
  // Resolves the active critical path strategy value to the label shown in the group header.
  const criticalPathStrategyLabel = useMemo(() => {
    const option = criticalPathStrategyOptions.find(
      (candidate) => candidate.value === criticalPathStrategy,
    );
    return option?.label ?? criticalPathStrategy;
  }, [criticalPathStrategy, criticalPathStrategyOptions]);
  // Parses the backend trace response into a TraceViz Trace for the overtime
  // critical path stack view.
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
  // Defines corpus table row behavior: clicking selects a trace and resets
  // trace-specific focus and zoom state.
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
  // Defines main trace interactions: span clicks push focus, brushes/reset gestures
  // update zoom, and category callouts are watched for hover highlighting.
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
  // Maps keypress state to temporal zoom and pan operations over the rendered trace domain.
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
  // Defines category-axis interactions independently from span interactions: hover
  // calls out a subtree, click toggles expansion, and mouseout clears the callout.
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
  // Defines focus-table navigation: clicking a causal "other span" pushes it onto
  // the focused span stack.
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
    state.criticalPathStart.val = DEFAULT_CRITICAL_PATH_START;
    state.criticalPathEnd.val = DEFAULT_CRITICAL_PATH_END;
    state.criticalPathStrategy.val = DEFAULT_CRITICAL_PATH_STRATEGY;
    state.draftCriticalPathStart.val = DEFAULT_CRITICAL_PATH_START;
    state.draftCriticalPathEnd.val = DEFAULT_CRITICAL_PATH_END;
    state.draftCriticalPathStrategy.val = DEFAULT_CRITICAL_PATH_STRATEGY;
    state.transformTemplate.val = '';
    state.draftTransformTemplate.val = '';
    state.expandMatches.val = 'false';
    state.hideNonMatching.val = 'false';
    state.hideEmpty.val = 'false';
    state.showOnlyCriticalPath.val = 'false';
    setSearchValidationError('');
    setCriticalPathValidationError('');
    setTransformValidationError('');
    setTransformModalOpen(false);
    resetTemporalDomain(state.temporalDomainStart, state.temporalDomainEnd);
    state.traceID.val = '';
  };

  const setCorpusPath = (nextCorpusPath: string): void => {
    state.focusSpanIDs.val = [];
    state.search.val = '';
    state.draftSearch.val = '';
    state.criticalPathStart.val = DEFAULT_CRITICAL_PATH_START;
    state.criticalPathEnd.val = DEFAULT_CRITICAL_PATH_END;
    state.criticalPathStrategy.val = DEFAULT_CRITICAL_PATH_STRATEGY;
    state.draftCriticalPathStart.val = DEFAULT_CRITICAL_PATH_START;
    state.draftCriticalPathEnd.val = DEFAULT_CRITICAL_PATH_END;
    state.draftCriticalPathStrategy.val = DEFAULT_CRITICAL_PATH_STRATEGY;
    state.transformTemplate.val = '';
    state.draftTransformTemplate.val = '';
    state.expandMatches.val = 'false';
    state.hideNonMatching.val = 'false';
    state.hideEmpty.val = 'false';
    state.showOnlyCriticalPath.val = 'false';
    setSearchValidationError('');
    setCriticalPathValidationError('');
    setTransformValidationError('');
    setTransformModalOpen(false);
    resetTemporalDomain(state.temporalDomainStart, state.temporalDomainEnd);
    state.traceID.val = '';
    state.corpusPath.val = nextCorpusPath;
  };

  const popFocusSpan = (): void => {
    if (focusSpanIDs.length <= 1) {
      setFocusStackOpen(false);
    }
    state.focusSpanIDs.val = focusSpanIDs.slice(1);
  };

  const clearFocusSpans = (): void => {
    setFocusStackOpen(false);
    state.focusSpanIDs.val = [];
  };

  const resetZoom = (): void => {
    resetTemporalDomain(state.temporalDomainStart, state.temporalDomainEnd);
  };

  const toggleTheme = (): void => {
    state.theme.val = theme === 'dark' ? 'light' : 'dark';
  };

  const applySearch = (): void => {
    state.searchValidationRequest.val = state.searchValidationRequest.val + 1;
  };

  const clearSearch = (): void => {
    state.draftSearch.val = '';
    state.search.val = '';
    setSearchValidationError('');
  };

  const recomputeCriticalPath = (): void => {
    state.criticalPathValidationRequest.val =
      state.criticalPathValidationRequest.val + 1;
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
    : traceStatus.loading || hierarchyTypes.loading || traceDiagnostics.loading || renderedTraceResult.loading || renderedCriticalPathResult.loading || spanCausality.loading || searchValidation.loading || criticalPathValidation.loading || transformValidation.loading;

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
      {focusSpanIDs.length === 0 ? (
        <div className="control-grid" aria-label="Trace controls">
          <section className="control-group" aria-label="Search within this trace">
            <div className="control-group-heading">
              <h2>Search within this trace</h2>
            </div>
            <div className="control-row">
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
            </div>
          </section>
          <section className="control-group" aria-label="Critical path controls">
            <div className="control-group-heading">
              <h2>Critical path</h2>
              {criticalPathValidationError !== '' ? (
                <span className="field-error">{criticalPathValidationError}</span>
              ) : (
                <span className="status-badge">{criticalPathStrategyLabel}</span>
              )}
            </div>
            <div className="critical-path-controls">
              <label
                className={
                  criticalPathValidationError === ''
                    ? 'field critical-path-endpoint'
                    : 'field critical-path-endpoint invalid'
                }
              >
                <span>CP start</span>
                <input
                  value={draftCriticalPathStart}
                  placeholder="trace default"
                  onChange={(event) => {
                    state.draftCriticalPathStart.val = event.target.value;
                    setCriticalPathValidationError('');
                  }}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault();
                      recomputeCriticalPath();
                    }
                  }}
                />
              </label>
              <label
                className={
                  criticalPathValidationError === ''
                    ? 'field critical-path-endpoint'
                    : 'field critical-path-endpoint invalid'
                }
              >
                <span>CP end</span>
                <input
                  value={draftCriticalPathEnd}
                  placeholder="trace default"
                  onChange={(event) => {
                    state.draftCriticalPathEnd.val = event.target.value;
                    setCriticalPathValidationError('');
                  }}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault();
                      recomputeCriticalPath();
                    }
                  }}
                />
              </label>
              <label className="field critical-path-strategy">
                <span>Pathing</span>
                <select
                  value={draftCriticalPathStrategy}
                  disabled={criticalPathStrategies.loading}
                  onChange={(event) => {
                    state.draftCriticalPathStrategy.val = event.target.value;
                    setCriticalPathValidationError('');
                  }}
                >
                  {criticalPathStrategyOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
              <button
                className="plain-button"
                type="button"
                disabled={criticalPathValidation.loading}
                onClick={recomputeCriticalPath}
              >
                {criticalPathValidation.loading ? 'Validating' : 'Recompute'}
              </button>
            </div>
          </section>
          <section className="control-group visibility-group" aria-label="Display controls">
            <div className="control-group-heading">
              <h2>Display options</h2>
            </div>
            <div className="toggle-row">
              <label className="field compact hierarchy-field">
                <span>Hierarchy</span>
                <select
                  value={selectedHierarchyType}
                  disabled={hierarchyTypes.loading || hierarchyOptions.length === 0}
                  onChange={(event) => {
                    state.focusSpanIDs.val = [];
                    state.calledOutCategoryID.val = '';
                    state.search.val = '';
                    state.draftSearch.val = '';
                    state.criticalPathStart.val = DEFAULT_CRITICAL_PATH_START;
                    state.criticalPathEnd.val = DEFAULT_CRITICAL_PATH_END;
                    state.draftCriticalPathStart.val = DEFAULT_CRITICAL_PATH_START;
                    state.draftCriticalPathEnd.val = DEFAULT_CRITICAL_PATH_END;
                    setSearchValidationError('');
                    setCriticalPathValidationError('');
                    state.hierarchyType.val = event.target.value;
                  }}
                >
                  {hierarchyOptions.length === 0 ? (
                    <option value={selectedHierarchyType}>
                      {selectedHierarchyType}
                    </option>
                  ) : (
                    hierarchyOptions.map((option) => (
                      <option
                        key={option.value}
                        value={option.value}
                        title={option.description}
                      >
                        {option.label}
                      </option>
                    ))
                  )}
                </select>
              </label>
              <button
                className={expandMatches ? 'toggle-button active' : 'toggle-button'}
                type="button"
                aria-pressed={expandMatches}
                onClick={() => {
                  state.expandMatches.val = expandMatches ? 'false' : 'true';
                }}
              >
                {expandMatches ? 'Matches autoexpanded' : 'Matches not autoexpanded'}
              </button>
              <button
                className={hideNonMatching ? 'toggle-button active' : 'toggle-button'}
                type="button"
                aria-pressed={hideNonMatching}
                disabled={committedSearch === ''}
                onClick={() => {
                  state.hideNonMatching.val = hideNonMatching ? 'false' : 'true';
                }}
              >
                {hideNonMatching ? 'Hiding non-matches' : 'Showing non-matches'}
              </button>
              <button
                className={hideEmpty ? 'toggle-button active' : 'toggle-button'}
                type="button"
                aria-pressed={hideEmpty}
                onClick={() => {
                  state.hideEmpty.val = hideEmpty ? 'false' : 'true';
                }}
              >
                {hideEmpty ? 'Hiding empty categories' : 'Showing empty categories'}
              </button>
              <button
                className={
                  showOnlyCriticalPath ? 'toggle-button active' : 'toggle-button'
                }
                type="button"
                aria-pressed={showOnlyCriticalPath}
                onClick={() => {
                  state.showOnlyCriticalPath.val = showOnlyCriticalPath
                    ? 'false'
                    : 'true';
                }}
              >
                {showOnlyCriticalPath
                  ? 'Showing only critical path'
                  : 'Showing off-critical-path work'}
              </button>
            </div>
          </section>
        </div>
      ) : null}
      <section
        className="trace-panel"
        ref={tracePanelRef}
        aria-label="Overtime trace"
      >
        <div className="panel-heading">
          <h2>Overtime trace</h2>
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
        {focusSpanIDs.length > 0 ? (
          <div className="focus-bar" aria-label="Focused span stack controls">
            <div className="focus-stack-menu">
              <button
                className="focus-head"
                type="button"
                aria-expanded={focusStackOpen}
                aria-controls="focus-stack-list"
                onClick={() => {
                  setFocusStackOpen((open) => !open);
                }}
              >
                Head: {focusSpanID}
              </button>
              {focusStackOpen ? (
                <div
                  className="focus-stack-popover"
                  id="focus-stack-list"
                  role="list"
                >
                  {focusSpanIDs.map((spanID, index) => (
                    <div
                      className="focus-stack-row"
                      key={`${spanID}:${index}`}
                      role="listitem"
                    >
                      <span className="focus-stack-index">
                        {index === 0 ? 'Head' : `#${index + 1}`}
                      </span>
                      <span className="focus-stack-id">{spanID}</span>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
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
            <h2>Overtime critical path</h2>
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
      <section className="table-panel trace-details-panel" aria-label="Selected trace details">
        <div className="panel-heading">
          <h2>Trace details</h2>
          <span
            className={
              traceStatus.loading ? 'status-badge busy' : 'status-badge'
            }
          >
            {traceStatus.loading ? 'Loading' : 'Ready'}
          </span>
        </div>
        <DataTable
          data={traceStatus.data}
          loading={traceStatus.loading}
          withPagination={false}
          scrollable={false}
        />
      </section>
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
      <div className="app-root" data-theme={theme === 'dark' ? 'dark' : 'light'}>
        <KeypressListener interactions={keypressInteractions} />
        <main className="app-shell">
          <section className="control-group trace-selection-group" aria-label="Trace selection">
            <div className="control-group-heading">
              <h2>Trace selection</h2>
              <button
                className="toggle-button theme-toggle"
                type="button"
                aria-pressed={theme === 'dark'}
                onClick={toggleTheme}
              >
                {theme === 'dark' ? 'Light mode' : 'Dark mode'}
              </button>
            </div>
            <div className="toolbar">
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
                    onClick={clearSelectedTrace}
                  >
                    Back to corpus
                  </button>
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
            </div>
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
      </div>
    </AppCoreContext.Provider>
  );
}
