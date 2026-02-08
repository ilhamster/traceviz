/**
 * @fileoverview Utilities for syncing TraceViz ValueMaps with the URL hash.
 */

import { merge, type Subscription } from 'rxjs';

import { ConfigurationError, Severity } from '../errors/errors.js';
import {
  compress,
  decompress,
  serializeHashFragment,
  unserializeHashFragment,
} from '../hash_encoding/hash_encoding.js';
import {
  decodeValueFromString,
  encodeValueToString,
  type ExportedValue,
} from '../value/value.js';
import type { ExportedKeyValueMap } from '../value/value_map.js';
import { ValueMap } from '../value/value_map.js';

const SOURCE = 'url_hash';
const DEFAULT_STATE_KEY = 'state';

/**
 * Configuration for UrlHash.
 */
export type UrlHashOptions = {
  /**
   * ValueMap encoded into a single compressed hash key.
   */
  encoded?: ValueMap;
  /**
   * ValueMap emitted as visible hash key/value pairs.
   * Unencoded values are serialized via encodeValueToString.
   */
  unencoded?: ValueMap;
  /**
   * Keys that should push a new browser history state when changed.
   */
  stateful?: readonly string[];
  /**
   * Hash key used to store encoded state.
   */
  stateKey?: string;
  /**
   * Window instance (useful for testing or non-browser environments).
   */
  window?: Window;
  /**
   * Optional error handler for parse or update failures.
   */
  onError?: (err: unknown) => void;
  /**
   * Optional scheduler used to debounce URL updates.
   */
  schedule?: (callback: () => void) => void;
};

/**
 * Synchronizes TraceViz ValueMaps with the URL hash.
 */
export class UrlHash {
  private readonly encoded?: ValueMap;
  private readonly unencoded?: ValueMap;
  private readonly statefulKeys: Set<string>;
  private readonly stateKey: string;
  private readonly window: Window;
  private readonly onError?: (err: unknown) => void;
  private readonly schedule: (callback: () => void) => void;

  private subscriptions: Subscription | null = null;
  private started = false;
  private updatePending = false;
  private applyingHash = false;

  private readonly handleNavigation = (): void => {
    this.parseURL();
  };

  constructor(options: UrlHashOptions) {
    this.encoded = options.encoded;
    this.unencoded = options.unencoded;
    this.statefulKeys = new Set(options.stateful ?? []);
    this.stateKey = options.stateKey ?? DEFAULT_STATE_KEY;
    this.onError = options.onError;
    this.schedule =
      options.schedule ?? ((callback) => this.window.setTimeout(callback, 0));

    const win = options.window ?? window;
    if (!win) {
      throw new ConfigurationError('window is required for UrlHash')
        .from(SOURCE)
        .at(Severity.ERROR);
    }
    this.window = win;

    this.validateKeyOverlap();
  }

  /**
   * Starts listening for Value changes and URL hash navigation.
   */
  start(): void {
    if (this.started) {
      return;
    }
    this.started = true;
    this.parseURL();
    this.subscribeToValues();
    this.window.addEventListener('popstate', this.handleNavigation);
    this.window.addEventListener('hashchange', this.handleNavigation);
    this.updateURL();
  }

  /**
   * Stops listening for Value changes and URL hash navigation.
   */
  stop(): void {
    if (!this.started) {
      return;
    }
    this.started = false;
    this.window.removeEventListener('popstate', this.handleNavigation);
    this.window.removeEventListener('hashchange', this.handleNavigation);
    this.subscriptions?.unsubscribe();
    this.subscriptions = null;
  }

  /**
   * Parses the current URL hash and updates any configured ValueMaps.
   */
  parseURL(): void {
    const hash = unserializeHashFragment(this.window.location.hash);
    const { [this.stateKey]: encodedState, ...paramJSON } = hash;

    this.applyingHash = true;
    try {
      if (encodedState != null && this.encoded) {
        const decodedStateJSON = this.safeDecompress(encodedState);
        if (decodedStateJSON) {
          this.updateValueMap(this.encoded, decodedStateJSON);
        }
      }

      if (this.unencoded) {
        this.updateValueMapFromStrings(this.unencoded, paramJSON);
      }
    } finally {
      this.applyingHash = false;
    }
  }

