import type {Value} from '@traceviz/client-core';
import {Fragment, useEffect, useMemo, useState} from 'react';
import type {CSSProperties} from 'react';

import {useAppCore} from '../../core';
import {useValue, type ValueWithVal} from '../../core';

type GlobalStateMonitorProps = {
  columns?: number;
};

type GlobalStateEntry = [string, Value];

function GlobalValueCells({
  name,
  value,
  dividerStyle,
}: {
  name: string;
  value: Value;
  dividerStyle?: CSSProperties;
}): JSX.Element {
  // Subscribe so these cells re-render when the underlying Value changes.
  useValue(value as ValueWithVal<Value>);
  return (
    <>
      <td style={{padding: '4px 6px', borderBottom: '1px solid var(--panel-border)'}}>
        {name}
      </td>
      <td
        style={{
          padding: '4px 6px',
          borderBottom: '1px solid var(--panel-border)',
          ...dividerStyle,
        }}
      >
        {value.toString()}
      </td>
    </>
  );
}

function normalizeColumnCount(columns?: number): number {
  if (!columns || columns < 1) {
    return 1;
  }
  return Math.floor(columns);
}

function splitEntries(
  entries: GlobalStateEntry[],
  columns: number,
): GlobalStateEntry[][] {
  if (columns <= 1 || entries.length === 0) {
    return [entries];
  }
  const baseSize = Math.floor(entries.length / columns);
  const remainder = entries.length % columns;
  const result: GlobalStateEntry[][] = [];
  let offset = 0;
  for (let idx = 0; idx < columns; idx += 1) {
    const size = baseSize + (idx < remainder ? 1 : 0);
    result.push(entries.slice(offset, offset + size));
    offset += size;
  }
  return result;
}

export function GlobalStateMonitor({
  columns = 1,
}: GlobalStateMonitorProps): JSX.Element {
  const appCore = useAppCore();
  const [opened, setOpened] = useState<boolean>(false);
  const [keys, setKeys] = useState<string[]>([]);
  useEffect((): (() => void) => {
    // Initialize immediately, then subscribe to key-set changes.
    setKeys(appCore.globalState.value);
    const sub = appCore.globalState.subscribe((newKeys: string[]) => {
      setKeys(newKeys);
    });
    return (): void => sub.unsubscribe();
  }, [appCore]);

  const entries = useMemo<GlobalStateEntry[]>(
    () =>
      keys
        .map((key) => [key, appCore.globalState.get(key)] as GlobalStateEntry)
        .sort((a, b) => a[0].localeCompare(b[0])),
    [appCore, keys],
  );
  const columnCount = useMemo(
    () => normalizeColumnCount(columns),
    [columns],
  );
  const columnsEntries = useMemo(
    () => splitEntries(entries, columnCount),
    [entries, columnCount],
  );
  const rowCount = useMemo(() => {
    if (columnsEntries.length === 0) {
      return 0;
    }
    return Math.max(...columnsEntries.map((column) => column.length));
  }, [columnsEntries]);
  const dividerStyle = useMemo<CSSProperties>(
    () => ({borderRight: '2px solid var(--mantine-color-gray-4)'}),
    [],
  );
  const columnWidth = useMemo(() => 100 / (columnCount * 2), [columnCount]);
  const columnStyle = useMemo<CSSProperties>(
    () => ({width: `${columnWidth}%`}),
    [columnWidth],
  );
  return (
    <div
      style={{
        border: '1px solid var(--panel-border)',
        background: 'var(--input-bg)',
        padding: 8,
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 6,
        }}
      >
        <h3 style={{margin: 0, fontSize: 13, fontWeight: 600}}>Global State</h3>
        <button
          type="button"
          onClick={(): void => setOpened((current) => !current)}
          style={{
            border: '1px solid var(--panel-border)',
            background: 'var(--panel)',
            color: 'var(--text)',
            fontSize: 11,
            padding: '2px 8px',
            cursor: 'pointer',
          }}
        >
          {opened ? 'Hide' : 'Show'}
        </button>
      </div>
      <p style={{margin: '0 0 6px', fontSize: 11, color: 'var(--muted)'}}>
        Keys update when globals are added; values update live.
      </p>
      {opened ? (
        <div style={{maxHeight: 320, overflow: 'auto'}}>
          <table
            style={{
              tableLayout: 'fixed',
              width: '100%',
              borderCollapse: 'collapse',
              fontSize: 12,
            }}
          >
            <colgroup>
              {Array.from({length: columnCount * 2}, (_, idx) => (
                <col key={`col-${idx}`} style={columnStyle} />
              ))}
            </colgroup>
            <thead>
              <tr>
                {Array.from({length: columnCount}, (_, idx) => (
                  <Fragment key={`pair-${idx}`}>
                    <th
                      style={{
                        textAlign: 'left',
                        padding: '4px 6px',
                        borderBottom: '1px solid var(--panel-border)',
                      }}
                    >
                      Key
                    </th>
                    <th
                      style={{
                        textAlign: 'left',
                        padding: '4px 6px',
                        borderBottom: '1px solid var(--panel-border)',
                        ...(idx < columnCount - 1 ? dividerStyle : undefined),
                      }}
                    >
                      Value
                    </th>
                  </Fragment>
                ))}
              </tr>
            </thead>
            <tbody>
              {Array.from({length: rowCount}, (_, rowIdx) => (
                <tr key={`row-${rowIdx}`}>
                  {columnsEntries.map((column, colIdx) => {
                    const entry = column[rowIdx];
                    if (!entry) {
                      return (
                        <Fragment key={`empty-${colIdx}`}>
                          <td
                            style={{
                              padding: '4px 6px',
                              borderBottom: '1px solid var(--panel-border)',
                            }}
                          />
                          <td
                            style={{
                              padding: '4px 6px',
                              borderBottom: '1px solid var(--panel-border)',
                              ...(colIdx < columnCount - 1
                                ? dividerStyle
                                : undefined),
                            }}
                          />
                        </Fragment>
                      );
                    }
                    const [name, value] = entry;
                    return (
                      <GlobalValueCells
                        key={`col-${name}`}
                        name={name}
                        value={value}
                        dividerStyle={
                          colIdx < columnCount - 1 ? dividerStyle : undefined
                        }
                      />
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  );
}
