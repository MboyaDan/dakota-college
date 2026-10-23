"use client";

import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { api, setCsrfToken } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { Loading, Skeleton } from "@/components/ui/skeleton";

type Session = components["schemas"]["Session"];

type SessionValue = {
  user: Session["user"];
  permissions: string[];
  can: (permission: string) => boolean;
  signOut: () => Promise<void>;
};

const SessionContext = createContext<SessionValue | null>(null);

/**
 * Loads the staff session. Without one, sends the person to the login page
 * and brings them back afterwards. The UI hides what a role cannot do, but
 * the API enforces every permission itself (AC-07).
 */
export function SessionProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [session, setSession] = useState<Session | null>(null);

  useEffect(() => {
    let active = true;
    api.GET("/api/v1/auth/session").then(({ data, response }) => {
      if (!active) return;
      // An applicant session is not a staff session: sign in as staff.
      if (data && data.user.kind === "staff") {
        setCsrfToken(data.csrf_token);
        setSession(data);
      } else if (data || response.status === 401) {
        router.replace(`/staff/login?next=${encodeURIComponent(pathname)}`);
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
    router.replace("/staff/login");
  }, [router]);

  const value = useMemo<SessionValue | null>(
    () =>
      session && {
        user: session.user,
        permissions: session.permissions,
        can: (p: string) => session.permissions.includes(p),
        signOut,
      },
    [session, signOut],
  );

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
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const v = useContext(SessionContext);
  if (!v) throw new Error("useSession must be used inside <SessionProvider>");
  return v;
}
