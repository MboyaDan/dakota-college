import type { ReactNode } from "react";
import { CircleAlert, CircleCheck, CircleX, Info } from "@/components/icons";
import { cn } from "@/lib/cn";

type Tone = "info" | "success" | "warning" | "danger";

const tones: Record<Tone, { box: string; icon: typeof Info; iconClass: string }> = {
  info: { box: "border-royal bg-royal/5", icon: Info, iconClass: "text-royal" },
  success: { box: "border-success bg-success/5", icon: CircleCheck, iconClass: "text-success" },
  warning: { box: "border-warning bg-warning/5", icon: CircleAlert, iconClass: "text-warning" },
  danger: { box: "border-danger bg-danger/5", icon: CircleX, iconClass: "text-danger" },
};

type AlertProps = {
  tone?: Tone;
  title: string;
  children?: ReactNode;
  /**
   * Announce to screen readers when it appears (e.g. after a form submit).
   * Leave off for messages present on page load.
   */
  live?: boolean;
  className?: string;
};

export function Alert({ tone = "info", title, children, live = false, className }: AlertProps) {
  const { box, icon: Icon, iconClass } = tones[tone];
  const role = live ? (tone === "danger" ? "alert" : "status") : undefined;
  return (
    <div role={role} className={cn("flex gap-3 rounded-control border-l-4 p-4", box, className)}>
      <Icon aria-hidden className={cn("mt-0.5 size-5 shrink-0", iconClass)} />
      <div className="flex flex-col gap-1">
        <p className="font-semibold text-ink">{title}</p>
        {children ? <div className="text-ink">{children}</div> : null}
      </div>
    </div>
  );
}
