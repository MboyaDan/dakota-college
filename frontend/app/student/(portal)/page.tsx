import type { Metadata } from "next";
import { StudentHome } from "@/components/student/pages";

export const metadata: Metadata = { title: "Home" };

export default function Page() {
  return <StudentHome />;
}
