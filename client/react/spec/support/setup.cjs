const { JSDOM } = require("jsdom");

const dom = new JSDOM("<!doctype html><html><body></body></html>");
const win = dom.window;

globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const requestAnimationFrameSafe = (cb) => setTimeout(() => cb(0), 0);
const cancelAnimationFrameSafe = (handle) => clearTimeout(handle);

const installAnimationFrame = (target) => {
  if (typeof target.requestAnimationFrame !== "function") {
    target.requestAnimationFrame = requestAnimationFrameSafe;
  }
  if (typeof target.cancelAnimationFrame !== "function") {
    target.cancelAnimationFrame = cancelAnimationFrameSafe;
  }
};

const applyWindow = (value) => {
  if (!value) {
    return;
  }
  globalThis.document = value.document;
  globalThis.navigator = value.navigator;
  globalThis.HTMLElement = value.HTMLElement;
  globalThis.SVGElement = value.SVGElement;
  globalThis.self = value;
  if (!value.ResizeObserver) {
    value.ResizeObserver = globalThis.ResizeObserver;
  }
  installAnimationFrame(value);
  installAnimationFrame(globalThis);
};

installAnimationFrame(globalThis);

globalThis.__tracevizApplyWindow = applyWindow;

let currentWindow = win;
try {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    enumerable: true,
    get() {
      return currentWindow;
    },
    set(value) {
      currentWindow = value;
      applyWindow(value);
    },
  });
} catch (err) {
  // Fall back to direct assignment if defineProperty fails.
  globalThis.window = currentWindow;
}

globalThis.window = win;
applyWindow(win);
