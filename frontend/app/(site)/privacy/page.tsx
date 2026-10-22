import type { Metadata } from "next";
import { TextPage, textPageMetadata } from "@/components/public/text-page";

export async function generateMetadata(): Promise<Metadata> {
  return textPageMetadata("privacy", "Privacy notice", "/privacy");
}

export default function PrivacyPage() {
  return <TextPage slug="privacy" title="Privacy notice" crumbs={[{ label: "Home", href: "/" }]} />;
}
