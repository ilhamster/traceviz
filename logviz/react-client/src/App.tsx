import { useEffect, useMemo, useState } from "react";
import {
  Action,
  And,
  AppCore,
  Case,
  Changed,
  Clear,
  DataQuery,
  DataSeriesQuery,
  Do,
  Equals,
  FixedValue,
  HttpDataFetcher,
  IntegerValue,
  Includes,
  Interactions,
  Keypress,
  LocalValue,
  Not,
  Or,
  Predicate,
  Reaction,
  Set as SetAction,
  SetOrClear,
  StringSetValue,
  StringValue,
  Switch,
  Timestamp,
  TimestampValue,
  Toggle,
  True,
  Value,
  ValueMap,
  Watch,
  type KeyedValueRef,
  type ResponseNode,
} from "@traceviz/client-core";
import {
  AppCoreContext,
  DataTable,
  ErrorToast,
  GlobalStateMonitor,
  LineChart,
  useValue,
} from "@traceviz/client-react";
import { type Subscription } from "rxjs";

const EMPTY_TIMESTAMP = new Timestamp(0, 0);

const SOURCE_FILE_KEY = "source_file";
const TIMESTAMP_KEY = "timestamp";

const QUERY_SOURCE_FILES = "logs.aggregate_source_files_table";
const QUERY_RAW_ENTRIES = "logs.raw_entries";
const QUERY_TIMESERIES = "logs.timeseries";
const QUERY_PAN_AND_ZOOM = "logs.pan_and_zoom";

type LogvizValues = {
  collectionName: StringValue;
  filteredSourceFiles: StringSetValue;
  filteredTimerangeStart: TimestampValue;
  filteredTimerangeEnd: TimestampValue;
  calledOutSourceFiles: StringSetValue;
  calledOutTimestamp: TimestampValue;
  sourceFileSearchRegex: StringValue;
  rawEventSearchRegex: StringValue;
  depressedKeyCodes: StringSetValue;
  appTheme: StringValue;
  pan: StringValue;
  zoom: StringValue;
};

