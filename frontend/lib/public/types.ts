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
export type PublicPage = Schemas["PublicPage"];
export type PublicPhoto = Schemas["PublicPhoto"];
export type PublicFounder = Schemas["PublicFounder"];
export type NewsList = Schemas["NewsList"];
export type NewsArticle = Schemas["NewsArticle"];
export type EventList = Schemas["EventList"];
export type EventDetail = Schemas["EventDetail"];
