import type { Metadata } from "next";
import { Suspense } from "react";
import { ApplicationWizard } from "@/components/apply/wizard";
import { PageHeader } from "@/components/public/page-header";
import { Loading, Skeleton } from "@/components/ui/skeleton";

// The application is chosen by ?id= (and the step by ?step=), so the page
// itself is static and the form loads in the browser.
export const metadata: Metadata = {
  title: "Your application",
  robots: { index: false, follow: false },
};

export default function ApplicationPage() {
  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Your account", href: "/apply/account" },
          { label: "Application" },
        ]}
        title="Your application"
      />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-8 lg:px-6">
        <Suspense
          fallback={
            <Loading label="Loading your application">
              <Skeleton className="h-64 w-full" />
            </Loading>
          }
        >
          <ApplicationWizard />
        </Suspense>
      </div>
    </>
  );
}
