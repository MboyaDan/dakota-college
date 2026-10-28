import type { Metadata } from "next";
import { SentMessages } from "@/components/staff/sent-messages";

export const metadata: Metadata = { title: "Sent messages" };

export default function SentMessagesPage() {
  return <SentMessages />;
}
