import type { Metadata } from "next";
import { AuthLayout, ForgotPasswordForm } from "@/components/staff/auth-forms";

export const metadata: Metadata = { title: "Reset your password" };

export default function ForgotPasswordPage() {
  return (
    <AuthLayout
      title="Reset your password"
      intro={
        <p>Enter your email address. If it has an account, we will send a link that works for one hour.</p>
      }
    >
      <ForgotPasswordForm />
    </AuthLayout>
  );
}
