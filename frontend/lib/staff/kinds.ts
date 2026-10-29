import type { components } from "@/lib/api/schema";

// One description per content type drives the list, the form and the
// preview, so every type shares one set of screens.

export type ContentKind = components["parameters"]["Kind"];
type Option = { value: string; label: string };

export type FieldSpec =
  | {
      type: "text" | "textarea" | "slug" | "link";
      name: string;
      label: string;
      hint?: string;
      required?: boolean;
      rows?: number;
      from?: string;
    }
  | { type: "number"; name: string; label: string; hint?: string; required?: boolean; nullable?: boolean }
  | {
      type: "select";
      name: string;
      label: string;
      hint?: string;
      options: Option[];
      nullable?: boolean;
      required?: boolean;
    }
  | {
      type: "date" | "datetime";
      name: string;
      label: string;
      hint?: string;
      nullable?: boolean;
      required?: boolean;
    }
  | { type: "checkbox"; name: string; label: string; hint?: string }
  | { type: "money"; name: string; label: string; hint?: string; required?: boolean }
  | { type: "image"; name: string; label: string; hint?: string; required?: boolean }
  | {
      type: "ref";
      name: string;
      label: string;
      hint?: string;
      refKind: ContentKind;
      nullable?: boolean;
      required?: boolean;
    }
  | {
      type: "list";
      name: string;
      label: string;
      hint?: string;
      itemLabel: string;
      fields: FieldSpec[];
      newItem: () => Record<string, unknown>;
    }
  | { type: "nullableText"; name: string; label: string; hint?: string };

export type KindConfig = {
  kind: ContentKind;
  title: string;
  singular: string;
  description: string;
  fields: FieldSpec[];
  empty: () => Record<string, unknown>;
};

const sortOrder: FieldSpec = {
  type: "number",
  name: "sort_order",
  label: "Order",
  hint: "Lower numbers show first.",
};

/** An optional photo and its description for screen readers. */
function photoFields(label: string, hint: string): FieldSpec[] {
  return [
    { type: "image", name: "photo_file_id", label, hint },
    {
      type: "text",
      name: "photo_alt",
      label: "Describe the photo",
      hint: "For people who cannot see it. Required when there is a photo.",
    },
  ];
}

const levels: Option[] = [
  { value: "short_course", label: "Short course" },
  { value: "artisan", label: "Artisan" },
  { value: "certificate", label: "Certificate" },
  { value: "diploma", label: "Diploma" },
  { value: "higher_diploma", label: "Higher diploma" },
];

const grades = ["A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D+", "D", "D-", "E"].map((g) => ({
  value: g,
  label: g,
}));

