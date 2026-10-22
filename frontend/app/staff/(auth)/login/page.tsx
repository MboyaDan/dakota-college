import type { Metadata } from "next";
import { Suspense } from "react";
import { AuthLayout, LoginForm } from "@/components/staff/auth-forms";

export const metadata: Metadata = { title: "Sign in" };

export default function LoginPage() {
  return (
    <AuthLayout title="Sign in" intro={<p>Use your college email address and password.</p>}>
      <Suspense>
        <LoginForm />
      </Suspense>
    </AuthLayout>
  );
}
