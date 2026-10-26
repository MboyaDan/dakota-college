import { Suspense } from "react";
import { MyClass } from "@/components/staff/teaching";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

// Ids are only known at request time, so params are read inside Suspense
// (Cache Components).
async function View({ params }: { params: PageProps<"/staff/teaching/[id]">["params"] }) {
  const { id } = await params;
  return <MyClass id={id} />;
}

export default function ClassPage({ params }: PageProps<"/staff/teaching/[id]">) {
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
