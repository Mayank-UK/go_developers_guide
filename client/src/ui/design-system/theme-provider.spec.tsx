import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { ThemeProvider } from './theme-provider';
import { STORAGE_KEY, useTheme } from './theme';

/** Reads the live mode out of the context so assertions observe behavior. */
function ModeProbe() {
  const { mode, toggle } = useTheme();
  return (
    <p data-testid="mode" onClick={toggle}>
      {mode}
    </p>
  );
}

describe('ThemeProvider', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
  });

  it('defaults to dark when nothing is stored and persists it under gdg-theme', () => {
    render(
      <ThemeProvider>
        <ModeProbe />
      </ThemeProvider>,
    );

    expect(screen.getByTestId('mode')).toHaveTextContent('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('dark');
  });

  it('toggle flips the mode, the html attribute, and the stored value', () => {
    render(
      <ThemeProvider>
        <ModeProbe />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByTestId('mode'));

    expect(screen.getByTestId('mode')).toHaveTextContent('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('light');
  });
});
