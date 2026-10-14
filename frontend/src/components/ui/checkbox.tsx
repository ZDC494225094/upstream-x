import * as React from "react";
import { Check } from "lucide-react";

import { cn } from "@/lib/utils";

interface CheckboxProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "type"> {
  label?: React.ReactNode;
}

const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, checked, onChange, disabled, label, id, ...props }, ref) => {
    const checkboxId = id ?? React.useId();

    return (
      <label
        htmlFor={checkboxId}
        className={cn(
          "inline-flex items-center gap-3 text-sm",
          disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer",
          className,
        )}
      >
        <span className="relative flex h-4 w-4 shrink-0 items-center justify-center">
          <input
            {...props}
            ref={ref}
            id={checkboxId}
            type="checkbox"
            checked={checked}
            onChange={onChange}
            disabled={disabled}
            className="peer sr-only"
          />
          <span className="flex h-4 w-4 items-center justify-center rounded-[4px] border border-input bg-background transition peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-checked:border-primary peer-checked:bg-primary" aria-hidden="true">
            <Check className="h-3 w-3 text-primary-foreground opacity-0 transition peer-checked:opacity-100" />
          </span>
        </span>
        {label ? <span>{label}</span> : null}
      </label>
    );
  },
);
Checkbox.displayName = "Checkbox";

export { Checkbox };
