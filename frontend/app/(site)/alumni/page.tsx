import type { Metadata } from "next";
import { TextPage, textPageMetadata } from "@/components/public/text-page";
import { LinkButton } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export async function generateMetadata(): Promise<Metadata> {
  return textPageMetadata("alumni", "Alumni", "/alumni");
}

export default function AlumniPage() {
  return (
    <TextPage slug="alumni" title="Alumni" crumbs={[{ label: "Home", href: "/" }]}>
      <Card className="max-w-2xl">
        <h2 className="text-2xl font-semibold text-navy-900">Transcripts and certificates</h2>
        <p>
          Former students can ask for a transcript or a copy of a certificate. Send us your request with your
          full name and the year you finished, and the office will reply.
        </p>
        <div>
          <LinkButton href="/contact?about=alumni">Request a transcript or certificate</LinkButton>
        </div>
      </Card>
    </TextPage>
  );
}
