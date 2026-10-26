import type { Metadata } from "next";
import { Suspense } from "react";
import { AuthLayout } from "@/components/staff/auth-forms";
import { StudentLoginForm } from "@/components/student/auth";

export const metadata: Metadata = { title: "Sign in" };

export default function StudentLoginPage() {
  return (
    <AuthLayout
      portal="student"
      title="Sign in"
      intro={<p>Use your student number and the password you chose from your enrolment email.</p>}
    >
      <Suspense>
        <StudentLoginForm />
      </Suspense>
    </AuthLayout>
  );
}
