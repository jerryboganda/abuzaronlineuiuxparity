/**
 * HID "wedge" barcode scanner detection.
 *
 * A physical HID barcode scanner behaves like a very fast keyboard: it types the
 * encoded digits/characters and then sends Enter (sometimes Tab) to terminate the
 * scan. This module distinguishes that pattern from ordinary human typing by
 * tracking the elapsed time between consecutive keystrokes on a single input –
 * scanners emit characters far faster than a person can type (typically well
 * under ~30-40ms apart), so a burst of keystrokes where nearly every gap is that
 * fast, terminated by Enter/Tab, is treated as a completed scan.
 *
 * Usage: wire `handleKeydown` into the `onkeydown` handler of the existing item
 * lookup / search input on a page. It never interferes with normal typing –
 * ordinary keystrokes simply fall through (the function returns false) so the
 * caller's own Enter handling (e.g. running a text search) still runs. Only when
 * the keystroke pattern looks like a scan does it intercept the terminating key
 * and invoke the `onScan` callback with the buffered code instead.
 */

export interface BarcodeScanListenerOptions {
  /**
   * Maximum milliseconds allowed between two consecutive keystrokes for the gap
   * to count as "scanner speed". Defaults to 40ms, comfortably faster than
   * sustained human typing (~80-150ms between keys) while still generous for
   * slower USB HID scanners.
   */
  maxInterKeystrokeMs?: number;
  /** Minimum number of characters required before a burst is treated as a barcode. */
  minLength?: number;
}

export interface BarcodeScanListener {
  /**
   * Feed every keydown event from the monitored input into this function.
   * Returns true when the event was consumed as a completed barcode scan (the
   * terminating key's default action was prevented and `onScan` was invoked) –
   * callers should skip their own handling of that keystroke in that case.
   */
  handleKeydown(event: KeyboardEvent): boolean;
  /** Discard any buffered keystrokes, e.g. when the input loses focus. */
  reset(): void;
}

const DEFAULT_MAX_INTER_KEYSTROKE_MS = 40;
const DEFAULT_MIN_LENGTH = 4;

export function createBarcodeScanListener(onScan: (code: string) => void, options: BarcodeScanListenerOptions = {}): BarcodeScanListener {
  const maxInterKeystrokeMs = options.maxInterKeystrokeMs ?? DEFAULT_MAX_INTER_KEYSTROKE_MS;
  const minLength = options.minLength ?? DEFAULT_MIN_LENGTH;
  let buffer = '';
  let lastKeystrokeAt = 0;
  let fastGaps = 0;
  let totalGaps = 0;

  function now(): number {
    return typeof performance !== 'undefined' && typeof performance.now === 'function' ? performance.now() : Date.now();
  }

  function reset(): void {
    buffer = '';
    lastKeystrokeAt = 0;
    fastGaps = 0;
    totalGaps = 0;
  }

  function handleKeydown(event: KeyboardEvent): boolean {
    if (event.key === 'Enter' || event.key === 'Tab') {
      const scanned = buffer;
      const isScan = scanned.length >= minLength && totalGaps > 0 && fastGaps === totalGaps;
      reset();
      if (isScan) {
        event.preventDefault();
        onScan(scanned);
        return true;
      }
      return false;
    }
    // Ignore modifier keys and non-character keys (ArrowLeft, Backspace, etc.) without
    // resetting on Shift, since shifted characters are common in scanner payloads.
    if (event.key.length !== 1) {
      if (event.key !== 'Shift') reset();
      return false;
    }
    if (event.ctrlKey || event.metaKey || event.altKey) {
      reset();
      return false;
    }
    const currentTime = now();
    if (lastKeystrokeAt) {
      const elapsed = currentTime - lastKeystrokeAt;
      totalGaps += 1;
      if (elapsed <= maxInterKeystrokeMs) fastGaps += 1;
    }
    buffer += event.key;
    lastKeystrokeAt = currentTime;
    return false;
  }

  return { handleKeydown, reset };
}

/**
 * Client-side mirror of the edge service's `hardware.NormalizeBarcode`: trims
 * surrounding whitespace and rejects empty or control-character input before a
 * network round trip is attempted. The edge route re-validates and is the
 * source of truth; this is only a fast local pre-check.
 */
export function normalizeBarcodeClientSide(raw: string): string | null {
  const trimmed = raw.replace(/^[ \t\r\n]+|[ \t\r\n]+$/g, '');
  if (!trimmed) return null;
  for (const character of trimmed) {
    const codePoint = character.codePointAt(0) ?? 0;
    if (codePoint < 0x20 || codePoint === 0x7f) return null;
  }
  return trimmed;
}
