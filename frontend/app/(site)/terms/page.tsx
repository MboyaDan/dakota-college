import type { Metadata } from "next";
import { TextPage, textPageMetadata } from "@/components/public/text-page";

export async function generateMetadata(): Promise<Metadata> {
  return textPageMetadata("terms", "Terms of use", "/terms");
}

export default function TermsPage() {
  return <TextPage slug="terms" title="Terms of use" crumbs={[{ label: "Home", href: "/" }]} />;
}
