import { describe, expect, it } from "vitest";
import { sortTimetable, weekdayName } from "./academics";

describe("weekdayName", () => {
  it("names 1 to 7 from Monday and nothing else", () => {
    expect(weekdayName(1)).toBe("Monday");
    expect(weekdayName(7)).toBe("Sunday");
    expect(weekdayName(0)).toBe("");
    expect(weekdayName(null)).toBe("");
  });
});

describe("sortTimetable", () => {
  it("puts classes by day and time first, then exams by date", () => {
    const e = (
      kind: "class" | "exam",
      weekday: number | null,
      on_date: string | null,
      starts_at: string,
    ) => ({
      kind,
      weekday,
      on_date,
      starts_at,
    });
    const sorted = sortTimetable([
      e("exam", null, "2027-03-02", "09:00"),
      e("class", 3, null, "08:00"),
      e("exam", null, "2027-03-01", "14:00"),
      e("class", 1, null, "10:00"),
      e("class", 1, null, "08:30"),
    ]);
    expect(sorted.map((x) => `${x.kind}${x.weekday ?? x.on_date}${x.starts_at}`)).toEqual([
      "class108:30",
      "class110:00",
      "class308:00",
      "exam2027-03-0114:00",
      "exam2027-03-0209:00",
    ]);
  });
});
