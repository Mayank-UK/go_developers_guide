import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// jsdom does not implement window.matchMedia; stub it once (always-unmatched)
// so any theme / media-query code under test sees a working global instead of
// a missing one. Guarded so a future jsdom that ships matchMedia wins.
if (typeof window.matchMedia !== 'function') {
  window.matchMedia = (query: string): MediaQueryList => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  });
}

afterEach(() => {
  cleanup();
});
