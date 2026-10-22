import { Suspense } from "react";
import { EnquiryView } from "@/components/staff/enquiries";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

// Enquiry ids are only known at request time, so params are read inside
// Suspense (Cache Components).
async function View({ params }: { params: PageProps<"/staff/enquiries/[id]">["params"] }) {
  const { id } = await params;
  return <EnquiryView id={id} />;
}

export default function EnquiryPage({ params }: PageProps<"/staff/enquiries/[id]">) {
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