export const kinds: Record<ContentKind, KindConfig> = {
  programmes: {
    kind: "programmes",
    title: "Programmes",
    singular: "programme",
    description:
      "Courses in the course finder. Every published programme shows fees, entry requirements and intakes.",
    empty: () => ({
      school_id: "",
      slug: "",
      title: "",
      level: "certificate",
      mode: "full_time",
      duration_months: null,
      duration_note: "",
      min_kcse_grade: null,
      summary: "",
      overview: "",
      what_you_learn: "",
      career_outcomes: "",
      assessment_model: "marks",
      seo_title: "",
      seo_description: "",
      sort_order: 0,
      requirements: [],
      intakes: [],
      fees: [],
    }),
    fields: [
      { type: "ref", name: "school_id", label: "School", refKind: "schools", required: true },
      {
        type: "text",
        name: "title",
        label: "Title",
        required: true,
        hint: "For example: Diploma in Community Health.",
      },
      { type: "slug", name: "slug", label: "Web address", required: true, from: "title" },
      { type: "select", name: "level", label: "Level", options: levels, required: true },
      {
        type: "select",
        name: "mode",
        label: "Mode of study",
        required: true,
        options: [
          { value: "full_time", label: "Full time" },
          { value: "part_time", label: "Part time" },
          { value: "blended", label: "Blended" },
        ],
      },
      { type: "number", name: "duration_months", label: "Duration in months", nullable: true },
      {
        type: "text",
        name: "duration_note",
        label: "Duration note",
        hint: "Optional, for example: includes 3 months of attachment.",
      },
      {
        type: "select",
        name: "min_kcse_grade",
        label: "Minimum KCSE mean grade",
        options: grades,
        nullable: true,
      },
      {
        type: "textarea",
        name: "summary",
        label: "Summary",
        rows: 3,
        hint: "One or two sentences for the course finder.",
      },
      { type: "textarea", name: "what_you_learn", label: "What you will learn", rows: 6 },
      { type: "textarea", name: "overview", label: "Overview", rows: 6 },
      { type: "textarea", name: "career_outcomes", label: "Career outcomes", rows: 4 },
      {
        type: "select",
        name: "assessment_model",
        label: "Assessment",
        options: [
          { value: "marks", label: "Marks and grades" },
          { value: "competency", label: "Competency based" },
          { value: "both", label: "Both" },
        ],
      },
      {
        type: "list",
        name: "requirements",
        label: "Entry requirements",
        itemLabel: "Requirement",
        newItem: () => ({ kind: "academic", requirement: "" }),
        fields: [
          {
            type: "select",
            name: "kind",
            label: "Type",
            options: [
              { value: "academic", label: "Academic" },
              { value: "document", label: "Document" },
              { value: "other", label: "Other" },
            ],
          },
          { type: "text", name: "requirement", label: "Requirement", required: true },
        ],
      },
      {
        type: "list",
        name: "fees",
        label: "Fees",
        hint: "Every line shows on the programme page.",
        itemLabel: "Fee line",
        newItem: () => ({
          label: "",
          category: "tuition",
          amount_cents: 0,
          frequency: "per_term",
          notes: "",
        }),
        fields: [
          { type: "text", name: "label", label: "Label", required: true },
          {
            type: "select",
            name: "category",
            label: "Category",
            options: ["tuition", "registration", "assessment", "caution", "other"].map((v) => ({
              value: v,
              label: v[0].toUpperCase() + v.slice(1),
            })),
          },
          { type: "money", name: "amount_cents", label: "Amount (KES)", required: true },
          {
            type: "select",
            name: "frequency",
            label: "Paid",
            options: [
              { value: "once", label: "Once" },
              { value: "per_term", label: "Per term" },
              { value: "per_year", label: "Per year" },
              { value: "whole_programme", label: "For the whole programme" },
            ],
          },
          { type: "text", name: "notes", label: "Notes" },
        ],
      },
      {
        type: "list",
        name: "intakes",
        label: "Intakes",
        hint: "Leave empty if dates are not decided. Only real dates are shown on the site.",
        itemLabel: "Intake",
        newItem: () => ({
          id: null,
          name: "",
          starts_on: "",
          applications_open_on: null,
          application_deadline: null,
          capacity: null,
          status: "planned",
        }),
        fields: [
          { type: "text", name: "name", label: "Name", required: true, hint: "For example: January 2027." },
          { type: "date", name: "starts_on", label: "Starts on", required: true },
          { type: "date", name: "applications_open_on", label: "Applications open", nullable: true },
          { type: "date", name: "application_deadline", label: "Application deadline", nullable: true },
          { type: "number", name: "capacity", label: "Places", nullable: true },
          {
            type: "select",
            name: "status",
            label: "Status",
            options: [
              { value: "planned", label: "Planned" },
              { value: "open", label: "Open for applications" },
              { value: "closed", label: "Closed" },
            ],
          },
        ],
      },
      {
        type: "text",
        name: "seo_title",
        label: "Search engine title",
        hint: "Optional. Up to 70 characters.",
      },
      {
        type: "textarea",
        name: "seo_description",
        label: "Search engine description",
        rows: 2,
        hint: "Optional. Up to 170 characters.",
      },
      sortOrder,
    ],
  },
  schools: {
    kind: "schools",
    title: "Schools",
    singular: "school",
    description: "The five schools and their pages.",
    empty: () => ({
      slug: "",
      name: "",
      short_name: "",
      accent_key: "medical",
      summary: "",
      description: "",
      head_message: "",
      sort_order: 0,
      photo_file_id: null,
      photo_alt: "",
      photo_consent_confirmed: false,
    }),
    fields: [
      { type: "text", name: "name", label: "Name", required: true },
      {
        type: "text",
        name: "short_name",
        label: "Short name",
        required: true,
        hint: "Used in menus, for example: ICT.",
      },
      { type: "slug", name: "slug", label: "Web address", required: true, from: "short_name" },
      {
        type: "select",
        name: "accent_key",
        label: "Colour",
        options: [
          { value: "medical", label: "Teal (medical)" },
          { value: "hospitality", label: "Amber (hospitality)" },
          { value: "ict", label: "Violet (ICT)" },
          { value: "business", label: "Green (business)" },
          { value: "pool", label: "Blue (pool)" },
        ],
      },
      { type: "textarea", name: "summary", label: "Summary", rows: 3 },
      { type: "textarea", name: "description", label: "Description", rows: 8 },
      { type: "textarea", name: "head_message", label: "Message from the head of school", rows: 5 },
      ...photoFields(
        "Photo",
        "Optional. Shown behind the school's card on the home page and at the top of its page. Use a wide, real photo of this school's training. Without one, the card uses the school's colour.",
      ),
      {
        type: "checkbox",
        name: "photo_consent_confirmed",
        label: "Everyone recognisable in this photo gave written consent",
        hint: "Required to publish with a photo.",
      },
      sortOrder,
    ],
  },
  pages: {
    kind: "pages",
    title: "Pages",
    singular: "page",
    description: "Text pages such as student life, alumni, how to apply and the legal notices.",
    empty: () => ({ slug: "", title: "", summary: "", body: "", seo_title: "", seo_description: "" }),
    fields: [
      { type: "text", name: "title", label: "Title", required: true },
      {
        type: "slug",
        name: "slug",
        label: "Web address",
        required: true,
        from: "title",
        hint: "The site shows these addresses: admissions (how to apply), student-life, alumni, privacy, cookies, terms, accessibility.",
      },
      {
        type: "textarea",
        name: "summary",
        label: "Summary",
        rows: 2,
        hint: "One or two sentences under the title.",
      },
      {
        type: "textarea",
        name: "body",
        label: "Page text",
        rows: 18,
        required: true,
        hint: 'Leave a blank line between paragraphs. Start a line with "## " for a heading and "- " for a list item.',
      },
      {
        type: "text",
        name: "seo_title",
        label: "Search engine title",
        hint: "Up to 70 characters. Leave empty to use the title.",
      },
      {
        type: "textarea",
        name: "seo_description",
        label: "Search engine description",
        rows: 2,
        hint: "Up to 170 characters. Leave empty to use the summary.",
      },
    ],
  },
  news: {
    kind: "news",
    title: "News",
    singular: "news article",
    description: "Articles on the news page.",
    empty: () => ({
      slug: "",
      title: "",
      summary: "",
      body: "",
      cover_file_id: null,
      cover_alt: "",
      seo_title: "",
      seo_description: "",
    }),
    fields: [
      { type: "text", name: "title", label: "Title", required: true },
      { type: "slug", name: "slug", label: "Web address", required: true, from: "title" },
      { type: "textarea", name: "summary", label: "Summary", rows: 3 },
      {
        type: "textarea",
        name: "body",
        label: "Article",
        rows: 14,
        hint: "Leave a blank line between paragraphs.",
      },
      { type: "image", name: "cover_file_id", label: "Cover photo" },
      {
        type: "text",
        name: "cover_alt",
        label: "Describe the photo",
        hint: "Read aloud to people who cannot see it. Required with a photo.",
      },
      { type: "text", name: "seo_title", label: "Search engine title" },
      { type: "textarea", name: "seo_description", label: "Search engine description", rows: 2 },
    ],
  },
  events: {
    kind: "events",
    title: "Events",
    singular: "event",
    description: "Open days, graduations and other dated events.",
    empty: () => ({ slug: "", title: "", summary: "", body: "", location: "", starts_at: "", ends_at: null }),
    fields: [
      { type: "text", name: "title", label: "Title", required: true },
      { type: "slug", name: "slug", label: "Web address", required: true, from: "title" },
      { type: "datetime", name: "starts_at", label: "Starts", required: true, hint: "Nairobi time." },
      { type: "datetime", name: "ends_at", label: "Ends", nullable: true, hint: "Nairobi time." },
      { type: "text", name: "location", label: "Location" },
      { type: "textarea", name: "summary", label: "Summary", rows: 3 },
      { type: "textarea", name: "body", label: "Details", rows: 8 },
    ],
  },
  faqs: {
    kind: "faqs",
    title: "FAQs",
    singular: "question",
    description: "Questions and answers on the FAQ page.",
    empty: () => ({ category_id: "", programme_id: null, question: "", answer: "", sort_order: 0 }),
    fields: [
      { type: "ref", name: "category_id", label: "Category", refKind: "faq-categories", required: true },
      { type: "text", name: "question", label: "Question", required: true },
      { type: "textarea", name: "answer", label: "Answer", rows: 6, required: true },
      {
        type: "ref",
        name: "programme_id",
        label: "Only for this programme",
        refKind: "programmes",
        nullable: true,
        hint: "Optional.",
      },
      sortOrder,
    ],
  },
  "faq-categories": {
    kind: "faq-categories",
    title: "FAQ categories",
    singular: "category",
    description: "Groups of questions. Changes apply at once and need a publisher.",
    empty: () => ({ slug: "", name: "", sort_order: 0 }),
    fields: [
      { type: "text", name: "name", label: "Name", required: true },
      { type: "slug", name: "slug", label: "Web address", required: true, from: "name" },
      sortOrder,
    ],
  },
  banners: {
    kind: "banners",
    title: "Announcements",
    singular: "announcement",
    description: "The bar above the header. It shows only between its start and end.",
    empty: () => ({ message: "", link_label: null, link_href: null, starts_at: "", ends_at: "" }),
    fields: [
      { type: "text", name: "message", label: "Message", required: true, hint: "Up to 200 characters." },
      {
        type: "nullableText",
        name: "link_label",
        label: "Link text",
        hint: "Optional, for example: Apply now.",
      },
      {
        type: "nullableText",
        name: "link_href",
        label: "Link",
        hint: "A page like /admissions, or a full https:// address.",
      },
      { type: "datetime", name: "starts_at", label: "Show from", required: true, hint: "Nairobi time." },
      { type: "datetime", name: "ends_at", label: "Show until", required: true, hint: "Nairobi time." },
    ],
  },
  gallery: {
    kind: "gallery",
    title: "Gallery",
    singular: "photo",
    description: "Campus, student life and alumni photos. Photos of people need written consent.",
    empty: () => ({
      file_id: "",
      caption: "",
      alt_text: "",
      consent_confirmed: false,
      consent_note: "",
      home_hero: false,
      alumni: false,
      sort_order: 0,
    }),
    fields: [
      { type: "image", name: "file_id", label: "Photo", required: true },
      { type: "text", name: "alt_text", label: "Describe the photo", required: true },
      { type: "text", name: "caption", label: "Caption" },
      {
        type: "checkbox",
        name: "consent_confirmed",
        label: "Everyone recognisable in this photo gave written consent",
        hint: "Required to publish.",
      },
      { type: "textarea", name: "consent_note", label: "Where the consent forms are kept", rows: 2 },
      {
        type: "checkbox",
        name: "home_hero",
        label: "Use this photo at the top of the home page",
        hint: "Choose a wide photo of the campus or training. If several photos are chosen, the most recently changed one is used.",
      },
      {
        type: "checkbox",
        name: "alumni",
        label: "Show this photo on the Alumni page",
        hint: "For photos of former students, such as a graduation or a reunion. Up to six photos are shown, in sort order.",
      },
      sortOrder,
    ],
  },
  founders: {
    kind: "founders",
    title: "Founders",
    singular: "founder",
    description:
      "The people who founded the college, on the founders page and the home page. Each profile needs the person's written consent.",
    empty: () => ({
      name: "",
      role_title: "",
      intro: "",
      story: "",
      photo_file_id: null,
      photo_alt: "",
      consent_confirmed: false,
      consent_note: "",
      sort_order: 0,
    }),
    fields: [
      { type: "text", name: "name", label: "Full name", required: true },
      { type: "text", name: "role_title", label: "Title", hint: "For example: Founder and Director." },
      {
        type: "textarea",
        name: "intro",
        label: "Short introduction",
        rows: 2,
        hint: "One or two sentences for the home page. Up to 300 characters.",
      },
      {
        type: "textarea",
        name: "story",
        label: "Their story",
        rows: 12,
        required: true,
        hint: 'Why they started the college and what it stands for. Leave a blank line between paragraphs. Start a line with "## " for a heading.',
      },
      ...photoFields("Portrait", "Optional. A real, recent photo, ideally upright (portrait)."),
      {
        type: "checkbox",
        name: "consent_confirmed",
        label: "The founder agreed in writing to this profile and photo being published",
        hint: "Required to publish.",
      },
      { type: "textarea", name: "consent_note", label: "Where the consent form is kept", rows: 2 },
      sortOrder,
    ],
  },
  statistics: {
    kind: "statistics",
    title: "Statistics",
    singular: "statistic",
    description: "Figures such as graduate numbers. Shown only when verified against evidence.",
    empty: () => ({ label: "", value: "", source_note: "", verified_on: null, sort_order: 0 }),
    fields: [
      { type: "text", name: "value", label: "Figure", required: true, hint: "For example: 1,200." },
      {
        type: "text",
        name: "label",
        label: "What it counts",
        required: true,
        hint: "For example: graduates since 2020.",
      },
      {
        type: "textarea",
        name: "source_note",
        label: "Evidence",
        rows: 2,
        hint: "Which report or record proves it. Required to publish.",
      },
      {
        type: "date",
        name: "verified_on",
        label: "Checked on",
        nullable: true,
        hint: "The date someone checked the evidence. Required to publish.",
      },
      sortOrder,
    ],
  },
  accreditations: {
    kind: "accreditations",
    title: "Accreditation",
    singular: "accreditation",
    description: "Registration and accreditation details. Shown only when verified.",
    empty: () => ({
      body: "",
      reference: "",
      description: "",
      evidence_note: "",
      verified_on: null,
      sort_order: 0,
    }),
    fields: [
      { type: "text", name: "body", label: "Issued by", required: true, hint: "For example: TVETA." },
      { type: "text", name: "reference", label: "Registration number", required: true },
      { type: "textarea", name: "description", label: "Description", rows: 3 },
      {
        type: "textarea",
        name: "evidence_note",
        label: "Evidence",
        rows: 2,
        hint: "Which certificate proves it. Required to publish.",
      },
      {
        type: "date",
        name: "verified_on",
        label: "Checked on",
        nullable: true,
        hint: "Required to publish.",
      },
      sortOrder,
    ],
  },
};

export const kindOrder: ContentKind[] = [
  "programmes",
  "schools",
  "news",
  "events",
  "faqs",
  "faq-categories",
  "banners",
  "gallery",
  "statistics",
  "accreditations",
  "pages",
  "founders",
];

export function isKind(v: string): v is ContentKind {
  return v in kinds;
}

/** Turns a title into a web address, e.g. "Diploma in ICT" -> "diploma-in-ict". */
export function slugify(text: string): string {
  return text
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 120);
}
