import { notFound } from "next/navigation";
import { ContentEditor } from "@/components/staff/content-editor";
import { isKind, kindOrder } from "@/lib/staff/kinds";

export function generateStaticParams() {
  return kindOrder.map((kind) => ({ kind }));
}

export default async function NewContentPage({ params }: PageProps<"/staff/content/[kind]/new">) {
  const { kind } = await params;
  if (!isKind(kind)) notFound();
  return <ContentEditor kind={kind} />;
}
