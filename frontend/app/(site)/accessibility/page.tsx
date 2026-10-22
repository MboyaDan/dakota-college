import type { Metadata } from "next";
import { TextPage, textPageMetadata } from "@/components/public/text-page";

export async function generateMetadata(): Promise<Metadata> {
  return textPageMetadata("accessibility", "Accessibility statement", "/accessibility");
}

export default function AccessibilityPage() {
  return (
    <TextPage slug="accessibility" title="Accessibility statement" crumbs={[{ label: "Home", href: "/" }]} />
  );
}
