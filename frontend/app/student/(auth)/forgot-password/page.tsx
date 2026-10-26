import type { Metadata } from "next";
import { AuthLayout } from "@/components/staff/auth-forms";
import { StudentForgotForm } from "@/components/student/auth";

export const metadata: Metadata = { title: "Choose a password" };

export default function StudentForgotPage() {
  return (
    <AuthLayout
      portal="student"
      title="Choose a password"
      intro={<p>Enter your student number. We will email a link to the address the college has for you.</p>}
    >
      <StudentForgotForm />
    </AuthLayout>
  );
}
