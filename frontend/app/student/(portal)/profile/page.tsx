import type { Metadata } from "next";
import { StudentProfile } from "@/components/student/pages";

export const metadata: Metadata = { title: "Profile" };

export default function Page() {
  return <StudentProfile />;
}
