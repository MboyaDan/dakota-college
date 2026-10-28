import type { Metadata } from "next";
import { Reports } from "@/components/staff/reports";

export const metadata: Metadata = { title: "Reports" };

export default function ReportsPage() {
  return <Reports />;
}
