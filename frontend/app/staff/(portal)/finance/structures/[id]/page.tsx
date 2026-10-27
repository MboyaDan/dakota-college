import { Suspense } from "react";
import { StructureView } from "@/components/staff/finance-structures";
import { Loading, SkeletonText } from "@/components/ui/skeleton";

type Props = PageProps<"/staff/finance/structures/[id]">;

// Ids are only known at request time, so params are read inside Suspense
// (Cache Components). Warnings from copying the published fees come once,
// in the address, right after the structure is started.
async function View({ params, searchParams }: Pick<Props, "params" | "searchParams">) {
  const [{ id }, query] = await Promise.all([params, searchParams]);
  const raw = typeof query.warnings === "string" ? query.warnings : "";
  return <StructureView id={id} warnings={raw ? raw.split("\n") : []} />;
}

export default function StructurePage({ params, searchParams }: Props) {
  return (
    <Suspense
      fallback={
        <Loading label="Loading">
          <SkeletonText lines={6} />
        </Loading>
      }
    >
      <View params={params} searchParams={searchParams} />
    </Suspense>
  );
}
