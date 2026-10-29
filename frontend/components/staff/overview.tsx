"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { api } from "@/lib/api/client";
import { kindOrder, kinds } from "@/lib/staff/kinds";
import { loadQueue } from "./approvals";
import { useSession } from "./session";

export function Overview() {
  const { user, can } = useSession();
  const [waiting, setWaiting] = useState<number | null>(null);
  const [toApprove, setToApprove] = useState<number | null>(null);
  const [toRelease, setToRelease] = useState<number | null>(null);
  const [enquiries, setEnquiries] = useState<{ new: number; mine: number } | null>(null);
  const [failedMessages, setFailedMessages] = useState(0);
  const [finance, setFinance] = useState<{ pending_payments: number; draft_invoices: number } | null>(null);
  const [applications, setApplications] = useState<{
    new: number;
    mine: number;
    info_requested: number;
    to_enrol: number;
  } | null>(null);

  const [suggestTwoStep, setSuggestTwoStep] = useState(false);

  useEffect(() => {
    // Holders of finance, registrar and admin permissions are asked to turn
    // on two-step sign-in until they do (D-206).
    void api.GET("/api/v1/auth/two-step").then(({ data }) => {
      if (data) setSuggestTwoStep(data.recommended && !data.enabled);
    });
    if (can("content.publish")) void loadQueue().then((q) => setWaiting(q.length));
    if (can("admissions.view")) {
      void api.GET("/api/v1/staff/applications/counts").then(({ data }) => {
        if (data) setApplications(data);
      });
    }
    if (can("academics.results.approve")) {
      void api.GET("/api/v1/staff/result-approvals").then(({ data }) => {
        if (data) setToApprove(data.items.length);
      });
    }
    if (can("academics.results.release")) {
      void api.GET("/api/v1/staff/result-releases").then(({ data }) => {
        if (data) setToRelease(data.items.length);
      });
    }
    if (can("enquiries.view")) {
      void api.GET("/api/v1/staff/enquiries/counts").then(({ data }) => {
        if (data) setEnquiries(data);
      });
    }
    if (can("notifications.manage")) {
      void api.GET("/api/v1/staff/notifications/counts").then(({ data }) => {
        if (data) setFailedMessages(data.failed_recent);
      });
    }
    if (can("finance.view")) {
      void api.GET("/api/v1/staff/finance/summary").then(({ data }) => {
        if (data) setFinance(data);
      });
    }
  }, [can]);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-semibold">Hello, {user.full_name.split(" ")[0]}</h1>
      {suggestTwoStep ? (
        <Alert tone="warning" title="Protect your account with two-step sign-in">
          <p>
            Your role can see or change money, student records or staff accounts. With two-step sign-in, a
            stolen password is not enough to get in.{" "}
            <Link href="/staff/account" className="font-semibold text-royal underline underline-offset-4">
              Set it up now
            </Link>
          </p>
        </Alert>
      ) : null}
      {can("reports.view") ? (
        <Card>
          <h2 className="text-xl font-semibold">Reports</h2>
          <p>
            Admissions, students, results, finance and enquiries for a period you choose
            {can("reports.export") ? ", with spreadsheet downloads" : ""}.
          </p>
          <LinkButton href="/staff/reports" className="self-start">
            See reports
          </LinkButton>
        </Card>
      ) : null}
      {can("academics.teach") ? (
        <Card>
          <h2 className="text-xl font-semibold">My teaching</h2>
          <p>Your classes, when they meet, and who is registered.</p>
          <LinkButton href="/staff/teaching" className="self-start">
            See my classes
          </LinkButton>
        </Card>
      ) : null}
      {can("academics.results.approve") ? (
        <Card>
          <h2 className="text-xl font-semibold">Results to approve</h2>
          <p>
            {toApprove === null
              ? "Checking…"
              : `${plural(toApprove, "class's results wait", "classes' results wait")} for your approval.`}
          </p>
          <LinkButton href="/staff/result-approvals" className="self-start">
            Review results
          </LinkButton>
        </Card>
      ) : null}
      {can("academics.results.release") ? (
        <Card>
          <h2 className="text-xl font-semibold">Results to release</h2>
          <p>
            {toRelease === null
              ? "Checking…"
              : `${plural(toRelease, "class's approved results are", "classes' approved results are")} ready to release.`}
          </p>
          <LinkButton href="/staff/result-releases" className="self-start">
            Release results
          </LinkButton>
        </Card>
      ) : null}
      {can("finance.view") ? (
        <Card>
          <h2 className="text-xl font-semibold">Finance</h2>
          {finance === null ? (
            <p>Checking…</p>
          ) : (
            <ul className="flex flex-col gap-1">
              <li>{plural(finance.pending_payments, "payment waits", "payments wait")} to be confirmed</li>
              <li>{plural(finance.draft_invoices, "draft invoice", "draft invoices")} not issued yet</li>
            </ul>
          )}
          <div className="flex flex-wrap gap-2">
            <LinkButton href="/staff/finance/payments" className="self-start">
              {can("finance.payment.confirm") ? "Check payments" : "See payments"}
            </LinkButton>
            <LinkButton href="/staff/finance" variant="outline" className="self-start">
              Finance
            </LinkButton>
          </div>
        </Card>
      ) : null}
      {can("admissions.view") ? (
        <Card>
          <h2 className="text-xl font-semibold">Applications</h2>
          {applications === null ? (
            <p>Checking…</p>
          ) : (
            <ul className="flex flex-col gap-1">
              <li>{plural(applications.new, "new application", "new applications")} to review</li>
              {can("admissions.review") ? (
                <li>{plural(applications.mine, "application is", "applications are")} with you</li>
              ) : null}
              <li>{plural(applications.info_requested, "applicant", "applicants")} to reply to a request</li>
              <li>{plural(applications.to_enrol, "accepted offer", "accepted offers")} to enrol</li>
            </ul>
          )}
          <div className="flex flex-wrap gap-2">
            <LinkButton href="/staff/applications?status=submitted" className="self-start">
              See new applications
            </LinkButton>
            {can("admissions.review") ? (
              <LinkButton href="/staff/applications?assigned=me" variant="outline" className="self-start">
                Assigned to me
              </LinkButton>
            ) : null}
          </div>
        </Card>
      ) : null}
      {can("enquiries.view") ? (
        <Card>
          <h2 className="text-xl font-semibold">Enquiries</h2>
          <p>
            {enquiries === null
              ? "Checking…"
              : `${enquiries.new === 1 ? "1 new enquiry" : `${enquiries.new} new enquiries`}; ${
                  enquiries.mine === 1 ? "1 open enquiry is" : `${enquiries.mine} open enquiries are`
                } assigned to you.`}
          </p>
          <div className="flex flex-wrap gap-2">
            <LinkButton href="/staff/enquiries?status=new" className="self-start">
              See new enquiries
            </LinkButton>
            <LinkButton href="/staff/enquiries?assigned=me" variant="outline" className="self-start">
              Assigned to me
            </LinkButton>
          </div>
        </Card>
      ) : null}
      {failedMessages > 0 ? (
        <Card>
          <h2 className="text-xl font-semibold">Messages not delivered</h2>
          <p>
            {failedMessages === 1
              ? "1 email could not be sent in the last seven days."
              : `${failedMessages} emails could not be sent in the last seven days.`}
          </p>
          <LinkButton href="/staff/messages/sent?status=failed" className="self-start">
            See failed messages
          </LinkButton>
        </Card>
      ) : null}
      {can("content.publish") ? (
        <Card>
          <h2 className="text-xl font-semibold">Waiting for approval</h2>
          <p>
            {waiting === null
              ? "Checking…"
              : waiting === 0
                ? "Nothing is waiting."
                : `${waiting} ${waiting === 1 ? "item is" : "items are"} waiting for you.`}
          </p>
          {waiting ? (
            <LinkButton href="/staff/approvals" className="self-start">
              Review now
            </LinkButton>
          ) : null}
        </Card>
      ) : null}
      {can("content.read") ? (
        <section aria-labelledby="content-heading" className="flex flex-col gap-3">
          <h2 id="content-heading" className="text-xl font-semibold">
            Website content
          </h2>
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {kindOrder.map((k) => (
              <li key={k}>
                <Link
                  href={`/staff/content/${k}`}
                  className="flex h-full min-h-touch flex-col gap-1 rounded-card border border-stone bg-white p-4 hover:border-royal"
                >
                  <span className="font-semibold text-royal">{kinds[k].title}</span>
                  <span className="text-sm text-ink-muted">{kinds[k].description}</span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      ) : (
        <p>Your account has no website content tasks. Other modules arrive in later releases.</p>
      )}
    </div>
  );
}

function plural(n: number, one: string, many: string) {
  return `${n} ${n === 1 ? one : many}`;
}
