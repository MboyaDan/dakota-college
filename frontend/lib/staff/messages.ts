// Preview of notification templates in the staff portal. The backend fills
// the same {placeholder} names when it sends (internal/notify).

export const sampleValues: Record<string, string> = {
  name: "Test Applicant 001",
  link: "https://dakota.example/apply/continue?token=sample",
  minutes: "30",
  reference: "DTC-APP-2026-000123",
  programme: "Diploma in Sample Studies (Sample)",
};

export function renderTemplate(text: string, values: Record<string, string> = sampleValues): string {
  return text.replace(/\{([a-z_]+)\}/g, (m, name: string) => values[name] ?? m);
}

/** Placeholders used in the text that the template does not offer. */
export function unknownPlaceholders(text: string, allowed: string[]): string[] {
  const found = [...text.matchAll(/\{([a-z_]+)\}/g)].map((m) => m[1]);
  return [...new Set(found.filter((n) => !allowed.includes(n)))];
}
