import { Suspense } from "react";
import { OfferingView } from "@/components/staff/offerings";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

// Ids are only known at request time, so params are read inside Suspense
// (Cache Components).
async function View({ params }: { params: PageProps<"/staff/offerings/[id]">["params"] }) {
  const { id } = await params;
  return <OfferingView id={id} />;
}

export default function OfferingPage({ params }: PageProps<"/staff/offerings/[id]">) {
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
