import type { Metadata } from "next";
import { Suspense } from "react";
import { DataRequestView } from "@/components/staff/privacy";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

export const metadata: Metadata = { title: "Data request" };

// Request ids are only known at request time, so params are read inside
// Suspense (Cache Components).
async function View({ params }: { params: PageProps<"/staff/privacy/[id]">["params"] }) {
  const { id } = await params;
  return <DataRequestView id={id} />;
}

export default function DataRequestPage({ params }: PageProps<"/staff/privacy/[id]">) {
  return (
    <Suspense
      fallback={
        <Loading label="Loading">
          <SkeletonText lines={6} />
        </Loading>
      }
    >
      <View params={params} />
    </Suspense>
  );
}
