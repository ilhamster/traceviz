import 'jasmine';

import {
  IntegerValue,
  StringValue,
  UrlHash,
  Value,
  ValueMap,
} from '../core.js';

type Listener = () => void;

class FakeHistory {
  private readonly entries: string[] = [];
  private index = -1;

  constructor(
    private readonly location: LocationLike,
    private readonly onPopState: Listener,
    private readonly onHashChange: Listener,
  ) {}

  get states(): string[] {
    return [...this.entries];
  }

  pushState(_data: unknown, _title: string, url: string) {
    if (this.index < this.entries.length - 1) {
      this.entries.splice(this.index + 1);
    }
    this.location.apply(url);
    this.entries.push(url);
    this.index = this.entries.length - 1;
  }

  replaceState(_data: unknown, _title: string, url: string) {
    this.location.apply(url);
    if (this.index >= 0) {
      this.entries[this.index] = url;
    } else {
      this.entries.push(url);
      this.index = 0;
    }
  }

  back() {
    if (this.index <= 0) {
      return;
    }
    const priorHash = this.location.hash;
    this.index -= 1;
    this.location.apply(this.entries[this.index]);
    this.onPopState();
    if (priorHash !== this.location.hash) {
      this.onHashChange();
    }
  }

  forward() {
    if (this.index >= this.entries.length - 1) {
      return;
    }
    const priorHash = this.location.hash;
    this.index += 1;
    this.location.apply(this.entries[this.index]);
    this.onPopState();
    if (priorHash !== this.location.hash) {
      this.onHashChange();
    }
  }
}

class LocationLike {
  pathname = '/';
  search = '';
  hash = '';

  apply(url: string) {
    const [pathAndSearch, hash = ''] = url.split('#', 2);
    const [path, search = ''] = pathAndSearch.split('?', 2);
    this.pathname = path || '/';
    this.search = search ? `?${search}` : '';
    this.hash = hash ? `#${hash}` : '';
  }
}

class FakeWindow {
  readonly location = new LocationLike();
  private readonly listeners = new Map<string, Set<Listener>>();
  readonly history = new FakeHistory(
    this.location,
    () => this.triggerPopState(),
    () => this.triggerHashChange(),
  );

  addEventListener(event: string, listener: Listener) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(listener);
  }

  removeEventListener(event: string, listener: Listener) {
    this.listeners.get(event)?.delete(listener);
  }

  trigger(event: string) {
    this.listeners.get(event)?.forEach((listener) => listener());
  }

  triggerPopState() {
    this.trigger('popstate');
  }

  triggerHashChange() {
    this.trigger('hashchange');
  }

  back() {
    this.history.back();
  }

  forward() {
    this.history.forward();
  }
}

describe('url hash', () => {
  it('updates unencoded from hash on navigation events', () => {
    const collectionName = new StringValue('trace-1');
    const magicNumber = new IntegerValue(42);
    const fakeWindow = new FakeWindow();
    const urlHash = new UrlHash({
      unencoded: new ValueMap(
        new Map<string, Value>([
          ['collection_name', collectionName],
          ['magic_number', magicNumber],
        ]),
      ),
      stateful: ['collection_name'],
      window: fakeWindow as unknown as Window,
      schedule: (callback: () => void) => {
        callback();
      },
    });
    urlHash.start();

    expect(fakeWindow.location.hash).toBe(
      '#collection_name=trace-1&magic_number=42',
    );

    collectionName.val = 'trace-2';
    magicNumber.val = 100;
    expect(fakeWindow.location.hash).toBe(
      '#collection_name=trace-2&magic_number=100',
    );
    fakeWindow.back();
    expect(fakeWindow.location.hash).toBe(
      '#collection_name=trace-1&magic_number=42',
    );
  });
});
