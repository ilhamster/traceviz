/**
 * @fileoverview A framework-agnostic keypress handler that maps browser
 * keydown/keyup events into TraceViz interactions.
 */

/*
        Copyright 2023 Google Inc.
        Licensed under the Apache License, Version 2.0 (the "License");
        you may not use this file except in compliance with the License.
        You may obtain a copy of the License at
                https://www.apache.org/licenses/LICENSE-2.0
        Unless required by applicable law or agreed to in writing, software
        distributed under the License is distributed on an "AS IS" BASIS,
        WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
        See the License for the specific language governing permissions and
        limitations under the License.
*/

import {Interactions} from '../interactions/interactions.js';
import {StringSetValue} from '../value/value.js';

export interface KeypressEvent {
  type: string;
  code: string;
}

export interface KeypressEventSource {
  addEventListener(
      type: 'keydown'|'keyup', listener: (event: KeypressEvent) => void): void;
  removeEventListener(
      type: 'keydown'|'keyup', listener: (event: KeypressEvent) => void): void;
}

/**
 * Listens to keydown/keyup and routes updates to Interactions 'key/press'.
 */
export class Keypress {
  private attachedEventSource: KeypressEventSource|undefined;
  private readonly listener = (event: KeypressEvent) => {
    this.keyEvent(event);
  };

  constructor(private readonly depressedKeyCodesVal: StringSetValue) {}

  attach(eventSource: KeypressEventSource) {
    if (this.attachedEventSource === eventSource) {
      return;
    }
    this.detach();
    this.attachedEventSource = eventSource;
    eventSource.addEventListener('keydown', this.listener);
    eventSource.addEventListener('keyup', this.listener);
  }

  detach() {
    if (this.attachedEventSource === undefined) {
      return;
    }
    this.attachedEventSource.removeEventListener('keydown', this.listener);
    this.attachedEventSource.removeEventListener('keyup', this.listener);
    this.attachedEventSource = undefined;
  }

  keyEvent(event: KeypressEvent) {
    if (event.code === '') {
      return;
    }
    // Assign a fresh Set so StringSetValue change detection can detect
    // updates.
    const depressedKeyCodes = new Set(this.depressedKeyCodesVal.val);
    if (event.type === 'keydown') {
      depressedKeyCodes.add(event.code);
    } else if (event.type === 'keyup') {
      depressedKeyCodes.delete(event.code);
    } else {
      return;
    }
    this.depressedKeyCodesVal.val = depressedKeyCodes;
  }
}
