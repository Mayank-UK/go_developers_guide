import type { ReactNode } from 'react';

/**
 * SectionWrapper — the top-level anatomy unit of every screen: an
 * anchor-addressable section carrying a numbered titled header, an optional
 * lead paragraph, and the section's content blocks. The accent bar marks
 * section titles so long pages stay scannable; scroll-mt-xl (64px) makes
 * in-page anchors land below the sticky header.
 */
interface SectionWrapperProps {
  id: string;
  title: string;
  /** 1-based display position, rendered zero-padded when provided. */
  index?: number;
  lead?: string;
  children: ReactNode;
}

export function SectionWrapper({ id, title, index, lead, children }: SectionWrapperProps) {
  return (
    <section
      id={id}
      className="flex w-full scroll-mt-xl flex-col gap-md rounded-lg border border-outline bg-surface-tint p-lg shadow-button-card"
    >
      <h2 className="flex items-center gap-sm font-display text-title-lg text-on-surface">
        {index !== undefined && (
          <span
            aria-hidden
            className="font-mono text-title-sm font-normal text-primary"
          >
            {String(index).padStart(2, '0')}
          </span>
        )}
        <span
          aria-hidden
          className="w-xs shrink-0 self-stretch rounded-pill bg-primary"
        />
        {title}
      </h2>
      {lead !== undefined && (
        <p className="text-body text-on-surface-variant">{lead}</p>
      )}
      {children}
    </section>
  );
}
