import type {Value} from '@traceviz/client-core';
import {
  Button,
  Card,
  Collapse,
  Group,
  MantineProvider,
  ScrollArea,
  Table,
  Text,
  Title,
} from '@mantine/core';
import {Fragment, useEffect, useMemo, useState} from 'react';
import type {CSSProperties} from 'react';

import '@mantine/core/styles.css';

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
      <Table.Td>{name}</Table.Td>
      <Table.Td style={dividerStyle}>{value.toString()}</Table.Td>
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
    <MantineProvider>
      <Card withBorder radius="md" padding="md" shadow="sm">
        <Group justify="space-between" align="center" mb="xs">
          <Title order={4}>Global State</Title>
          <Button
            size="xs"
            variant="subtle"
            onClick={(): void => setOpened((current) => !current)}
          >
            {opened ? 'Hide' : 'Show'}
          </Button>
        </Group>
        <Text size="sm" c="dimmed" mb="sm">
          Keys update when globals are added; values update live.
        </Text>
        <Collapse in={opened}>
          <ScrollArea.Autosize mah={320}>
            <Table
              striped
              highlightOnHover
              withTableBorder
              withColumnBorders
              style={{tableLayout: 'fixed', width: '100%'}}
            >
              <colgroup>
                {Array.from({length: columnCount * 2}, (_, idx) => (
                  <col key={`col-${idx}`} style={columnStyle} />
                ))}
              </colgroup>
              <Table.Thead>
                <Table.Tr>
                  {Array.from({length: columnCount}, (_, idx) => (
                    <Fragment key={`pair-${idx}`}>
                      <Table.Th>Key</Table.Th>
                      <Table.Th
                        style={idx < columnCount - 1 ? dividerStyle : undefined}
                      >
                        Value
                      </Table.Th>
                    </Fragment>
                  ))}
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {Array.from({length: rowCount}, (_, rowIdx) => (
                  <Table.Tr key={`row-${rowIdx}`}>
                    {columnsEntries.map((column, colIdx) => {
                      const entry = column[rowIdx];
                      if (!entry) {
                        return (
                          <Fragment key={`empty-${colIdx}`}>
                            <Table.Td />
                            <Table.Td
                              style={
                                colIdx < columnCount - 1
                                  ? dividerStyle
                                  : undefined
                              }
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
                            colIdx < columnCount - 1
                              ? dividerStyle
                              : undefined
                          }
                        />
                      );
                    })}
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </ScrollArea.Autosize>
        </Collapse>
      </Card>
    </MantineProvider>
  );
}
