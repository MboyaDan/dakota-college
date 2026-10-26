import { Suspense } from "react";
import { StudentSessionProvider } from "@/components/student/session";
import { StudentShell } from "@/components/student/shell";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { ToastProvider } from "@/components/ui/toast";

// Rendered in the browser after the session check, inside one Suspense
// boundary (it reads the URL), like the staff portal.
export default function StudentPortalLayout({ children }: { children: React.ReactNode }) {
  return (
    <Suspense
      fallback={
        <Loading label="Loading the student portal">
          <div className="mx-auto flex max-w-5xl flex-col gap-4 p-6">
            <Skeleton className="h-10 w-1/3" />
            <Skeleton className="h-40 w-full" />
          </div>
        </Loading>
      }
    >
      <StudentSessionProvider>
        <ToastProvider>
          <StudentShell>{children}</StudentShell>
        </ToastProvider>
      </StudentSessionProvider>
    </Suspense>
  );
}
