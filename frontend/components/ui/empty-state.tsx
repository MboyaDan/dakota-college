import type { ReactNode } from "react";
import { Inbox, type LucideIcon } from "@/components/icons";

type EmptyStateProps = {
  title: string;
  /** Explain why it is empty and what to do next. */
  description: ReactNode;
  icon?: LucideIcon;
  action?: ReactNode;
};

export function EmptyState({ title, description, icon: Icon = Inbox, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-card border border-dashed border-stone bg-white px-6 py-10 text-center">
      <Icon aria-hidden className="size-10 text-ink-muted" />
      <p className="font-serif text-xl font-semibold text-navy-900">{title}</p>
      <div className="max-w-prose text-ink-muted">{description}</div>
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}
