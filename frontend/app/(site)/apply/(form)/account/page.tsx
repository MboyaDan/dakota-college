import type { Metadata } from "next";
import { ApplicantAccount } from "@/components/apply/applicant-forms";
import { PageHeader } from "@/components/public/page-header";

export const metadata: Metadata = {
  title: "Your account",
  robots: { index: false, follow: false },
};

export default function AccountPage() {
  return (
    <>
      <PageHeader
        crumbs={[{ label: "Home", href: "/" }, { label: "Apply", href: "/apply" }, { label: "Your account" }]}
        title="Your account"
      />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-10 lg:px-6">
        <ApplicantAccount />
      </div>
    </>
  );
}
