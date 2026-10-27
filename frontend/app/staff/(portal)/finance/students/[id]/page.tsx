import { Suspense } from "react";
import { StudentAccount } from "@/components/staff/finance-account";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

// Ids are only known at request time, so params are read inside Suspense
// (Cache Components).
async function View({ params }: { params: PageProps<"/staff/finance/students/[id]">["params"] }) {
  const { id } = await params;
  return <StudentAccount id={id} />;
}

export default function AccountPage({ params }: PageProps<"/staff/finance/students/[id]">) {
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
