import { notFound } from "next/navigation";
import { ContentList } from "@/components/staff/content-list";
import { isKind, kindOrder } from "@/lib/staff/kinds";

export function generateStaticParams() {
  return kindOrder.map((kind) => ({ kind }));
}

export default async function ContentListPage({ params }: PageProps<"/staff/content/[kind]">) {
  const { kind } = await params;
  if (!isKind(kind)) notFound();
  return <ContentList kind={kind} />;
}
