import type { Metadata } from "next";
import { Suspense } from "react";
import { ContinueForm } from "@/components/apply/applicant-forms";
import { PageHeader } from "@/components/public/page-header";

// The emailed link lands here with a one-time token in the address. The page
// sends no referrer, so the token never leaks to another site.
export const metadata: Metadata = {
  title: "Continue your application",
  robots: { index: false, follow: false },
  referrer: "no-referrer",
};

export default function ContinuePage() {
  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Apply", href: "/apply" }, { label: "Continue" }]}
        title="Continue your application"
      />
      <div className="mx-auto flex w-full max-w-xl flex-col gap-6 px-4 py-10 lg:px-6">
        <p>Select the button to sign in on this device. The link works once.</p>
        <Suspense fallback={null}>
          <ContinueForm />
        </Suspense>
      </div>
    </>
  );
}
