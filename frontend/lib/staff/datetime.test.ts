import { describe, expect, it } from "vitest";
import { formatDate, isoToNairobiInput, nairobiInputToIso } from "./datetime";

describe("Nairobi date-time inputs", () => {
  it("shows a UTC timestamp in Nairobi time", () => {
    expect(isoToNairobiInput("2027-01-11T07:00:00Z")).toBe("2027-01-11T10:00");
    expect(isoToNairobiInput("2027-01-11T22:30:00Z")).toBe("2027-01-12T01:30");
  });

  it("round-trips", () => {
    const iso = nairobiInputToIso("2027-01-11T10:00");
    expect(iso).toBe("2027-01-11T10:00:00+03:00");
    expect(isoToNairobiInput(iso)).toBe("2027-01-11T10:00");
  });

  it("rejects incomplete input", () => {
    expect(nairobiInputToIso("2027-01-11")).toBeNull();
    expect(isoToNairobiInput("")).toBe("");
  });

  it("formats dates without shifting the day", () => {
    expect(formatDate("2027-01-11")).toBe("11 January 2027");
  });
});
