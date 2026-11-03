import type { Metadata } from "next";
import Link from "next/link";
import { PageHeader } from "@/components/public/page-header";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { StatusTag } from "@/components/ui/tag";
import { applicationStatus, type ApplicationStatus } from "@/lib/apply/status";

// Where "Track your application" in the menu and footer leads. The status
// itself is in the applicant's account, reached by an emailed sign-in link
// (D-96) or the reference number; this page explains the ways in and what
// each stage means. Plain links, so it renders without the API.

export const metadata: Metadata = {
  title: "Track your application",
  description: "See where your application to Dakota Training College has got to.",
};

const stages: ApplicationStatus[] = [
  "submitted",
  "under_review",
  "info_requested",
  "offered",
  "offer_accepted",
  "enrolled",
  "rejected",
  "withdrawn",
  "expired",
  "draft",
];

export default function TrackPage() {
  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Home", href: "/" },
          { label: "Apply", href: "/apply" },
          { label: "Track your application" },
        ]}
        title="Track your application"
        lead="Your application's progress is in your account. There is no password: we email you a link to sign in."
      />
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-10 px-4 py-10 lg:px-6">
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <h2 className="text-xl font-semibold">Sign in with your email</h2>
            <p>Use the email address you applied with. We send you a link that opens your account.</p>
            <div>
              <LinkButton href="/apply/start?next=/apply/account">Email me a sign-in link</LinkButton>
            </div>
          </Card>
          <Card>
            <h2 className="text-xl font-semibold">Use your reference number</h2>
            <p>
              Your reference number looks like <span className="font-mono">DTC-APP-2026-000123</span> and is
              in the email we sent when you started.
            </p>
            <div>
              <LinkButton href="/apply/resume" variant="outline">
                Continue with a reference number
              </LinkButton>
            </div>
          </Card>
        </div>
        <p>
          Already signed in on this device?{" "}
          <Link href="/apply/account" className="font-semibold text-royal underline">
            Go to your account
          </Link>
          .
        </p>

        <section aria-labelledby="stages-title" className="flex flex-col gap-4">
          <h2 id="stages-title" className="text-2xl font-semibold">
            What each stage means
          </h2>
          <p className="max-w-prose">
            We also email you when your application moves on, so check your inbox and spam folder.
          </p>
          <dl className="flex flex-col divide-y divide-stone rounded-card border border-stone bg-white">
            {stages.map((s) => (
              <div key={s} className="flex flex-col gap-2 p-4 sm:flex-row sm:items-center sm:gap-6">
                <dt className="sm:w-56 sm:shrink-0">
                  <StatusTag status={applicationStatus[s].tone}>{applicationStatus[s].label}</StatusTag>
                </dt>
                <dd>{applicationStatus[s].next}</dd>
              </div>
            ))}
          </dl>
        </section>

        <p>
          Questions about your application?{" "}
          <Link href="/contact?about=admissions" className="font-semibold text-royal underline">
            Contact the admissions office
          </Link>
          .
        </p>
      </div>
    </>
  );
}
