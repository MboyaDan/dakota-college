import { describe, expect, it } from "vitest";
import { applicationStatus } from "@/lib/apply/status";
import { ageOn, inboxStatuses, staffStatus } from "./admissions";

describe("staffStatus", () => {
  it("names every status the applicant can see", () => {
    expect(Object.keys(staffStatus).sort()).toEqual(Object.keys(applicationStatus).sort());
  });
  it("filters by every status except draft", () => {
    expect([...inboxStatuses, "draft"].sort()).toEqual(Object.keys(staffStatus).sort());
  });
});

describe("ageOn", () => {
  // 2026-03-10 21:30 UTC is already 11 March in Nairobi.
  const today = new Date("2026-03-10T21:30:00Z");
  it("counts whole years in Nairobi time", () => {
    expect(ageOn("2008-03-11", today)).toBe(18);
    expect(ageOn("2008-03-12", today)).toBe(17);
    expect(ageOn("2000-01-01", today)).toBe(26);
  });
  it("is null for something that is not a date", () => {
    expect(ageOn("", today)).toBeNull();
  });
});
