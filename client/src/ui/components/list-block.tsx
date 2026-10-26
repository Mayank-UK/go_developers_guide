interface ListBlockProps {
  /** Item strings — validated 3–8 and non-empty at the DTO boundary. */
  items: string[];
  /** Ordered items render a numbered `<ol>`; unordered a bulleted `<ul>`. */
  ordered: boolean;
}

/**
 * ListBlock — the enumeration renderer for a topic's list blocks
 * (block-selection standard: 3–8 parallel items, no nesting). `ordered`
 * picks the semantic element — `<ol>` when sequence matters, `<ul>` when it
 * doesn't — so assistive tech announces list size and ordering for free;
 * Tailwind's preflight strips native list styling, so `list-decimal` /
 * `list-disc` restore it with the accent-colored marker. Items are plain
 * text in the guide's body typography, matching TextBlock's paragraphs.
 */
export function ListBlock({ items, ordered }: ListBlockProps) {
  const className =
    'flex w-full list-outside flex-col gap-sm pl-lg text-body leading-[1.7] text-on-surface-variant marker:text-primary';
  // Content is served static (never reordered client-side), so index keys
  // are stable — and keep duplicate item strings valid.
  const children = items.map((item, index) => <li key={index}>{item}</li>);

  return ordered ? (
    <ol className={`${className} list-decimal`}>{children}</ol>
  ) : (
    <ul className={`${className} list-disc`}>{children}</ul>
  );
}
