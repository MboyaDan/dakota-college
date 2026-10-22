import type { PublicSite } from "./types";

// Site settings as the pages use them. Every value may be empty until the
// college supplies it (DQ-01); the UI hides anything empty.

export type SiteSettings = {
  phone: string;
  email: string;
  admissionsEmail: string;
  whatsapp: string;
  address: string;
  officeHours: string;
  mapUrl: string;
  directions: string;
  visitingHours: string;
  motto: string;
  mission: string;
  vision: string;
  social: { label: string; href: string }[];
};

export function settingsOf(site: Pick<PublicSite, "settings">): SiteSettings {
  const s = (key: string) => site.settings[key]?.trim() ?? "";
  const social = [
    { label: "Facebook", href: s("social.facebook") },
    { label: "Instagram", href: s("social.instagram") },
    { label: "X", href: s("social.x") },
    { label: "YouTube", href: s("social.youtube") },
  ].filter((l) => l.href.startsWith("https://"));
  return {
    phone: s("contact.phone"),
    email: s("contact.email"),
    admissionsEmail: s("contact.admissions_email"),
    whatsapp: s("contact.whatsapp"),
    address: s("contact.address"),
    officeHours: s("contact.office_hours"),
    mapUrl: s("campus.map_url").startsWith("https://") ? s("campus.map_url") : "",
    directions: s("campus.directions"),
    visitingHours: s("campus.visiting_hours"),
    motto: s("about.motto"),
    mission: s("about.mission"),
    vision: s("about.vision"),
    social,
  };
}

/** "+254 712 345 678" -> "tel:+254712345678". */
export function telHref(phone: string): string {
  return `tel:${phone.replace(/[^\d+]/g, "")}`;
}

/** WhatsApp click-to-chat with a prefilled message (brief 6.2). Empty if no number. */
export function whatsappHref(number: string, text: string): string {
  const digits = number.replace(/\D/g, "");
  if (digits.length < 7) return "";
  return `https://wa.me/${digits}?text=${encodeURIComponent(text)}`;
}
