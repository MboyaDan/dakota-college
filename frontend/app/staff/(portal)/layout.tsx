import { Suspense } from "react";
import { SessionProvider } from "@/components/staff/session";
import { StaffShell } from "@/components/staff/staff-shell";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { ToastProvider } from "@/components/ui/toast";

// The portal is rendered in the browser after the session check, so the
// whole tree sits inside one Suspense boundary (it reads the URL).
export default function PortalLayout({ children }: { children: React.ReactNode }) {
  return (
    <Suspense
      fallback={
        <Loading label="Loading the staff portal">
          <div className="mx-auto flex max-w-5xl flex-col gap-4 p-6">
            <Skeleton className="h-10 w-1/3" />
            <Skeleton className="h-40 w-full" />
          </div>
        </Loading>
      }
    >
      <SessionProvider>
        <ToastProvider>
          <StaffShell>{children}</StaffShell>
        </ToastProvider>
      </SessionProvider>
    </Suspense>
  );
}