type LogvizState = {
  core: AppCore;
  dataQuery: DataQuery;
  values: LogvizValues;
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

function createLogvizState(): LogvizState {
  const values: LogvizValues = {
    collectionName: new StringValue("logs/cockroachdb.log"),
    filteredSourceFiles: new StringSetValue(new globalThis.Set<string>()),
    filteredTimerangeStart: new TimestampValue(EMPTY_TIMESTAMP),
    filteredTimerangeEnd: new TimestampValue(EMPTY_TIMESTAMP),
    calledOutSourceFiles: new StringSetValue(new globalThis.Set<string>()),
    calledOutTimestamp: new TimestampValue(EMPTY_TIMESTAMP),
    sourceFileSearchRegex: new StringValue(""),
    rawEventSearchRegex: new StringValue(""),
    depressedKeyCodes: new StringSetValue(new Set([])),
    appTheme: new StringValue("light"),
    pan: new StringValue(""),
    zoom: new StringValue(""),
  };

  const core = new AppCore();
  core.globalState.set("collection_name", values.collectionName);
  core.globalState.set("filtered_source_files", values.filteredSourceFiles);
  core.globalState.set(
    "filtered_timerange_start",
    values.filteredTimerangeStart
  );
  core.globalState.set("filtered_timerange_end", values.filteredTimerangeEnd);
  core.globalState.set("called_out_source_files", values.calledOutSourceFiles);
  core.globalState.set("called_out_timestamp", values.calledOutTimestamp);
  core.globalState.set(
    "source_file_search_regex",
    values.sourceFileSearchRegex
  );
  core.globalState.set("raw_event_search_regex", values.rawEventSearchRegex);
  core.globalState.set("depressed_key_codes", values.depressedKeyCodes);
  core.globalState.set("app_theme", values.appTheme);
  core.globalState.set("pan", values.pan);
  core.globalState.set("zoom", values.zoom);

  const dataQuery = core.addDataQuery();
  dataQuery.connect(new HttpDataFetcher(core));
  dataQuery.setGlobalFilters(
    new ValueMap(
      new Map<string, Value>([
        ["collection_name", values.collectionName],
        ["filtered_source_files", values.filteredSourceFiles],
        ["start_timestamp", values.filteredTimerangeStart],
        ["end_timestamp", values.filteredTimerangeEnd],
        ["app_theme", values.appTheme],
      ])
    )
  );
  dataQuery.debounceUpdates(50);

  core.publish();

  return { core, dataQuery, values };
}

type SeriesResult = {
  data?: ResponseNode;
  loading: boolean;
};

function shouldIgnoreKeypress(event: KeyboardEvent): boolean {
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  if (target.isContentEditable) {
    return true;
  }
  const tag = target.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}

// Create a DataSeries with the specified name and parameters
function useDataSeries(
  dataQuery: DataQuery,
  queryName: string,
  parameters: ValueMap,
  fetch: Predicate
): SeriesResult {
  const [data, setData] = useState<ResponseNode | undefined>(undefined);
  const [loading, setLoading] = useState<boolean>(false);

  useEffect(() => {
    const seriesQuery = new DataSeriesQuery(
      dataQuery,
      new StringValue(queryName),
      parameters,
      fetch.match()()
    );
    const subscriptions: Subscription[] = [];
    subscriptions.push(seriesQuery.response.subscribe(setData));
    subscriptions.push(seriesQuery.loading.subscribe(setLoading));
    return () => {
      subscriptions.forEach((sub) => sub.unsubscribe());
      seriesQuery.dispose();
    };
  }, [dataQuery, fetch, parameters, queryName]);

  return { data, loading };
}

export default function App(): JSX.Element {
  const logviz = useMemo(() => createLogvizState(), []);

  const sourceSearch = useValue(logviz.values.sourceFileSearchRegex, "") ?? "";
  const rawSearch = useValue(logviz.values.rawEventSearchRegex, "") ?? "";
  const appThemeRaw = useValue(logviz.values.appTheme, "light") ?? "light";
  const appTheme = appThemeRaw === "dark" ? "dark" : "light";

  const globalRef = (key: string): GlobalValueRef =>
    new GlobalValueRef(logviz.core, key);
  const fixed = (value: string): FixedValue =>
    new FixedValue(new StringValue(value), value);

  // Define 'Source files' table parameters, fetch predicate, data series query,
  // and interactions.
  const sourceTableParams = useMemo(
    () =>
      new ValueMap(
        new Map([["search_regex", logviz.values.sourceFileSearchRegex]])
      ),
    [logviz.values.sourceFileSearchRegex]
  );
  const sourceTableFetch = useMemo(
    () =>
      new And([
        new Not(new Equals(globalRef("collection_name"), fixed(""))),
        new Or([
          new Changed([
            globalRef("collection_name"),
            globalRef("source_file_search_regex"),
            globalRef("filtered_timerange_start"),
            globalRef("filtered_timerange_end"),
          ]),
        ]),
      ]),
    [logviz.core]
  );
  const sourceTableDataSeries = useDataSeries(
    logviz.dataQuery,
    QUERY_SOURCE_FILES,
    sourceTableParams,
    sourceTableFetch
  );
  const sourceTableInteractions = useMemo(() => {
    const sourceFile = new LocalValue(SOURCE_FILE_KEY);
    return new Interactions()
      .withAction(
        new Action("rows", "mouseover", [
          new SetAction(globalRef("called_out_source_files"), sourceFile),
        ])
      )
      .withAction(
        new Action("rows", "mouseout", [
          new Clear([globalRef("called_out_source_files")]),
        ])
      )
      .withAction(
        new Action("rows", "click", [
          new SetOrClear(globalRef("filtered_source_files"), sourceFile),
        ])
      )
      .withAction(
        new Action("rows", "shift-click", [
          new Toggle(globalRef("filtered_source_files"), sourceFile),
        ])
      )
      .withReaction(
        new Reaction(
          "rows",
          "highlight",
          new Includes(globalRef("called_out_source_files"), sourceFile)
        )
      );
  }, [logviz.core]);

  // Define 'Raw log entries' table parameters, fetch predicate, data series
  // query, and interactions.
  const rawEntryParams = useMemo(
    () =>
      new ValueMap(
        new Map([["search_regex", logviz.values.rawEventSearchRegex]])
      ),
    [logviz.values.rawEventSearchRegex]
  );
  const rawEntryFetch = useMemo(
    () =>
      new And([
        new Not(new Equals(globalRef("collection_name"), fixed(""))),
        new Or([
          new Changed([
            globalRef("collection_name"),
            globalRef("raw_event_search_regex"),
            globalRef("filtered_source_files"),
            globalRef("filtered_timerange_start"),
            globalRef("filtered_timerange_end"),
          ]),
        ]),
      ]),
    [logviz.core]
  );
  const rawEntryTableDataSeries = useDataSeries(
    logviz.dataQuery,
    QUERY_RAW_ENTRIES,
    rawEntryParams,
    rawEntryFetch
  );
  const rawEntryInteractions = useMemo(() => {
    const sourceFile = new LocalValue(SOURCE_FILE_KEY);
    const timestamp = new LocalValue(TIMESTAMP_KEY);
    return new Interactions()
      .withAction(
        new Action("rows", "mouseover", [
          new SetAction(globalRef("called_out_timestamp"), timestamp),
        ])
      )
      .withAction(
        new Action("rows", "mouseout", [
          new Clear([globalRef("called_out_timestamp")]),
        ])
      )
      .withReaction(
        new Reaction(
          "rows",
          "highlight",
          new Includes(globalRef("called_out_source_files"), sourceFile)
        )
      );
  }, [logviz.core]);

  // Define 'Timeseries' graph parameters, fetch predicate, data series query,
  // and interactions.
  const timeseriesParams = useMemo(
    () =>
      new ValueMap(
        new Map<string, Value>([
          ["aggregate_by", new StringValue("level_name")],
          ["bin_count", new IntegerValue(1000)],
        ])
      ),
    []
  );
  const timeseriesFetch = useMemo(
    () =>
      new And([
        new Not(new Equals(globalRef("collection_name"), fixed(""))),
        new Or([
          new Changed([
            globalRef("collection_name"),
            globalRef("filtered_source_files"),
            globalRef("filtered_timerange_start"),
            globalRef("filtered_timerange_end"),
          ]),
        ]),
      ]),
    [logviz.core]
  );
  const timeseriesDataSeries = useDataSeries(
    logviz.dataQuery,
    QUERY_TIMESERIES,
    timeseriesParams,
    timeseriesFetch
  );
  const timeseriesInteractions = useMemo(() => {
    const zoomStart = new LocalValue("zoom_start");
    const zoomEnd = new LocalValue("zoom_end");
    return new Interactions()
      .withAction(
        new Action("chart", "brush", [
          new SetAction(globalRef("filtered_timerange_start"), zoomStart),
          new SetAction(globalRef("filtered_timerange_end"), zoomEnd),
        ])
      )
      .withWatch(
        new Watch(
          "update_x_axis_marker",
          new ValueMap(
            new Map<string, Value>([
              ["x_axis_marker_position", logviz.values.calledOutTimestamp],
            ])
          )
        )
      );
  }, [logviz.core, logviz.values.calledOutTimestamp]);

  // Send pan and zoom requests to the backend, which will update and return
  // the filtered time range.
  const panAndZoomFetch = useMemo(
    () =>
      new Or([
        new Not(new Equals(globalRef("pan"), fixed(""))),
        new Not(new Equals(globalRef("zoom"), fixed(""))),
      ]),
    [logviz.core]
  );
  const panAndZoomDataSeries = useDataSeries(
    logviz.dataQuery,
    QUERY_PAN_AND_ZOOM,
    new ValueMap(),
    panAndZoomFetch
  );
  const panAndZoomUpdate = useMemo(() => {
    const zoomedStart = new LocalValue("start_timestamp");
    const zoomedEnd = new LocalValue("end_timestamp");
    return new Do([
      new SetAction(globalRef("filtered_timerange_start"), zoomedStart),
      new SetAction(globalRef("filtered_timerange_end"), zoomedEnd),
    ]);
  }, [logviz.core]);
  useEffect(() => {
    if (panAndZoomDataSeries.data) {
      panAndZoomUpdate.update(panAndZoomDataSeries.data?.properties);
    }
  }, [panAndZoomDataSeries.data]);

  // Handle panning and zooming by WASD keypress.
  useEffect(() => {
    const keypress = new Keypress(logviz.values.depressedKeyCodes);
    const depressedKeyCodes = globalRef("depressed_key_codes");

    const keypressUpdate = new Switch([
      new Case(new Includes(depressedKeyCodes, fixed("KeyW")), [
        new SetAction(globalRef("zoom"), fixed("in")),
        new SetAction(globalRef("pan"), fixed("")),
      ]),
      new Case(new Includes(depressedKeyCodes, fixed("KeyS")), [
        new SetAction(globalRef("zoom"), fixed("out")),
        new SetAction(globalRef("pan"), fixed("")),
      ]),
      new Case(new Includes(depressedKeyCodes, fixed("KeyA")), [
        new SetAction(globalRef("pan"), fixed("left")),
        new SetAction(globalRef("zoom"), fixed("")),
      ]),
      new Case(new Includes(depressedKeyCodes, fixed("KeyD")), [
        new SetAction(globalRef("pan"), fixed("right")),
        new SetAction(globalRef("zoom"), fixed("")),
      ]),
      new Case(new True(), [
        new SetAction(globalRef("pan"), fixed("")),
        new SetAction(globalRef("zoom"), fixed("")),
      ]),
    ]);

    const sub = logviz.values.depressedKeyCodes.subscribe(() => {
      keypressUpdate.update();
    });

    const onKeyDown = (event: KeyboardEvent): void => {
      if (shouldIgnoreKeypress(event)) {
        return;
      }
      keypress.keyEvent(event);
    };
    const onKeyUp = (event: KeyboardEvent): void => {
      if (shouldIgnoreKeypress(event)) {
        return;
      }
      keypress.keyEvent(event);
    };
    window.addEventListener("keydown", onKeyDown);
    window.addEventListener("keyup", onKeyUp);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("keyup", onKeyUp);
      sub.unsubscribe();
    };
  }, [logviz.values.depressedKeyCodes]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", appTheme);
  }, [appTheme]);

  return (
    <AppCoreContext.Provider value={logviz.core}>
      <ErrorToast />
      <div className="logviz">
        <header className="topbar">
          <h1>LogViz React</h1>
          <button
            type="button"
            className="theme-toggle"
            onClick={() => {
              logviz.values.appTheme.val =
                appTheme === "dark" ? "light" : "dark";
            }}
            aria-label={
              appTheme === "dark"
                ? "Switch to light theme"
                : "Switch to dark theme"
            }
            title={
              appTheme === "dark"
                ? "Switch to light theme"
                : "Switch to dark theme"
            }
          >
            {appTheme === "dark" ? (
              <svg
                viewBox="0 0 24 24"
                width="14"
                height="14"
                aria-hidden="true"
              >
                <circle cx="12" cy="12" r="4.2" fill="currentColor" />
                <line
                  x1="12"
                  y1="1.8"
                  x2="12"
                  y2="5"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                />
                <line
                  x1="12"
                  y1="19"
                  x2="12"
                  y2="22.2"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                />
                <line
                  x1="1.8"
                  y1="12"
                  x2="5"
                  y2="12"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                />
                <line
                  x1="19"
                  y1="12"
                  x2="22.2"
                  y2="12"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                />
                <line
                  x1="4.2"
                  y1="4.2"
                  x2="6.5"
                  y2="6.5"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                />
                <line
                  x1="17.5"
                  y1="17.5"
                  x2="19.8"
                  y2="19.8"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                />
                <line
                  x1="17.5"
                  y1="6.5"
                  x2="19.8"
                  y2="4.2"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                />
                <line
                  x1="4.2"
                  y1="19.8"
                  x2="6.5"
                  y2="17.5"
                  stroke="currentColor"
                  strokeWidth="1.8"
                  strokeLinecap="round"
                />
              </svg>
            ) : (
              <svg
                viewBox="0 0 24 24"
                width="14"
                height="14"
                aria-hidden="true"
              >
                <path
                  d="M16.4 3.2a8.8 8.8 0 1 0 4.4 15.8A9.5 9.5 0 1 1 16.4 3.2Z"
                  fill="currentColor"
                />
              </svg>
            )}
          </button>
        </header>

        <section className="cards">
          <article className="card">
            <div className="card-header">
              <div>
                <h2>Log source file summary</h2>
                <p>Hover to call out, click to filter.</p>
              </div>
              <input
                className="search"
                placeholder="Search for source files by regex"
                value={sourceSearch}
                onChange={(event) => {
                  logviz.values.sourceFileSearchRegex.val = event.target.value;
                }}
              />
            </div>
            <DataTable
              data={sourceTableDataSeries.data}
              loading={sourceTableDataSeries.loading}
              interactions={sourceTableInteractions}
              className="table-wrapper"
              withPagination
              scrollable={false}
            />
          </article>

          <article className="card">
            <div className="card-header">
              <div>
                <h2>Raw events</h2>
                <p>Hover to call out timestamps.</p>
              </div>
              <input
                className="search"
                placeholder="Search for event text by regex"
                value={rawSearch}
                onChange={(event) => {
                  logviz.values.rawEventSearchRegex.val = event.target.value;
                }}
              />
            </div>
            <DataTable
              data={rawEntryTableDataSeries.data}
              loading={rawEntryTableDataSeries.loading}
              interactions={rawEntryInteractions}
              className="table-wrapper raw-events"
              withPagination
              scrollable={false}
              rowHeightPxOverride={18}
              fontSizePxOverride={12}
            />
          </article>
        </section>

        <section className="chart-band">
          <div className="chart-header">
            <h2>Log messages over time</h2>
          </div>
          <LineChart
            data={timeseriesDataSeries.data}
            loading={timeseriesDataSeries.loading}
            interactions={timeseriesInteractions}
            className="chart-wrapper"
          />
        </section>

        <section className="state-band">
          <div className="state-header">
            <h2>Global State Monitor</h2>
            <p>Live view of TraceViz global state values.</p>
          </div>
          <div className="state-content">
            <GlobalStateMonitor columns={2} />
          </div>
        </section>
      </div>
    </AppCoreContext.Provider>
  );
}
