import type { Metadata, Viewport } from "next";
import { inter, newsreader, plexMono } from "@/lib/fonts";
import { site } from "@/lib/site";
import { resolveSiteUrl } from "@/lib/site-url";
import "./globals.css";

// Supplied social card (docs/brand/dakota-logo), 1200 x 630.
const ogImage = {
  url: "/brand/dakota-og-image.png",
  width: 1200,
  height: 630,
  alt: site.name,
};

export const metadata: Metadata = {
  metadataBase: resolveSiteUrl(process.env.SITE_URL),
  title: site.name,
  // Scaffold only. Stage 6 replaces this with real SEO metadata.
  robots: { index: false, follow: false },
  // Supplied icon files in public/, so browsers that request /favicon.ico
  // directly get the same icon.
  icons: {
    icon: [
      { url: "/favicon.ico", sizes: "any" },
      { url: "/dakota-favicon.svg", type: "image/svg+xml" },
    ],
    apple: [{ url: "/apple-touch-icon-180.png", sizes: "180x180", type: "image/png" }],
  },
  openGraph: {
    type: "website",
    siteName: site.name,
    title: site.name,
    locale: "en_KE",
    images: [ogImage],
  },
  twitter: {
    card: "summary_large_image",
    title: site.name,
    images: [ogImage],
  },
};

export const viewport: Viewport = {
  themeColor: "#0A1F4D",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${newsreader.variable} ${plexMono.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col">{children}</body>
    </html>
  );
}
