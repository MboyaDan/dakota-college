import type { Metadata } from "next";
import { SettingsForm } from "@/components/staff/settings-form";

export const metadata: Metadata = { title: "Site settings" };

export default function SettingsPage() {
  return <SettingsForm />;
}
