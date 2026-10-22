import type { Metadata } from "next";
import { ApprovalQueue } from "@/components/staff/approvals";

export const metadata: Metadata = { title: "Waiting for approval" };

export default function ApprovalsPage() {
  return <ApprovalQueue />;
}
