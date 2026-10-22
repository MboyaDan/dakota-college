import type { Metadata } from "next";
import { TextPage, textPageMetadata } from "@/components/public/text-page";

export async function generateMetadata(): Promise<Metadata> {
  return textPageMetadata("cookies", "Cookie notice", "/cookies");
}

export default function CookiesPage() {
  return <TextPage slug="cookies" title="Cookie notice" crumbs={[{ label: "Home", href: "/" }]} />;
}
