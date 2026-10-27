import { Suspense } from "react";
import { ClassResults } from "@/components/staff/results";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

// Ids are only known at request time, so params are read inside Suspense
// (Cache Components).
async function View({ params }: { params: PageProps<"/staff/teaching/[id]/results">["params"] }) {
  const { id } = await params;
  return <ClassResults id={id} />;
}

export default function ResultsPage({ params }: PageProps<"/staff/teaching/[id]/results">) {
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
