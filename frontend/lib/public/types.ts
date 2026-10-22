import type { components } from "@/lib/api/schema";

type Schemas = components["schemas"];

export type PublicSite = Schemas["PublicSite"];
export type PublicSchool = Schemas["PublicSchool"];
export type PublicProgramme = Schemas["PublicProgramme"];
export type PublicIntake = PublicProgramme["intakes"][number];
export type PublicFee = PublicProgramme["fees"][number];
export type PublicFAQ = Schemas["PublicFAQ"];
export type PublicStatistic = Schemas["PublicStatistic"];
export type PublicEvent = Schemas["PublicEvent"];
export type PublicNews = Schemas["PublicNews"];
export type Level = PublicProgramme["level"];
