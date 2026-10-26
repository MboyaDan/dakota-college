import type { Metadata } from "next";
import { Suspense } from "react";
import { AuthLayout, ResetPasswordForm } from "@/components/staff/auth-forms";

// Linked from the password reset email (D-47) and, for new students, the
// enrolment email (D-141). Never indexed.
export const metadata: Metadata = { title: "Choose a new password", robots: { index: false, follow: false } };

export default function ResetPasswordPage() {
  return (
    <AuthLayout title="Choose a new password">
      <Suspense>
        <ResetPasswordForm />
      </Suspense>
    </AuthLayout>
  );
}
