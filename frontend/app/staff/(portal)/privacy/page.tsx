import type { Metadata } from "next";
import { DataRequests } from "@/components/staff/privacy";

export const metadata: Metadata = { title: "Data requests" };

export default function DataRequestsPage() {
  return <DataRequests />;
}
