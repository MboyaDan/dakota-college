import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

/** A grey placeholder block. Hidden from screen readers; wrap in <Loading>. */
export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden className={cn("animate-pulse rounded-control bg-stone", className)} />;
}

export function SkeletonText({ lines = 3 }: { lines?: number }) {
  return (
    <div aria-hidden className="flex flex-col gap-2">
      {Array.from({ length: lines }, (_, i) => (
        <Skeleton key={i} className={cn("h-4", i === lines - 1 ? "w-2/3" : "w-full")} />
      ))}
    </div>
  );
}

/** Marks a region as loading and tells screen readers, while skeletons show the layout. */
export function Loading({ label = "Loading", children }: { label?: string; children: ReactNode }) {
  return (
    <div aria-busy="true">
      <span role="status" className="sr-only">
        {label}…
      </span>
      {children}
    </div>
  );
}
