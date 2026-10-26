"use client";

import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { Loading, Skeleton } from "@/components/ui/skeleton";
import { api, setCsrfToken } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

type Session = components["schemas"]["Session"];

type StudentSession = { user: Session["user"]; signOut: () => Promise<void> };

const Context = createContext<StudentSession | null>(null);

/**
 * Loads the student session. Anyone else (no session, or a staff or
 * applicant account) is sent to the student sign-in page and brought back.
 * The API serves only the signed-in student's own data (AC-08).
 */
export function StudentSessionProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [session, setSession] = useState<Session | null>(null);

  useEffect(() => {
    let active = true;
    api.GET("/api/v1/auth/session").then(({ data, response }) => {
      if (!active) return;
      if (data && data.user.kind === "student") {
        setCsrfToken(data.csrf_token);
        setSession(data);
      } else if (data || response.status === 401) {
        router.replace(`/student/login?next=${encodeURIComponent(pathname)}`);
      }
    });
    return () => {
      active = false;
    };
    // Load once per mount; navigation inside the portal keeps the session.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const signOut = useCallback(async () => {
    await api.POST("/api/v1/auth/logout");
    setCsrfToken("");
    router.replace("/student/login");
  }, [router]);

  const value = useMemo(() => session && { user: session.user, signOut }, [session, signOut]);
  if (!value) {
    return (
      <Loading label="Checking your session">
        <div className="mx-auto flex max-w-5xl flex-col gap-4 p-6">
          <Skeleton className="h-10 w-1/3" />
          <Skeleton className="h-40 w-full" />
        </div>
      </Loading>
    );
  }
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useStudent(): StudentSession {
  const v = useContext(Context);
  if (!v) throw new Error("useStudent must be used inside <StudentSessionProvider>");
  return v;
}
