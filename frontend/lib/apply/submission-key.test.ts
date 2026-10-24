import { beforeEach, describe, expect, it } from "vitest";
import { submissionKey } from "./submission-key";

describe("submissionKey", () => {
  beforeEach(() => sessionStorage.clear());

  it("reuses the key until the attempt is settled, then makes a new one", () => {
    const first = submissionKey.get("app-1");
    expect(first.length).toBeGreaterThanOrEqual(8);
    expect(submissionKey.get("app-1")).toBe(first);
    expect(sessionStorage.getItem("dtc-submit-key:app-1")).toBe(first);
    expect(submissionKey.get("app-2")).not.toBe(first);
    submissionKey.clear("app-1");
    expect(submissionKey.get("app-1")).not.toBe(first);
  });
});
