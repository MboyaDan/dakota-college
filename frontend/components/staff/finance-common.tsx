"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/ui/alert";
import { Loading, SkeletonText } from "@/components/ui/skeleton";
import { StatusTag } from "@/components/ui/tag";
import { errorMessage } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { invoiceStatus, paymentStatus } from "@/lib/finance";

// Shared pieces of the staff finance screens (Stage 14b).

export type FinanceSummary = components["schemas"]["FinanceSummary"];
export type StructureSummary = components["schemas"]["FeeStructureSummary"];
export type Structure = components["schemas"]["FeeStructure"];
export type InvoiceSummary = components["schemas"]["InvoiceSummary"];
export type Invoice = components["schemas"]["Invoice"];
export type Payment = components["schemas"]["Payment"];
export type Account = components["schemas"]["Account"];
export type ArrearsRow = components["schemas"]["ArrearsRow"];
export type StatementEntry = components["schemas"]["StatementEntry"];

/** Loads once per change of deps; reload() fetches again. */
export function useLoaded<T>(load: () => Promise<{ data?: T; error?: unknown }>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  useEffect(() => {
    let active = true;
    void load().then(({ data: d, error: err }) => {
      if (!active) return;
      if (d !== undefined) {
        setData(d);
        setError(null);
      } else setError(errorMessage(err));
    });
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reload when the inputs change
  }, [...deps, tick]);
  return { data, setData, error, reload: () => setTick((t) => t + 1) };
}

export function LoadState({ error, label }: { error: string | null; label: string }) {
  if (error) {
    return (
      <Alert tone="danger" title="Could not load">
        {error}
      </Alert>
    );
  }
  return (
    <Loading label={label}>
      <SkeletonText lines={8} />
    </Loading>
  );
}

export function PaymentStatusTag({ status }: { status: Payment["status"] }) {
  return <StatusTag status={paymentStatus[status].tone}>{paymentStatus[status].label}</StatusTag>;
}

export function InvoiceStatusTag({ status }: { status: InvoiceSummary["status"] }) {
  return <StatusTag status={invoiceStatus[status].tone}>{invoiceStatus[status].label}</StatusTag>;
}

/** A link that downloads a PDF the API makes from records. */
export function PdfLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <a href={href} download className="inline-flex min-h-touch items-center text-royal underline">
      {children}
    </a>
  );
}
