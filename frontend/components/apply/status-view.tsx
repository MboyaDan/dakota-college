"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Card } from "@/components/ui/card";
import { StatusTag } from "@/components/ui/tag";
import { applicationStatus } from "@/lib/apply/status";
import { formatNairobi } from "@/lib/staff/datetime";
import { InfoRequestPanel, OfferPanel, WithdrawPanel } from "./decision-panels";
import { DocumentList } from "./documents";
import type { Detail } from "./step-kit";

/**
 * Where an application stands once it can no longer be changed: anything
 * the applicant needs to do (send more, reply to an offer), its status,
 * what happens next, every change so far, and the documents sent.
 */
export function StatusView({
  detail,
  id,
  onChange,
  reload,
}: {
  detail: Detail;
  id: string;
  onChange: (d: Detail) => void;
  reload: () => Promise<void>;
}) {
  const pathname = usePathname();
  const st = applicationStatus[detail.status];
  const panel = { id, detail, onChange, reload };
  return (
    <div className="flex flex-col gap-6">
      <InfoRequestPanel {...panel} />
      <OfferPanel {...panel} />
      <section aria-labelledby="status-title" className="flex flex-col gap-3">
        <h2 id="status-title" className="text-2xl font-semibold text-navy-900">
          Where your application is
        </h2>
        <div className="flex flex-wrap items-center gap-3">
          <StatusTag status={st.tone}>{st.label}</StatusTag>
          {detail.submitted_at ? <span>Submitted {formatNairobi(detail.submitted_at)}</span> : null}
        </div>
        <p>
          <strong>What happens next:</strong> {st.next}
        </p>
        <p className="text-sm text-ink-muted">
          Keep your reference number, <span className="font-mono break-all">{detail.reference}</span>, for any
          questions to the admissions office.
        </p>
      </section>

      <section aria-labelledby="history-title" className="flex flex-col gap-3">
        <h2 id="history-title" className="text-xl font-semibold text-navy-900">
          History
        </h2>
        <ol className="flex flex-col gap-3 border-l-2 border-stone pl-4">
          {[...detail.history].reverse().map((h, i) => (
            <li key={i} className="flex flex-col">
              <span className="font-semibold">{applicationStatus[h.status].label}</span>
              <span className="text-sm text-ink-muted">{formatNairobi(h.at)}</span>
              {h.reason ? <span>{h.reason}</span> : null}
            </li>
          ))}
        </ol>
      </section>

      {detail.documents.length > 0 ? (
        <Card className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold text-navy-900">Documents you sent</h2>
          <DocumentList appId={id} docs={detail.documents} />
        </Card>
      ) : null}

      <WithdrawPanel {...panel} />

      <div className="flex flex-wrap gap-x-6 gap-y-2">
        <Link
          href={`${pathname}?id=${id}&step=review`}
          className="inline-flex min-h-touch items-center font-semibold text-royal underline underline-offset-4"
        >
          See your answers
        </Link>
        <Link
          href="/apply/account"
          className="inline-flex min-h-touch items-center font-semibold text-royal underline underline-offset-4"
        >
          Back to your applications
        </Link>
      </div>
    </div>
  );
}
