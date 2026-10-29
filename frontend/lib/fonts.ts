import localFont from "next/font/local";

// Fonts come from fontsource packages and are self-hosted by next/font, so
// pages never request Google Fonts. Latin subset only: the site is English.

// Headings use one weight, 600, so one static file (24 KB) replaces the
// variable font (58 KB) on every page's critical path (Stage 18, D-218).
export const newsreader = localFont({
  src: "../node_modules/@fontsource/newsreader/files/newsreader-latin-600-normal.woff2",
  weight: "600",
  variable: "--font-newsreader",
  display: "swap",
  adjustFontFallback: "Times New Roman",
});

export const inter = localFont({
  src: "../node_modules/@fontsource-variable/inter/files/inter-latin-wght-normal.woff2",
  weight: "100 900",
  variable: "--font-inter",
  display: "swap",
  adjustFontFallback: "Arial",
});

export const plexMono = localFont({
  src: [
    {
      path: "../node_modules/@fontsource/ibm-plex-mono/files/ibm-plex-mono-latin-400-normal.woff2",
      weight: "400",
    },
    {
      path: "../node_modules/@fontsource/ibm-plex-mono/files/ibm-plex-mono-latin-500-normal.woff2",
      weight: "500",
    },
  ],
  variable: "--font-plex-mono",
  display: "swap",
  // IDs appear below the fold; do not compete with body and heading fonts.
  preload: false,
});
