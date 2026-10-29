import { describe, expect, it } from "vitest";
import { canExport, dueText, kindLabel, nairobiToday, resultText } from "./privacy";

describe("dueText", () => {
  it("counts days to the due date", () => {
    expect(dueText("2026-11-05", "2026-10-29")).toBe("Due in 7 days");
    expect(dueText("2026-10-30", "2026-10-29")).toBe("Due tomorrow");
    expect(dueText("2026-10-29", "2026-10-29")).toBe("Due today");
    expect(dueText("2026-10-28", "2026-10-29")).toBe("1 day overdue");
    expect(dueText("2026-10-20", "2026-10-29")).toBe("9 days overdue");
  });
});

describe("resultText", () => {
  it("says what a rule did in each mode", () => {
    expect(resultText({ rule: "enquiries", due: 0, done: 0 }, "apply")).toBe("Nothing due");
    expect(resultText({ rule: "enquiries", due: 3, done: 3 }, "apply")).toBe("3 removed");
    expect(resultText({ rule: "enquiries", due: 3, done: 0 }, "report")).toBe("3 due, not removed (report)");
    expect(resultText({ rule: "removed_files", due: 5, done: 2 }, "apply")).toBe("2 of 5 removed");
  });
});

describe("request kinds", () => {
  it("exports only for access and portability", () => {
    expect(canExport("access")).toBe(true);
    expect(canExport("portability")).toBe(true);
    expect(canExport("erasure")).toBe(false);
    expect(kindLabel("erasure")).toBe("Delete their data");
  });
});

describe("nairobiToday", () => {
  it("uses Nairobi's date, three hours ahead of UTC", () => {
    expect(nairobiToday(new Date("2026-10-29T21:30:00Z"))).toBe("2026-10-30");
    expect(nairobiToday(new Date("2026-10-29T20:59:00Z"))).toBe("2026-10-29");
  });
});
