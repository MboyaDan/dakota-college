import { describe, expect, it } from "vitest";
import { safeStudentNext } from "@/components/student/auth";

describe("safeStudentNext", () => {
  it("follows student portal paths only", () => {
    expect(safeStudentNext("/student/units")).toBe("/student/units");
    expect(safeStudentNext(null)).toBe("/student");
    expect(safeStudentNext("/staff")).toBe("/student");
    expect(safeStudentNext("//evil.example/student")).toBe("/student");
    expect(safeStudentNext("https://evil.example/student")).toBe("/student");
    expect(safeStudentNext("/student/login")).toBe("/student");
  });
});
