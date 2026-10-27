import { Suspense } from "react";
import { ResultReview } from "@/components/staff/results";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

// Ids are only known at request time, so params are read inside Suspense
// (Cache Components).
async function View({ params }: { params: PageProps<"/staff/result-releases/[id]">["params"] }) {
  const { id } = await params;
  return <ResultReview id={id} step="release" />;
}

export default function ResultsPage({ params }: PageProps<"/staff/result-releases/[id]">) {
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
