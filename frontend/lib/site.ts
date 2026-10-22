// Site-wide facts confirmed by the owner (brief section 2). Contact details,
// registration and everything else the college can change come from site
// settings and content in the API (lib/public), not from code.

export const site = {
  name: "Dakota Training College",
  location: "Makueni, Kenya",
} as const;

/**
 * Search engines may index the site only when SITE_INDEXABLE=1. Off by
 * default, so local, CI and staging builds (which may hold sample content)
 * are never indexed; switched on at launch (Stage 20).
 */
export const indexable = process.env.SITE_INDEXABLE === "1";
