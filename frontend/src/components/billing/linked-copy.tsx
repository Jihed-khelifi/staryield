import * as React from "react";

/**
 * A translated sentence with links in it. The copy marks each link's label
 * with square brackets ("Review our [Terms] and [Refund Policy]."); the n-th
 * bracketed label becomes the n-th entry of `links`, so every language keeps
 * its own word order and its own label. Text with more brackets than `links`
 * keeps the extra labels as plain text.
 */
export function LinkedCopy({ copy, links }: { copy: string; links: ((label: string) => React.ReactNode)[] }) {
  const parts = copy.split(/\[([^\]]+)\]/);
  // Odd indexes are the bracketed labels.
  return (
    <>
      {parts.map((part, i) => {
        if (i % 2 === 0) return part;
        const render = links[(i - 1) / 2];
        return <React.Fragment key={i}>{render ? render(part) : part}</React.Fragment>;
      })}
    </>
  );
}
