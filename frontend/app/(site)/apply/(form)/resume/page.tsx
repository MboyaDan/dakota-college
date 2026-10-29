import type { Metadata } from "next";
import { Suspense } from "react";
import { ResumeForm } from "@/components/apply/applicant-forms";
import { PageHeader } from "@/components/public/page-header";

export const metadata: Metadata = {
  title: "Continue with your reference number",
  description: "Continue an application with its reference number and email address.",
  robots: { index: false, follow: false },
};

export default function ResumePage() {
  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Apply", href: "/apply" },
          { label: "Reference number" },
        ]}
        title="Continue with your reference number"
      />
      <div className="mx-auto flex w-full max-w-xl flex-col gap-6 px-4 py-10 lg:px-6">
        <p>
          Enter the reference number of your application and the email address you used. If they match, we
          will email you a link to continue.
        </p>
        <Suspense fallback={null}>
          <ResumeForm />
        </Suspense>
      </div>
    </>
  );
}
