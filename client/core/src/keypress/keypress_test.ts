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

import 'jasmine';

import {Action, Interactions, Set as SetAction} from '../interactions/interactions.js';
import {StringSetValue} from '../value/value.js';
import {FixedValue, LocalValue} from '../value/value_reference.js';

import {Keypress, KeypressEvent, KeypressEventSource} from './keypress.js';

class FakeKeypressEventSource implements KeypressEventSource {
  private readonly listenersByType =
      new Map<string, Array<(event: KeypressEvent) => void>>([
        ['keydown', []],
        ['keyup', []],
      ]);

  addEventListener(
      type: 'keydown'|'keyup', listener: (event: KeypressEvent) => void): void {
    this.listenersByType.get(type)?.push(listener);
  }

  removeEventListener(
      type: 'keydown'|'keyup', listener: (event: KeypressEvent) => void): void {
    const listeners = this.listenersByType.get(type);
    if (listeners === undefined) {
      return;
    }
    const idx = listeners.indexOf(listener);
    if (idx >= 0) {
      listeners.splice(idx, 1);
    }
  }

  emit(type: 'keydown'|'keyup', code: string) {
    for (const listener of this.listenersByType.get(type) ?? []) {
      listener({type, code});
    }
  }
}

describe('keypress test', () => {
  it('handles keypress actions through interactions', () => {
    const depressedKeyCodes = new StringSetValue(new Set<string>());
    const kp = new Keypress(depressedKeyCodes);
    const fakeEventSource = new FakeKeypressEventSource();
    kp.attach(fakeEventSource);

    fakeEventSource.emit('keydown', 'KeyA');
    expect(depressedKeyCodes.val).toEqual(new Set(['KeyA']));

    fakeEventSource.emit('keydown', 'ControlLeft');
    expect(depressedKeyCodes.val).toEqual(new Set(['KeyA', 'ControlLeft']));

    fakeEventSource.emit('keyup', 'KeyA');
    expect(depressedKeyCodes.val).toEqual(new Set(['ControlLeft']));

    fakeEventSource.emit('keyup', 'ControlLeft');
    expect(depressedKeyCodes.val).toEqual(new Set([]));
  });

  it('does not process key events while detached', () => {
    const depressedKeyCodes = new StringSetValue(new Set<string>());
    const kp = new Keypress(depressedKeyCodes);
    const fakeEventSource = new FakeKeypressEventSource();
    kp.attach(fakeEventSource);
    kp.detach();

    fakeEventSource.emit('keydown', 'KeyA');
    expect(depressedKeyCodes.val).toEqual(new Set([]));
  });
});
