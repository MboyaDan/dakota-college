import { Briefcase, ChefHat, Monitor, Stethoscope, Waves, type LucideIcon } from "@/components/icons";

// The five schools are confirmed by the owner (brief section 2). Their
// descriptions and programmes are content, managed in the staff portal from
// Stage 5; only names, slugs and visual identity live in code (ADR-09).

export type SchoolKey = "medical" | "hospitality" | "ict" | "business" | "pool";

export type School = {
  key: SchoolKey;
  name: string;
  shortName: string;
  slug: string;
  icon: LucideIcon;
};

export const schools: readonly School[] = [
  {
    key: "medical",
    name: "School of Medical and Health Sciences",
    shortName: "Medical and Health Sciences",
    slug: "medical-and-health-sciences",
    icon: Stethoscope,
  },
  {
    key: "hospitality",
    name: "School of Hospitality and Tourism",
    shortName: "Hospitality and Tourism",
    slug: "hospitality-and-tourism",
    icon: ChefHat,
  },
  { key: "ict", name: "School of ICT", shortName: "ICT", slug: "ict", icon: Monitor },
  { key: "business", name: "School of Business", shortName: "Business", slug: "business", icon: Briefcase },
  {
    key: "pool",
    name: "School of Pool Management",
    shortName: "Pool Management",
    slug: "pool-management",
    icon: Waves,
  },
];

/** Full class strings so Tailwind can see them. Accents never carry meaning alone. */
export const schoolAccent: Record<SchoolKey, { bg: string; border: string; text: string }> = {
  medical: { bg: "bg-school-medical", border: "border-school-medical", text: "text-school-medical" },
  hospitality: {
    bg: "bg-school-hospitality",
    border: "border-school-hospitality",
    text: "text-school-hospitality",
  },
  ict: { bg: "bg-school-ict", border: "border-school-ict", text: "text-school-ict" },
  business: { bg: "bg-school-business", border: "border-school-business", text: "text-school-business" },
  pool: { bg: "bg-school-pool", border: "border-school-pool", text: "text-school-pool" },
};
