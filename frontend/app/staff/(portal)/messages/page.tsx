import type { Metadata } from "next";
import { MessageTemplates } from "@/components/staff/message-templates";

export const metadata: Metadata = { title: "Messages to applicants" };

export default function MessagesPage() {
  return <MessageTemplates />;
}
