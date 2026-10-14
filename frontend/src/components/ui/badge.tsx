import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const badgeVariants = cva("inline-flex h-6 items-center justify-center rounded-full px-2.5 text-xs font-medium leading-none whitespace-nowrap", {
  variants: {
    variant: {
      default: "bg-secondary text-secondary-foreground",
      success: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300",
      warning: "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300",
      destructive: "bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300",
      outline: "border bg-background text-foreground",
    },
  },
  defaultVariants: {
    variant: "default",
  },
});

export function Badge({
  className,
  variant,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof badgeVariants>) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}
