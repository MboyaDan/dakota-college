import type { Metadata } from "next";
import { StudentTimetable } from "@/components/student/pages";

export const metadata: Metadata = { title: "Timetable" };

export default function Page() {
  return <StudentTimetable />;
}
