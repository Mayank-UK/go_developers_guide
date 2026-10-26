interface TextBlockProps {
  /** Prose from a served text block; `\n\n` separates paragraphs. */
  text: string;
}

/**
 * TextBlock — the prose renderer for a topic's text blocks. The content model
 * sends one string where `\n\n` marks a paragraph break; this component splits
 * it into real `<p>` elements (semantic paragraphs, not newline-styled text)
 * and preserves any single `\n` inside a paragraph as a hard line break via
 * `pre-line`.
 */
export function TextBlock({ text }: TextBlockProps) {
  const paragraphs = text.split(PARAGRAPH_SEPARATOR).filter(isRenderableParagraph);

  return (
    <div className="flex w-full flex-col gap-sm">
      {paragraphs.map((paragraph, index) => (
        <p
          key={index}
          className="w-full text-body leading-[1.7] text-on-surface-variant [white-space:pre-line]"
        >
          {paragraph}
        </p>
      ))}
    </div>
  );
}

/** Two or more consecutive newlines are one paragraph break, however many. */
const PARAGRAPH_SEPARATOR = /\n{2,}/;

/** Served text is validated non-empty, but never render a blank paragraph. */
function isRenderableParagraph(paragraph: string): boolean {
  return paragraph.trim() !== '';
}
