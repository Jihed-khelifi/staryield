"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { ExpandableList, PanelSection } from "@/components/chat/client-panel/panel-section";
import { PLACEMENTS_PREVIEW, type Placement } from "@/components/chat/client-panel/panel-helpers";

/**
 * "Overview": the client's placements, three at first and the rest behind
 * "+ expand" (Figma 1172:1191, 1172:1613). Each sign has its glyph; the
 * planet's name is read out to screen readers.
 */
export function PlacementsOverview({ placements, timeKnown }: { placements: Placement[]; timeKnown?: boolean }) {
  const { text } = useI18n();
  return (
    <PanelSection title={text("Overview")} testId="client-placements">
      {placements.length ? (
        <ExpandableList
          items={placements}
          preview={PLACEMENTS_PREVIEW}
          className="placement-grid"
          listLabel={text("Placements")}
          render={(p) => (
            <li key={p.body} className={p.unknown ? "placement placement-unknown" : "placement"} title={text(p.body)} data-testid={`placement-${p.body.toLowerCase()}`}>
              <span aria-hidden className="placement-glyph">
                {p.glyph}
              </span>
              {p.unknown ? (
                <>
                  <span className="sr-only">{text(p.body === "Ascendant" ? "Rising sign unknown" : "{value0} sign unknown", { value0: text(p.body) })}</span>
                  <span aria-hidden>{text("Unknown")}</span>
                </>
              ) : (
                <>
                  <span className="sr-only">{text("{value0} in", { value0: text(p.body) })} </span>
                  <span>{text(p.sign)}</span>
                </>
              )}
              {p.retrograde ? <span className="sr-only"> {text("Retrograde")}</span> : null}
            </li>
          )}
        />
      ) : (
        <p className="panel-muted">{text("This client has not added their birth date yet.")}</p>
      )}
      {placements.length && timeKnown === false ? (
        <p className="panel-muted mt-2">{text("Birth time unknown: rising sign and houses are not available.")}</p>
      ) : null}
    </PanelSection>
  );
}
