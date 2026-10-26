"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { api } from "@/lib/api/client";
import { kindOrder, kinds } from "@/lib/staff/kinds";
import { loadQueue } from "./approvals";
import { useSession } from "./session";

export function Overview() {
  const { user, can } = useSession();
  const [waiting, setWaiting] = useState<number | null>(null);
  const [enquiries, setEnquiries] = useState<{ new: number; mine: number } | null>(null);
  const [applications, setApplications] = useState<{
    new: number;
    mine: number;
    info_requested: number;
    to_enrol: number;
  } | null>(null);

  useEffect(() => {
    if (can("content.publish")) void loadQueue().then((q) => setWaiting(q.length));
    if (can("admissions.view")) {
      void api.GET("/api/v1/staff/applications/counts").then(({ data }) => {
        if (data) setApplications(data);
      });
    }
    if (can("enquiries.view")) {
      void api.GET("/api/v1/staff/enquiries/counts").then(({ data }) => {
        if (data) setEnquiries(data);
      });
    }
  }, [can]);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-semibold">Hello, {user.full_name.split(" ")[0]}</h1>
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
