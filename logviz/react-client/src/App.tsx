import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Action,
  AppCore,
  Clear,
  DataQuery,
  DataSeriesQuery,
  HttpDataFetcher,
  Includes,
  Interactions,
  LocalValue,
  Reaction,
  Set as SetAction,
  SetOrClear,
  StringSetValue,
  StringValue,
  Timestamp,
  TimestampValue,
  Toggle,
  Value,
  ValueMap,
  type KeyedValueRef,
  type ResponseNode,
} from "@traceviz/client-core";
import {
  AppCoreContext,
  DataTable,
  ErrorToast,
  GlobalStateMonitor,
  useValue,
} from "@traceviz/client-react";
import { BehaviorSubject, type Subscription } from "rxjs";

const EMPTY_TIMESTAMP = new Timestamp(0, 0);

const SOURCE_FILE_KEY = "source_file";
const TIMESTAMP_KEY = "timestamp";

const QUERY_SOURCE_FILES = "logs.aggregate_source_files_table";
const QUERY_RAW_ENTRIES = "logs.raw_entries";

type LogvizValues = {
  collectionName: StringValue;
  filteredSourceFiles: StringSetValue;
  filteredTimerangeStart: TimestampValue;
  filteredTimerangeEnd: TimestampValue;
  calledOutSourceFiles: StringSetValue;
  calledOutTimestamp: TimestampValue;
  sourceFileSearchRegex: StringValue;
  rawEventSearchRegex: StringValue;
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
    pan: new StringValue("none"),
    zoom: new StringValue("none"),
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
        ["pan", values.pan],
        ["zoom", values.zoom],
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
  triggerFetch: () => void;
};

function useDataSeries(
  dataQuery: DataQuery,
  queryName: string,
  parameters: ValueMap
): SeriesResult {
  const fetchSignal = useMemo(() => new BehaviorSubject<boolean>(false), []);
  const [data, setData] = useState<ResponseNode | undefined>(undefined);
  const [loading, setLoading] = useState<boolean>(false);

  useEffect(() => {
    const seriesQuery = new DataSeriesQuery(
      dataQuery,
      new StringValue(queryName),
      parameters,
      fetchSignal
    );
    const subscriptions: Subscription[] = [];
    subscriptions.push(seriesQuery.response.subscribe(setData));
    subscriptions.push(seriesQuery.loading.subscribe(setLoading));
    return () => {
      subscriptions.forEach((sub) => sub.unsubscribe());
      seriesQuery.dispose();
    };
  }, [dataQuery, fetchSignal, parameters, queryName]);

  const triggerFetch = useCallback(() => {
    fetchSignal.next(true);
    fetchSignal.next(false);
  }, [fetchSignal]);

  return { data, loading, triggerFetch };
}

export default function App(): JSX.Element {
  const logviz = useMemo(() => createLogvizState(), []);
  const emptySet = useMemo(() => new globalThis.Set<string>(), []);

  const sourceSearch = useValue(logviz.values.sourceFileSearchRegex, "") ?? "";
  const rawSearch = useValue(logviz.values.rawEventSearchRegex, "") ?? "";
  const filteredSourceFiles =
    useValue(logviz.values.filteredSourceFiles, emptySet) ?? emptySet;

  const filteredSourceFilesKey = useMemo(
    () => Array.from(filteredSourceFiles).sort().join("|"),
    [filteredSourceFiles]
  );

  const sourceTableParams = useMemo(
    () =>
      new ValueMap(
        new Map([["search_regex", logviz.values.sourceFileSearchRegex]])
      ),
    [logviz.values.sourceFileSearchRegex]
  );
  const rawEntryParams = useMemo(
    () =>
      new ValueMap(
        new Map([["search_regex", logviz.values.rawEventSearchRegex]])
      ),
    [logviz.values.rawEventSearchRegex]
  );

  const sourceTable = useDataSeries(
    logviz.dataQuery,
    QUERY_SOURCE_FILES,
    sourceTableParams
  );
  const rawEntryTable = useDataSeries(
    logviz.dataQuery,
    QUERY_RAW_ENTRIES,
    rawEntryParams
  );

  useEffect(() => {
    sourceTable.triggerFetch();
  }, [sourceSearch, sourceTable.triggerFetch]);

  useEffect(() => {
    rawEntryTable.triggerFetch();
  }, [rawSearch, filteredSourceFilesKey, rawEntryTable.triggerFetch]);

  const sourceTableInteractions = useMemo(() => {
    const sourceFile = new LocalValue(SOURCE_FILE_KEY);
    const globalRef = (key: string): GlobalValueRef =>
      new GlobalValueRef(logviz.core, key);
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
          new Includes(globalRef("filtered_source_files"), sourceFile)
        )
      );
  }, [logviz.core]);

  const rawEntryInteractions = useMemo(() => {
    const sourceFile = new LocalValue(SOURCE_FILE_KEY);
    const timestamp = new LocalValue(TIMESTAMP_KEY);
    const globalRef = (key: string): GlobalValueRef =>
      new GlobalValueRef(logviz.core, key);
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

  return (
    <AppCoreContext.Provider value={logviz.core}>
      <ErrorToast />
      <div className="logviz">
        <header className="hero">
          <div>
            <p className="eyebrow">LogViz React</p>
            <h1>LogViz Tables</h1>
          </div>
          <p className="subtitle">
            React table views backed by the TraceViz data query engine. Timeline
            charting will follow.
          </p>
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
              data={sourceTable.data}
              loading={sourceTable.loading}
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
              data={rawEntryTable.data}
              loading={rawEntryTable.loading}
              interactions={rawEntryInteractions}
              className="table-wrapper raw-events"
              withPagination
              scrollable={false}
              rowHeightPxOverride={18}
              fontSizePxOverride={12}
            />
          </article>
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
