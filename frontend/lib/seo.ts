import type { Metadata } from "next";
import { site } from "./site";

// Supplied social card (docs/brand/dakota-logo), 1200 x 630.
export const ogImage = {
  url: "/brand/dakota-og-image.png",
  width: 1200,
  height: 630,
  alt: site.name,
};

/**
 * Title, description, canonical URL and social cards for a public page.
 * Next.js replaces (does not merge) a parent's openGraph and twitter objects,
 * so every page sets them in full here, image included.
 */
export function pageMetadata({
  title,
  description,
  path,
}: {
  title: string;
  description: string;
  path: string;
}): Metadata {
  return {
    title,
    description,
    alternates: { canonical: path },
    openGraph: {
      type: "website",
      siteName: site.name,
      locale: "en_KE",
      title,
      description,
      url: path,
      images: [ogImage],
    },
    twitter: { card: "summary_large_image", title, description, images: [ogImage] },
  };
}
