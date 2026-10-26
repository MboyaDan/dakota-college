import { Suspense } from "react";
import { StudentRecord } from "@/components/staff/students";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

// Ids are only known at request time, so params are read inside Suspense
// (Cache Components).
async function View({ params }: { params: PageProps<"/staff/students/[id]">["params"] }) {
  const { id } = await params;
  return <StudentRecord id={id} />;
}

export default function StudentPage({ params }: PageProps<"/staff/students/[id]">) {
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
