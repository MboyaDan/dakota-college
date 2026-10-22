// WCAG 2.x contrast ratio, used to check the colour tokens in tests and to
// print measured ratios on the /design page.

function channel(c: number) {
  const s = c / 255;
  return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

export function luminance(hex: string): number {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex);
  if (!m) throw new Error(`Not a 6-digit hex colour: ${hex}`);
  const n = parseInt(m[1], 16);
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
}

export function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

export const tokens = {
  navy900: "#0A1F4D",
  royal: "#1E3A8A",
  gold: "#C9A227",
  goldHover: "#B8931F",
  goldText: "#7A5C00",
  ivory: "#FBF8F1",
  ink: "#1B2430",
  inkMuted: "#4A5565",
  stone: "#E5E0D5",
  white: "#FFFFFF",
  success: "#1E7A46",
  danger: "#B42318",
  dangerHover: "#8F1C13",
  warning: "#8A4B00",
  medical: "#0F766E",
  hospitality: "#B45309",
  ict: "#6D28D9",
  business: "#15803D",
  pool: "#0369A1",
} as const;

export type ContrastPair = { fg: keyof typeof tokens; bg: keyof typeof tokens; min: number; use: string };

/** Every text/background pairing the design system uses, with its WCAG minimum. */
export const usedPairs: ContrastPair[] = [
  { fg: "ink", bg: "ivory", min: 4.5, use: "Body text" },
  { fg: "ink", bg: "white", min: 4.5, use: "Body text on cards" },
  { fg: "inkMuted", bg: "ivory", min: 4.5, use: "Hints, secondary text" },
  { fg: "inkMuted", bg: "white", min: 4.5, use: "Hints on cards and fields" },
  { fg: "navy900", bg: "ivory", min: 4.5, use: "Headings" },
  { fg: "royal", bg: "ivory", min: 4.5, use: "Links" },
  { fg: "royal", bg: "white", min: 4.5, use: "Links on cards" },
  { fg: "goldText", bg: "ivory", min: 4.5, use: "Gold-coloured text" },
  { fg: "navy900", bg: "gold", min: 4.5, use: "Apply now button, announcement bar" },
  { fg: "navy900", bg: "goldHover", min: 4.5, use: "Apply now button, hover" },
  { fg: "white", bg: "navy900", min: 4.5, use: "Header and footer text" },
  { fg: "gold", bg: "navy900", min: 3, use: "Focus ring and icons on navy" },
  { fg: "royal", bg: "ivory", min: 3, use: "Focus ring on light" },
  { fg: "white", bg: "royal", min: 4.5, use: "Secondary button" },
  { fg: "white", bg: "success", min: 4.5, use: "Success status" },
  { fg: "white", bg: "danger", min: 4.5, use: "Danger status and button" },
  { fg: "white", bg: "dangerHover", min: 4.5, use: "Danger button, hover" },
  { fg: "white", bg: "warning", min: 4.5, use: "Warning status" },
  { fg: "danger", bg: "white", min: 4.5, use: "Field error text" },
  { fg: "inkMuted", bg: "white", min: 3, use: "Field borders" },
  { fg: "white", bg: "medical", min: 4.5, use: "Medical tag" },
  { fg: "white", bg: "hospitality", min: 4.5, use: "Hospitality tag" },
  { fg: "white", bg: "ict", min: 4.5, use: "ICT tag" },
  { fg: "white", bg: "business", min: 4.5, use: "Business tag" },
  { fg: "white", bg: "pool", min: 4.5, use: "Pool management tag" },
];
