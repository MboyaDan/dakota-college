import { describe, expect, it } from "vitest";
import {
  cellKey,
  changedEntries,
  describeEvent,
  describeResult,
  describeScheme,
  passCount,
  weightTotal,
  type Sheet,
} from "./results";

const result = (total: number | null, grade = "", passed = false, complete = total !== null) => ({
  complete,
  total,
  grade,
  passed,
  outcome: null,
});

function sheet(kind: "marks" | "competency" = "marks"): Sheet {
  const components =
    kind === "marks"
      ? [
          { key: "cat", name: "Continuous assessment", weight: 30 },
          { key: "exam", name: "Final exam", weight: 70 },
        ]
      : [{ key: "competency", name: "Competency", weight: 0 }];
  return {
    id: "s",
    offering_id: "o",
    status: "draft",
    scheme: { kind, components, pass_mark: 40, grade_bands: [{ grade: "A", min: 0 }] },
    unit_code: "ICT 101",
    unit_title: "Basics",
    term_name: "Term",
    complete: false,
    events: [],
    released_version: 0,
    updated_at: "2026-10-27T00:00:00Z",
    rows: [
      {
        student_id: "a",
        student_number: "DTC/SAMPLE/0001",
        name: "A",
        marks: kind === "marks" ? { cat: { score: 20, outcome: null } } : {},
        result: result(70, "A", true),
      },
      { student_id: "b", student_number: "DTC/SAMPLE/0002", name: "B", marks: {}, result: result(null) },
    ],
  };
}

describe("changedEntries", () => {
  it("sends only the fields that changed", () => {
    const s = sheet();
    const { entries, keys, problems } = changedEntries(s, {
      [cellKey("a", "cat")]: "20",
      [cellKey("a", "exam")]: "65",
      [cellKey("b", "cat")]: " 7 ",
    });
    expect(entries).toEqual([
      { student_id: "a", component: "exam", score: 65 },
      { student_id: "b", component: "cat", score: 7 },
    ]);
    expect(keys).toEqual(["a:exam", "b:cat"]);
    expect(problems).toEqual({});
  });

  it("refuses numbers outside 0 to 100, decimals and removed marks", () => {
    const { entries, problems } = changedEntries(sheet(), {
      [cellKey("a", "cat")]: "",
      [cellKey("a", "exam")]: "101",
      [cellKey("b", "cat")]: "4.5",
      [cellKey("b", "exam")]: "-1",
    });
    expect(entries).toEqual([]);
    expect(Object.keys(problems).sort()).toEqual(["a:cat", "a:exam", "b:cat", "b:exam"]);
    expect(problems["a:cat"]).toMatch(/cannot be removed/);
  });

  it("takes outcomes on a competency sheet", () => {
    const { entries, problems } = changedEntries(sheet("competency"), {
      [cellKey("a", "competency")]: "competent",
      [cellKey("b", "competency")]: "maybe",
    });
    expect(entries).toEqual([{ student_id: "a", component: "competency", outcome: "competent" }]);
    expect(problems["b:competency"]).toMatch(/Choose/);
  });
});

describe("wording", () => {
  it("describes schemes and results", () => {
    expect(describeScheme(sheet().scheme)).toBe("Continuous assessment 30%, Final exam 70%. Pass mark 40.");
    expect(describeScheme(sheet("competency").scheme)).toMatch(/^Competency based/);
    expect(describeResult(result(70, "A", true))).toBe("70 (A), pass");
    expect(describeResult(result(null))).toBe("Marks missing");
    expect(describeResult({ ...result(null, "", true, true), outcome: "not_yet_competent" })).toBe(
      "Not yet competent",
    );
  });

  it("names each step of the history", () => {
    expect(describeEvent("draft", "submitted")).toBe("Sent for approval");
    expect(describeEvent("submitted", "draft")).toBe("Returned to the trainer");
    expect(describeEvent("released", "draft")).toBe("Reopened for a correction");
    expect(describeEvent("approved", "released")).toBe("Released to students");
  });

  it("counts passes among complete results and adds weights", () => {
    expect(passCount(sheet())).toEqual({ passed: 1, complete: 1 });
    expect(
      weightTotal([
        { name: "x", weight: 30 },
        { name: "y", weight: Number.NaN },
      ]),
    ).toBe(30);
  });
});
