import type { ReactNode } from "react";
import { CircleAlert, CircleCheck, CircleX, Info } from "@/components/icons";
import { cn } from "@/lib/cn";
import { schoolAccent, type SchoolKey } from "@/lib/schools";

const pill = "inline-flex min-h-7 items-center gap-1.5 rounded-full px-3 py-0.5 text-sm font-semibold";

type TagProps = {
  children: ReactNode;
  /** School accent fill with white text (all accents pass 4.5:1). */
  school?: SchoolKey;
  tone?: "neutral" | "navy" | "gold";
  className?: string;
};

export function Tag({ children, school, tone = "neutral", className }: TagProps) {
  const toneClass = school
    ? cn(schoolAccent[school].bg, "text-white")
    : {
        neutral: "bg-stone text-ink",
        navy: "bg-navy-900 text-white",
        gold: "bg-gold text-navy-900",
      }[tone];
  return <span className={cn(pill, toneClass, className)}>{children}</span>;
}

export type Status = "success" | "danger" | "warning" | "info";

const statusStyle: Record<Status, { className: string; icon: typeof Info }> = {
  success: { className: "bg-success text-white", icon: CircleCheck },
  danger: { className: "bg-danger text-white", icon: CircleX },
  warning: { className: "bg-warning text-white", icon: CircleAlert },
  info: { className: "bg-royal text-white", icon: Info },
};

/** Status always shows an icon and a text label, never colour alone. */
export function StatusTag({ status, children }: { status: Status; children: ReactNode }) {
  const { className, icon: Icon } = statusStyle[status];
  return (
    <span className={cn(pill, className)}>
      <Icon aria-hidden className="size-4" />
      {children}
    </span>
  );
}
