import * as matchers from '@testing-library/jest-dom/matchers';
import { expect } from 'vitest';

expect.extend(matchers);

// React Router creates navigation signals in the jsdom realm, while Node 24's
// native Request validates signals against undici's realm. Preserve the real
// Request implementation but attach the already-valid DOM signal after native
// construction so redirects exercise the router in tests.
const NativeRequest = globalThis.Request;
class CompatibleRequest extends NativeRequest {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    if (!init?.signal) {
      super(input, init);
      return;
    }
    const { signal, ...nativeInit } = init;
    super(input, nativeInit);
    Object.defineProperty(this, 'signal', {
      configurable: true,
      value: signal,
    });
  }
}
globalThis.Request = CompatibleRequest;

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
});

class ResizeObserverStub {
  constructor(private readonly callback: ResizeObserverCallback) {}
  observe(target: Element) {
    this.callback(
      [
        {
          target,
          contentRect: {
            width: 800,
            height: 300,
            x: 0,
            y: 0,
            top: 0,
            left: 0,
            right: 800,
            bottom: 300,
            toJSON: () => ({}),
          },
          borderBoxSize: [],
          contentBoxSize: [],
          devicePixelContentBoxSize: [],
        },
      ],
      this as unknown as ResizeObserver,
    );
  }
  unobserve() {}
  disconnect() {}
}

globalThis.ResizeObserver = ResizeObserverStub;

// Base UI dispatches a PointerEvent when a headless switch is clicked. jsdom
// does not currently expose that constructor, so provide the browser-shaped
// subset needed by interaction tests.
if (!window.PointerEvent) {
  class PointerEventStub extends MouseEvent {
    readonly pointerId = 1;
    readonly pointerType = 'mouse';
    readonly isPrimary = true;
  }
  Object.defineProperty(window, 'PointerEvent', {
    configurable: true,
    value: PointerEventStub,
  });
  Object.defineProperty(globalThis, 'PointerEvent', {
    configurable: true,
    value: PointerEventStub,
  });
}
