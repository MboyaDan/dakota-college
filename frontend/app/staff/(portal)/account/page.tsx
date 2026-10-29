import type { Metadata } from "next";
import { AccountSecurity } from "@/components/staff/account-security";

export const metadata: Metadata = { title: "Your account" };

export default function AccountPage() {
  return <AccountSecurity />;
}
