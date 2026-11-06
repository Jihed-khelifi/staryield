"use client"

import * as React from "react"
import { EyeIcon, EyeOffIcon } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { cn } from "@/lib/utils"

/**
 * Password field with a show/hide toggle (Figma "Icon/Eye"). The toggle is
 * a real button with aria-pressed, and it never submits the form.
 */
function PasswordInput({
  className,
  wrapperClassName,
  ...props
}: Omit<React.ComponentProps<"input">, "type"> & { wrapperClassName?: string }) {
  const { text } = useI18n()
  const [visible, setVisible] = React.useState(false)
  return (
    <div data-slot="password-input" className={cn("relative flex w-full items-center", wrapperClassName)}>
      <input
        type={visible ? "text" : "password"}
        autoCapitalize="none"
        spellCheck={false}
        className={cn(
          "h-10 w-full min-w-0 rounded-lg border border-gold bg-cream-light px-3 pr-11 text-[15px] text-ink outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-danger",
          className
        )}
        {...props}
      />
      <button
        type="button"
        aria-pressed={visible}
        aria-label={text(visible ? "Hide password" : "Show password")}
        disabled={props.disabled}
        onClick={() => setVisible((v) => !v)}
        className="absolute right-1.5 inline-flex size-8 items-center justify-center rounded-md text-ink/80 hover:bg-warm-gray/40 hover:text-ink"
      >
        {visible ? <EyeOffIcon aria-hidden className="size-4" /> : <EyeIcon aria-hidden className="size-4" />}
      </button>
    </div>
  )
}

export { PasswordInput }
