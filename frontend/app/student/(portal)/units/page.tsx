import type { Metadata } from "next";
import { StudentUnits } from "@/components/student/pages";

export const metadata: Metadata = { title: "Units" };

export default function Page() {
  return <StudentUnits />;
}
