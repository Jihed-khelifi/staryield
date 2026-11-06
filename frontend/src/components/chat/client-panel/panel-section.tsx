"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { cn } from "@/lib/utils";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";

/** Castellar 9px heading with a hairline to the right ("OVERVIEW ———"). */
export function PanelSection({
  title,
  icon,
  children,
  className,
  testId,
}: {
  /** Translated heading. */
  title: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  testId?: string;
}) {
  const id = React.useId();
  return (
    <section className={cn("panel-section", className)} aria-labelledby={id} data-testid={testId}>
      <h3 id={id} className="panel-heading">
        {icon ? <span aria-hidden className="panel-heading-icon">{icon}</span> : null}
        <span>{title}</span>
      </h3>
      {children}
    </section>
  );
}

/**
 * A preview with "+ expand" / "- collapse" for the rest (Figma 1172:1613,
 * 1174:1858). Without hidden items there is no toggle.
 */
export function ExpandableList<T>({
  items,
  preview,
  render,
  className,
  listLabel,
}: {
  items: T[];
  preview: number;
  render: (item: T) => React.ReactNode;
  className?: string;
  /** Accessible name of the list (translated). */
  listLabel: string;
}) {
  const { text } = useI18n();
  const [open, setOpen] = React.useState(false);
  const rest = items.slice(preview);
  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <ul className={className} aria-label={listLabel}>
        {items.slice(0, preview).map(render)}
      </ul>
      {rest.length ? (
        <>
          <CollapsibleContent asChild>
            <ul className={cn(className, "mt-[var(--panel-gap,8px)]")} aria-label={listLabel}>
              {rest.map(render)}
            </ul>
          </CollapsibleContent>
          <CollapsibleTrigger className="panel-expand" data-testid="panel-expand">
            {text(open ? "- collapse" : "+ expand")}
          </CollapsibleTrigger>
        </>
      ) : null}
    </Collapsible>
  );
}
