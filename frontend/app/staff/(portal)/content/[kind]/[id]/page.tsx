import { notFound } from "next/navigation";
import { Suspense } from "react";
import { ContentEditor } from "@/components/staff/content-editor";
import { Loading, SkeletonText } from "@/components/ui/skeleton";
import { isKind } from "@/lib/staff/kinds";

// Item ids are only known at request time, so params are read inside
// Suspense (Cache Components).
async function Editor({ params }: { params: PageProps<"/staff/content/[kind]/[id]">["params"] }) {
  const { kind, id } = await params;
  if (!isKind(kind)) notFound();
  return <ContentEditor kind={kind} id={id} />;
}

export default function EditContentPage({ params }: PageProps<"/staff/content/[kind]/[id]">) {
  return (
    <Suspense
      fallback={
        <Loading label="Loading">
          <SkeletonText lines={6} />
        </Loading>
      }
    >
      <Editor params={params} />
    </Suspense>
  );
}
