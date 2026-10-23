import { describe, expect, it } from "vitest";
import { applicationStatus } from "./status";

describe("applicationStatus", () => {
  it("covers every state in brief section 8 with a label and a next step", () => {
    const states = [
      "draft",
      "submitted",
      "under_review",
      "info_requested",
      "offered",
      "offer_accepted",
      "enrolled",
      "rejected",
      "withdrawn",
      "expired",
    ];
    expect(Object.keys(applicationStatus).sort()).toEqual([...states].sort());
    for (const s of Object.values(applicationStatus)) {
      expect(s.label).not.toBe("");
      expect(s.next).toMatch(/\.$/);
    }
  });
});
