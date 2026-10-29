import type { Metadata } from "next";
import { Retention } from "@/components/staff/privacy";

export const metadata: Metadata = { title: "Retention" };

export default function RetentionPage() {
  return <Retention />;
}
