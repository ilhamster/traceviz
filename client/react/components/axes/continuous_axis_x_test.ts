import 'jasmine';

import {
  Axis,
  AxisType,
  Duration,
  Timestamp,
} from '@traceviz/client-core';

import {domainFromAxis, tickFormatterForAxis} from './continuous_axis_x.tsx';

function sec(seconds: number): Timestamp {
  return new Timestamp(seconds, 0);
}

function nanos(n: number): Duration {
  return new Duration(n);
}

describe('StandardContinuousXAxis helpers', () => {
  it('formats timestamp ticks as duration since start', () => {
    const axis = new Axis<Timestamp>(
        AxisType.TIMESTAMP,
        // Category fields are only used for display here.
        {id: 'x', displayName: 'time', description: ''},
        sec(10),
        sec(11),
        (properties, key) => properties.expectTimestamp(key),
        (a, b) => b.sub(a).nanos,
    ) as Axis<unknown>;

    const fmt = tickFormatterForAxis(axis);
    expect(fmt).toBeDefined();
    expect(fmt!(500_000_000)).toBe('500.000ms');
  });

  it('formats duration ticks using Duration.toString', () => {
    const axis = new Axis<Duration>(
        AxisType.DURATION,
        {id: 'x', displayName: 'dur', description: ''},
        nanos(0),
        nanos(2_000_000),
        (properties, key) => properties.expectDuration(key),
        (a, b) => b.sub(a).nanos,
    ) as Axis<unknown>;

    const fmt = tickFormatterForAxis(axis);
    expect(fmt).toBeDefined();
    expect(fmt!(1_500)).toBe('1.500μs');
  });

  it('leaves double ticks unformatted and preserves domain', () => {
    const typed = new Axis<number>(
        AxisType.DOUBLE,
        {id: 'x', displayName: 'x', description: ''},
        1,
        3,
        (properties, key) => properties.expectNumber(key),
        (a, b) => b - a,
    ) as Axis<unknown>;

    expect(tickFormatterForAxis(typed)).toBeUndefined();
    expect(domainFromAxis(typed)).toEqual([1, 3]);
  });
});
