import type { Metadata } from "next";
import { Suspense } from "react";
import { StartForm } from "@/components/apply/applicant-forms";
import { PageHeader } from "@/components/public/page-header";

export const metadata: Metadata = {
  title: "Start or continue your application",
  description: "Get a sign-in link by email to start or continue an application.",
  robots: { index: false, follow: false },
};

export default function StartPage() {
  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Apply", href: "/apply" },
          { label: "Start or continue" },
        ]}
        title="Start or continue your application"
      />
      <div className="mx-auto flex w-full max-w-xl flex-col gap-6 px-4 py-10 lg:px-6">
        <p>
          No password needed. Enter your email address and we will send you a link. Open it on this device to
          start a new application or carry on where you stopped.
        </p>
        <Suspense fallback={null}>
          <StartForm />
        </Suspense>
      </div>
    </>
  );
}
