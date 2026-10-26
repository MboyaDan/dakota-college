import type { Metadata } from "next";
import { StudentNotices } from "@/components/student/pages";

export const metadata: Metadata = { title: "Notices" };

export default function Page() {
  return <StudentNotices />;
}
