import type { Metadata } from "next";
import { SiteChrome } from "@/components/layout/site-chrome";
import { NotFoundContent } from "@/components/public/not-found-content";

export const metadata: Metadata = { title: "Page not found", robots: { index: false, follow: true } };

/** Addresses that match no route. Wrapped in the site shell so visitors can find their way. */
export default function NotFound() {
  return (
    <SiteChrome>
      <NotFoundContent />
    </SiteChrome>
  );
}
