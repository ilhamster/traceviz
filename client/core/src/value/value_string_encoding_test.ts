import 'jasmine';

import {
  decodeValueFromString,
  encodeValueToString,
  DoubleValue,
  DurationValue,
  IntegerListValue,
  IntegerSetValue,
  IntegerValue,
  StringListValue,
  StringSetValue,
  StringValue,
  TimestampValue,
} from '../core.js';
import { Duration } from '../duration/duration.js';
import { Timestamp } from '../timestamp/timestamp.js';

describe('value string encoding', () => {
  it('encodes and decodes string values', () => {
    const value = new StringValue('hello world');
    const encoded = encodeValueToString(value);
    const target = new StringValue('');
    expect(decodeValueFromString(target, encoded)).toBeTrue();
    expect(target.val).toBe('hello world');
  });

  it('encodes and decodes string list values with escaping', () => {
    const value = new StringListValue(['a,b', 'c\\d', '']);
    const encoded = encodeValueToString(value);
    expect(encoded).toBe('a\\,b,c\\\\d,');
    const target = new StringListValue([]);
    expect(decodeValueFromString(target, encoded)).toBeTrue();
    expect(target.val).toEqual(['a,b', 'c\\d', '']);
  });

  it('encodes and decodes string set values deterministically', () => {
    const value = new StringSetValue(new Set(['b', 'a']));
    const encoded = encodeValueToString(value);
    expect(encoded).toBe('a,b');
    const target = new StringSetValue(new Set());
    expect(decodeValueFromString(target, encoded)).toBeTrue();
    expect(Array.from(target.val).sort()).toEqual(['a', 'b']);
  });

  it('encodes and decodes integer list values', () => {
    const value = new IntegerListValue([3, -2, 1]);
    const encoded = encodeValueToString(value);
    expect(encoded).toBe('3,-2,1');
    const target = new IntegerListValue([]);
    expect(decodeValueFromString(target, encoded)).toBeTrue();
    expect(target.val).toEqual([3, -2, 1]);
  });

  it('encodes and decodes integer set values deterministically', () => {
    const value = new IntegerSetValue(new Set([3, 1]));
    const encoded = encodeValueToString(value);
    expect(encoded).toBe('1,3');
    const target = new IntegerSetValue(new Set());
    expect(decodeValueFromString(target, encoded)).toBeTrue();
    expect(Array.from(target.val).sort((a, b) => a - b)).toEqual([1, 3]);
  });

  it('encodes and decodes numeric values', () => {
    const intValue = new IntegerValue(42);
    const intEncoded = encodeValueToString(intValue);
    const intTarget = new IntegerValue(0);
    expect(decodeValueFromString(intTarget, intEncoded)).toBeTrue();
    expect(intTarget.val).toBe(42);

    const doubleValue = new DoubleValue(3.14);
    const doubleEncoded = encodeValueToString(doubleValue);
    const doubleTarget = new DoubleValue(0);
    expect(decodeValueFromString(doubleTarget, doubleEncoded)).toBeTrue();
    expect(doubleTarget.val).toBeCloseTo(3.14, 10);
  });

  it('encodes and decodes duration values', () => {
    const value = new DurationValue(new Duration(1_500_000_000));
    const encoded = encodeValueToString(value);
    expect(encoded).toBe('1500000000');
    const target = new DurationValue(new Duration(0));
    expect(decodeValueFromString(target, '1.5s')).toBeTrue();
    expect(target.val.nanos).toBe(1_500_000_000);
  });

  it('encodes and decodes timestamp values', () => {
    const value = new TimestampValue(new Timestamp(170, 42));
    const encoded = encodeValueToString(value);
    expect(encoded).toBe('170.000000042');
    const target = new TimestampValue(new Timestamp(0, 0));
    expect(decodeValueFromString(target, encoded)).toBeTrue();
    expect(target.val.seconds).toBe(170);
    expect(target.val.nanos).toBe(42);
  });

  it('rejects invalid integer values', () => {
    const target = new IntegerValue(0);
    expect(decodeValueFromString(target, 'abc')).toBeFalse();
  });
});
