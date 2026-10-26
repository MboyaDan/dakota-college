import type { Metadata } from "next";
import { StudentRequests } from "@/components/student/pages";

export const metadata: Metadata = { title: "Requests" };

export default function Page() {
  return <StudentRequests />;
}
