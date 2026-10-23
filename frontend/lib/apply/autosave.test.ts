import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AUTOSAVE_DELAY_MS, useAutosave, type SaveOutcome } from "./autosave";

describe("useAutosave", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("saves once, a moment after the last change", async () => {
    const save = vi.fn(async (): Promise<SaveOutcome> => ({ ok: true }));
    const { result } = renderHook(() => useAutosave({ name: "" }, save));
    act(() => result.current.update({ name: "T" }));
    act(() => result.current.update({ name: "Te" }));
    await act(async () => vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS - 1));
    expect(save).not.toHaveBeenCalled();
    await act(async () => vi.advanceTimersByTimeAsync(1));
    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenLastCalledWith({ name: "Te" });
    expect(result.current.state.kind).toBe("saved");
  });

  it("keeps a failed save pending and sends it on flush", async () => {
    const outcomes: SaveOutcome[] = [{ ok: false, message: "offline" }, { ok: true }];
    const save = vi.fn(async () => outcomes.shift()!);
    const { result } = renderHook(() => useAutosave({ n: 0 }, save));
    act(() => result.current.update({ n: 1 }));
    await act(async () => vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS));
    expect(result.current.state).toEqual({ kind: "failed", message: "offline" });
    let ok = false;
    await act(async () => {
      ok = await result.current.flush();
    });
    expect(ok).toBe(true);
    expect(save).toHaveBeenCalledTimes(2);
  });

  it("shows refused fields and does not report the step as saved", async () => {
    const save = vi.fn(async (): Promise<SaveOutcome> => ({ ok: false, fields: { phone: "Enter digits." } }));
    const { result } = renderHook(() => useAutosave({ phone: "x" }, save));
    act(() => result.current.update({ phone: "call me" }));
    let ok = true;
    await act(async () => {
      ok = await result.current.flush();
    });
    expect(ok).toBe(false);
    expect(result.current.fields).toEqual({ phone: "Enter digits." });
    // Flushing again without a change does not resend, and still says no.
    await act(async () => {
      ok = await result.current.flush();
    });
    expect(ok).toBe(false);
    expect(save).toHaveBeenCalledTimes(1);
  });

  it("saves a change made during a save straight after it, in order", async () => {
    let release: () => void = () => {};
    const seen: number[] = [];
    const save = vi.fn(async (v: { n: number }): Promise<SaveOutcome> => {
      seen.push(v.n);
      if (seen.length === 1) await new Promise<void>((r) => (release = r));
      return { ok: true };
    });
    const { result } = renderHook(() => useAutosave({ n: 0 }, save));
    act(() => result.current.update({ n: 1 }));
    let flushed: Promise<boolean> = Promise.resolve(false);
    await act(async () => {
      flushed = result.current.flush();
      // Let the first save start before the next change.
      for (let i = 0; i < 5 && seen.length === 0; i++) await Promise.resolve();
    });
    expect(seen).toEqual([1]);
    act(() => result.current.update({ n: 2 }));
    await act(async () => release());
    await act(async () => {
      expect(await flushed).toBe(true);
    });
    expect(seen).toEqual([1, 2]);
  });
});
