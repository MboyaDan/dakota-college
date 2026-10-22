import type { Metadata } from "next";
import { NotFoundContent } from "@/components/public/not-found-content";

export const metadata: Metadata = { title: "Page not found", robots: { index: false, follow: true } };

/** Missing programmes, articles and other content inside the public site. */
export default function NotFound() {
  return <NotFoundContent />;
}