  /**
   * Serializes configured ValueMaps into the URL hash.
   */
  updateURL(): void {
    if (this.applyingHash) {
      return;
    }

    const hash = unserializeHashFragment(this.window.location.hash);
    let replaceState = true;
    let decodedStateJSON: ExportedKeyValueMap | undefined;
    let compressedState: string | undefined;
    const unencodedState: { [k: string]: string } = {};

    if (this.encoded && this.encoded.size > 0) {
      const encodedState = hash[this.stateKey];
      if (encodedState != null) {
        decodedStateJSON = this.safeDecompress(encodedState);
      }

      if (this.encoded) {
        for (const [key, value] of this.encoded.entries()) {
          if (!this.statefulKeys.has(key)) {
            continue;
          }
          const priorValue = decodedStateJSON
            ? decodedStateJSON[key]
            : undefined;
          if (!exportedValuesEqual(priorValue, value.exportTo())) {
            replaceState = false;
            break;
          }
        }
        compressedState = compress(this.encoded.exportKeyValueMap());
      }
    }

    if (this.unencoded) {
      for (const [key, value] of this.unencoded.entries()) {
        const encodedValue = encodeValueToString(value);
        const oldValue = hash[key];
        unencodedState[key] = encodedValue;
        if (this.statefulKeys.has(key) && oldValue !== encodedValue) {
          replaceState = false;
        }
      }
    }

    let finalHash: { [k: string]: string } = {};
    if (compressedState) {
      finalHash[this.stateKey] = compressedState;
    }
    finalHash = { ...finalHash, ...unencodedState };

    const serializedHash = serializeHashFragment(finalHash);
    if (serializedHash === this.window.location.hash) {
      return;
    }
    const baseURL = `${this.window.location.pathname}${this.window.location.search}`;
    const newURL = `${baseURL}${serializedHash}`;
    if (replaceState) {
      this.window.history.replaceState(null, '', newURL);
    } else {
      this.window.history.pushState(null, '', newURL);
    }
  }

  private validateKeyOverlap(): void {
    if (this.unencoded?.has(this.stateKey)) {
      throw new ConfigurationError(
        `unencoded ValueMap cannot include reserved key '${this.stateKey}'`,
      )
        .from(SOURCE)
        .at(Severity.ERROR);
    }

    if (!this.encoded || !this.unencoded) {
      return;
    }
    const encodedKeys = new Set(this.encoded.keys());
    const overlapping: string[] = [];
    for (const key of this.unencoded.keys()) {
      if (encodedKeys.has(key)) {
        overlapping.push(key);
      }
    }
    if (overlapping.length > 0) {
      throw new ConfigurationError(
        `encoded and unencoded ValueMaps share keys: ${overlapping.join(', ')}`,
      )
        .from(SOURCE)
        .at(Severity.ERROR);
    }
  }

  private updateValueMap(
    valueMap: ValueMap,
    hashContents: ExportedKeyValueMap,
  ): void {
    // Filter out unexpected keys.
    const filteredHashContents = Object.fromEntries(
      Object.entries(hashContents).filter(([key]) => valueMap.has(key)),
    );
    try {
      valueMap.updateFromExportedKeyValueMap(filteredHashContents);
    } catch (err: unknown) {
      this.handleError(err);
    }
  }

  private updateValueMapFromStrings(
    valueMap: ValueMap,
    hashContents: { [k: string]: string },
  ): void {
    const filteredHashContents = Object.fromEntries(
      Object.entries(hashContents).filter(([key]) => valueMap.has(key)),
    );
    try {
      for (const [key, encoded] of Object.entries(filteredHashContents)) {
        const target = valueMap.get(key);
        if (!decodeValueFromString(target, encoded)) {
          throw new ConfigurationError(
            `failed to decode value for key '${key}'`,
          )
            .from(SOURCE)
            .at(Severity.ERROR);
        }
      }
    } catch (err: unknown) {
      this.handleError(err);
    }
  }

  private safeDecompress(
    encodedState: string,
  ): ExportedKeyValueMap | undefined {
    try {
      return decompress<ExportedKeyValueMap>(encodedState);
    } catch (err: unknown) {
      this.handleError(err);
      return undefined;
    }
  }

  private subscribeToValues(): void {
    const values = [
      ...(this.encoded ? Array.from(this.encoded.values()) : []),
      ...(this.unencoded ? Array.from(this.unencoded.values()) : []),
    ];
    if (values.length === 0) {
      return;
    }
    this.subscriptions = merge(...values).subscribe(() => {
      this.scheduleUpdate();
    });
  }

  private scheduleUpdate(): void {
    if (this.updatePending || this.applyingHash) {
      return;
    }
    this.updatePending = true;
    this.schedule(() => {
      this.updatePending = false;
      this.updateURL();
    });
  }

  private handleError(err: unknown): void {
    if (this.onError) {
      this.onError(err);
      return;
    }
    throw err;
  }
}

function exportedValuesEqual(
  left: ExportedValue | undefined,
  right: ExportedValue,
): boolean {
  if (left === right) {
    return true;
  }
  if (left == null || right == null) {
    return false;
  }
  if (Array.isArray(left) && Array.isArray(right)) {
    if (left.length !== right.length) {
      return false;
    }
    for (let idx = 0; idx < left.length; idx += 1) {
      if (left[idx] !== right[idx]) {
        return false;
      }
    }
    return true;
  }
  if (typeof left === 'object' && typeof right === 'object') {
    const leftRecord = left as Record<string, unknown>;
    const rightRecord = right as Record<string, unknown>;
    const leftKeys = Object.keys(leftRecord);
    const rightKeys = Object.keys(rightRecord);
    if (leftKeys.length !== rightKeys.length) {
      return false;
    }
    for (const key of leftKeys) {
      if (!(key in rightRecord)) {
        return false;
      }
      if (leftRecord[key] !== rightRecord[key]) {
        return false;
      }
    }
    return true;
  }
  return false;
}
