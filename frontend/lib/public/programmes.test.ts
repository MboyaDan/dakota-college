import { describe, expect, it } from "vitest";
import {
  durationText,
  feeText,
  filterProgrammes,
  intakesTakingApplications,
  nairobiToday,
  nextIntake,
  parseFilters,
  tuitionFee,
  upcomingIntakes,
} from "./programmes";
import type { PublicProgramme } from "./types";

function programme(over: Partial<PublicProgramme>): PublicProgramme {
  return {
    slug: "p",
    title: "Programme",
    school_slug: "ict",
    level: "diploma",
    mode: "full_time",
    duration_months: 24,
    duration_note: "",
    min_kcse_grade: null,
    summary: "",
    overview: "",
    what_you_learn: "",
    career_outcomes: "",
    seo_title: "",
    seo_description: "",
    requirements: [],
    intakes: [],
    fees: [],
    published_at: null,
    updated_at: "2026-10-22T00:00:00Z",
    is_sample: false,
    ...over,
  };
}

const list = [
  programme({ slug: "nursing", title: "Diploma in Nursing", school_slug: "medical", min_kcse_grade: "C" }),
  programme({
    slug: "ict-cert",
    title: "Certificate in ICT",
    level: "certificate",
    duration_months: 12,
    min_kcse_grade: "D",
  }),
  programme({
    slug: "food",
    title: "Food Safety",
    level: "short_course",
    duration_months: 1,
    summary: "Hygiene in kitchens",
  }),
];

describe("filterProgrammes", () => {
  it("returns everything with no filters", () => {
    expect(filterProgrammes(list, {})).toHaveLength(3);
  });

  it("filters by school, level and duration band", () => {
    expect(filterProgrammes(list, { school: "medical" }).map((p) => p.slug)).toEqual(["nursing"]);
    expect(filterProgrammes(list, { level: "certificate" }).map((p) => p.slug)).toEqual(["ict-cert"]);
    expect(filterProgrammes(list, { duration: "up-to-6" }).map((p) => p.slug)).toEqual(["food"]);
    expect(filterProgrammes(list, { duration: "7-12" }).map((p) => p.slug)).toEqual(["ict-cert"]);
    expect(filterProgrammes(list, { duration: "13-24" }).map((p) => p.slug)).toEqual(["nursing"]);
  });

  it("shows programmes whose minimum grade the visitor meets", () => {
    // C- meets D and "no minimum", not C.
    expect(filterProgrammes(list, { grade: "C-" }).map((p) => p.slug)).toEqual(["ict-cert", "food"]);
    expect(filterProgrammes(list, { grade: "C" })).toHaveLength(3);
    expect(filterProgrammes(list, { grade: "E" }).map((p) => p.slug)).toEqual(["food"]);
  });

  it("searches every word across title and summary, ignoring case", () => {
    expect(filterProgrammes(list, { q: "HYGIENE kitchens" }).map((p) => p.slug)).toEqual(["food"]);
    expect(filterProgrammes(list, { q: "nursing kitchens" })).toEqual([]);
  });
});

describe("parseFilters", () => {
  it("keeps known values and drops the rest", () => {
    expect(
      parseFilters({ level: "diploma", duration: "7-12", grade: "B+", school: "ict", q: "  nurse " }),
    ).toEqual({
      level: "diploma",
      duration: "7-12",
      grade: "B+",
      school: "ict",
      q: "nurse",
    });
    expect(parseFilters({ level: "phd", duration: "forever", grade: "Z", q: "" })).toEqual({
      level: undefined,
      duration: undefined,
      grade: undefined,
      school: undefined,
      q: undefined,
    });
    expect(parseFilters({ level: ["certificate", "diploma"] }).level).toBe("certificate");
  });
});

describe("display helpers", () => {
  it("writes durations in plain words", () => {
    expect(durationText({ duration_months: 24, duration_note: "" })).toBe("2 years");
    expect(durationText({ duration_months: 12, duration_note: "" })).toBe("1 year");
    expect(durationText({ duration_months: 9, duration_note: "including attachment" })).toBe(
      "9 months (including attachment)",
    );
    expect(durationText({ duration_months: null, duration_note: "" })).toBe("");
  });

  it("writes fees with how often they are paid", () => {
    expect(feeText({ amount_cents: 4500000, frequency: "per_term" })).toBe("KES 45,000.00 per term");
  });

  it("prefers the general tuition line and the first open intake", () => {
    const p = programme({
      fees: [
        {
          intake_id: "i1",
          label: "Tuition (January)",
          category: "tuition",
          amount_cents: 1,
          frequency: "per_term",
          notes: "",
        },
        {
          intake_id: null,
          label: "Tuition",
          category: "tuition",
          amount_cents: 2,
          frequency: "per_term",
          notes: "",
        },
      ],
      intakes: [
        {
          id: "a",
          name: "January",
          starts_on: "2027-01-11",
          applications_open_on: null,
          application_deadline: null,
          status: "planned",
        },
        {
          id: "b",
          name: "May",
          starts_on: "2027-05-03",
          applications_open_on: null,
          application_deadline: null,
          status: "open",
        },
      ],
    });
    expect(tuitionFee(p)?.amount_cents).toBe(2);
    expect(nextIntake(p)?.id).toBe("b");
    expect(upcomingIntakes({ programmes: [p] }).map((u) => u.intake.id)).toEqual(["a", "b"]);
  });
});

describe("intakesTakingApplications", () => {
  const intake = (id: string, over: object) => ({
    id,
    name: id,
    starts_on: "2027-01-11",
    applications_open_on: null,
    application_deadline: null,
    status: "open" as const,
    ...over,
  });
  it("keeps open intakes whose window includes today, deadline day included", () => {
    const p = programme({
      intakes: [
        intake("a", {}),
        intake("planned", { status: "planned" }),
        intake("not-yet", { applications_open_on: "2026-11-01" }),
        intake("closed", { application_deadline: "2026-10-22" }),
        intake("last-day", { application_deadline: "2026-10-23" }),
      ],
    });
    expect(intakesTakingApplications(p, "2026-10-23").map((i) => i.id)).toEqual(["a", "last-day"]);
  });
  it("uses the Nairobi date", () => {
    // 22:30 UTC on 23 October is already 24 October in Nairobi (UTC+3).
    expect(nairobiToday(new Date("2026-10-23T22:30:00Z"))).toBe("2026-10-24");
  });
});
