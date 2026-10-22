import type { Metadata } from "next";
import { Overview } from "@/components/staff/overview";

export const metadata: Metadata = { title: "Overview" };

export default function StaffHome() {
  return <Overview />;
}
