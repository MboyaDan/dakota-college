import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";
import { schoolAccent, type SchoolKey } from "@/lib/schools";

type CardProps = ComponentProps<"div"> & {
  /** School accent shown as a top border. Decoration only; the school is named in the content. */
  accent?: SchoolKey;
};

export function Card({ accent, className, ...rest }: CardProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-card border border-stone bg-white p-5 shadow-sm",
        accent && cn("border-t-4", schoolAccent[accent].border),
        className,
      )}
      {...rest}
    />
  );
}
