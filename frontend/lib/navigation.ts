import { schools } from "./schools";

export type NavLink = { label: string; href: string; description?: string };
export type NavSection = { label: string; links: NavLink[] };

/** A top-level item is either a direct link or a mega-menu with sections. */
export type NavItem =
  | { kind: "link"; label: string; href: string }
  | { kind: "menu"; label: string; sections: NavSection[]; footerLink?: NavLink };

export const mainNav: NavItem[] = [
  {
    kind: "menu",
    label: "Programmes",
    sections: [
      {
        label: "Our schools",
        links: schools.map((s) => ({ label: s.shortName, href: `/schools/${s.slug}` })),
      },
      {
        label: "Find a course",
        links: [
          { label: "Course finder", href: "/programmes", description: "Search and filter every programme." },
          { label: "All schools", href: "/schools" },
        ],
      },
    ],
    footerLink: { label: "Browse all programmes", href: "/programmes" },
  },
  {
    kind: "menu",
    label: "Admissions",
    sections: [
      {
        label: "Apply",
        links: [
          { label: "How to apply", href: "/admissions" },
          { label: "Entry requirements", href: "/admissions/requirements" },
          { label: "Track your application", href: "/apply/track" },
        ],
      },
      {
        label: "Plan",
        links: [
          { label: "Fees and payment", href: "/admissions/fees" },
          { label: "Intake calendar", href: "/admissions/intakes" },
          { label: "Questions and answers", href: "/faqs" },
        ],
      },
    ],
  },
  {
    kind: "menu",
    label: "About",
    sections: [
      {
        label: "The college",
        links: [
          { label: "Our story", href: "/about" },
          { label: "Campus and facilities", href: "/about/campus" },
          { label: "Student life", href: "/student-life" },
        ],
      },
    ],
  },
  { kind: "link", label: "News and events", href: "/news" },
  { kind: "link", label: "Contact", href: "/contact" },
];

export const utilityNav: NavLink[] = [
  { label: "Contact", href: "/contact" },
  { label: "Alumni", href: "/alumni" },
  { label: "Student Portal", href: "/student" },
  { label: "Staff Portal", href: "/staff" },
];

export const applyHref = "/apply";

export const footerNav: NavSection[] = [
  {
    label: "Study",
    links: [
      { label: "Course finder", href: "/programmes" },
      { label: "Schools", href: "/schools" },
      { label: "How to apply", href: "/admissions" },
      { label: "Fees", href: "/admissions/fees" },
    ],
  },
  {
    label: "College",
    links: [
      { label: "About", href: "/about" },
      { label: "News and events", href: "/news" },
      { label: "Alumni", href: "/alumni" },
      { label: "Contact", href: "/contact" },
    ],
  },
  {
    label: "Portals",
    links: [
      { label: "Student Portal", href: "/student" },
      { label: "Staff Portal", href: "/staff" },
      { label: "Track your application", href: "/apply/track" },
    ],
  },
];

export const legalNav: NavLink[] = [
  { label: "Privacy notice", href: "/privacy" },
  { label: "Cookie notice", href: "/cookies" },
  { label: "Terms", href: "/terms" },
  { label: "Accessibility", href: "/accessibility" },
];
