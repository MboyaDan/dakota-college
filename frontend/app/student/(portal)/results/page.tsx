import type { Metadata } from "next";
import { StudentResults } from "@/components/student/results";

export const metadata: Metadata = { title: "Results" };

export default function Page() {
  return <StudentResults />;
}
