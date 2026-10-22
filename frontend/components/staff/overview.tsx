"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { kindOrder, kinds } from "@/lib/staff/kinds";
import { loadQueue } from "./approvals";
import { useSession } from "./session";

export function Overview() {
  const { user, can } = useSession();
  const [waiting, setWaiting] = useState<number | null>(null);

  useEffect(() => {
    if (can("content.publish")) void loadQueue().then((q) => setWaiting(q.length));
  }, [can]);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-semibold">Hello, {user.full_name.split(" ")[0]}</h1>
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
        <p>Your account has no website content tasks yet. Other modules arrive in later releases.</p>
      )}
    </div>
  );
}
