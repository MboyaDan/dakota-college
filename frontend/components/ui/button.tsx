import Link from "next/link";
import type { ComponentProps, ReactNode } from "react";
import { LoaderCircle } from "@/components/icons";
import { cn } from "@/lib/cn";

export type ButtonVariant = "primary" | "secondary" | "outline" | "ghost" | "danger" | "inverse";

const base =
  "inline-flex min-h-touch items-center justify-center gap-2 rounded-control px-5 py-2 text-base font-semibold leading-tight no-underline transition-colors disabled:cursor-not-allowed disabled:opacity-60";

const variants: Record<ButtonVariant, string> = {
  // The single main action on a screen: gold fill, navy text (6.6:1).
  primary: "bg-gold text-navy-900 hover:bg-[#b8931f]",
  secondary: "bg-royal text-white hover:bg-navy-900",
  outline: "border-2 border-royal bg-transparent text-royal hover:bg-royal/5",
  ghost: "text-royal underline-offset-4 hover:underline",
  danger: "bg-danger text-white hover:bg-[#8f1c13]",
  // For use on navy surfaces.
  inverse: "border-2 border-white bg-transparent text-white hover:bg-white/10",
};

export function buttonClasses(variant: ButtonVariant = "primary", className?: string) {
  return cn(base, variants[variant], className);
}

type ButtonProps = ComponentProps<"button"> & {
  variant?: ButtonVariant;
  /** Shows a spinner, disables the button and announces busy state. */
  loading?: boolean;
  icon?: ReactNode;
};

export function Button({
  variant = "primary",
  loading = false,
  icon,
  className,
  children,
  disabled,
  type = "button",
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={buttonClasses(variant, className)}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <LoaderCircle aria-hidden className="size-5 animate-spin" /> : icon}
      {children}
    </button>
  );
}

type LinkButtonProps = ComponentProps<typeof Link> & { variant?: ButtonVariant; icon?: ReactNode };

/** A link styled as a button, for navigation such as "Apply now". */
export function LinkButton({ variant = "primary", icon, className, children, ...rest }: LinkButtonProps) {
  return (
    <Link className={buttonClasses(variant, className)} {...rest}>
      {icon}
      {children}
    </Link>
  );
}
