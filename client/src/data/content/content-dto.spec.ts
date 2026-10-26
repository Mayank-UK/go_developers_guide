import { describe, expect, it } from 'vitest';
import {
  contentIndexDtoFromJson,
  topicDetailDtoFromJson,
  toContentIndex,
  toTopicDetail,
} from './content-dto';

/**
 * DTO parse spec — happy path (both endpoints, all four block kinds),
 * malformed payloads throw, and unknown wire fields are whitelisted away.
 * Fixtures mirror the recorded content wire contract exactly (camelCase
 * fields; text blocks carry `text`, code blocks carry `snippet`, a demo
 * block's id doubles as its demo id, list blocks carry `ordered` + `items`).
 */

const contentIndex = {
  version: 1,
  groups: [
    { id: 'foundations', title: 'Foundations', order: 1 },
    { id: 'framework', title: 'Framework', order: 2 },
  ],
  topics: [
    {
      id: 'go-language',
      title: 'Go Language',
      subtitle: 'Why Go on the server',
      group: 'foundations',
      order: 1,
      sectionCount: 8,
    },
    { id: 'go-runtime', title: 'Go Runtime', group: 'foundations', order: 2, sectionCount: 0 },
  ],
};

const topicDetail = {
  version: 1,
  id: 'go-runtime',
  title: 'Go Runtime',
  subtitle: 'The scheduler behind the code',
  group: 'foundations',
  order: 2,
  sections: [
    {
      id: 'go-runtime:models',
      title: 'Goroutines + channels',
      lead: 'The lead paragraph.',
      blocks: [
        { kind: 'text', id: 'b-text', text: 'The body of the text block.' },
        {
          kind: 'code',
          id: 'b-code',
          language: 'py',
          caption: 'the caption',
          snippet: 'type Hello struct {\n    Name string\n}',
          source: 'server/guide/hello.py',
        },
        {
          kind: 'demo',
          id: 'runtime:lab',
          title: 'The goroutine lab',
          description: 'Watch validation coerce in real time.',
        },
        {
          kind: 'list',
          id: 'b-list',
          ordered: false,
          items: ['Field — declares a typed field.', 'Validator — custom checks.', 'Dump — serializes out.'],
        },
      ],
    },
    {
      id: 'go-runtime:terminal',
      title: 'Terminal companion',
      blocks: [],
    },
  ],
};

describe('contentIndexDtoFromJson', () => {
  it('parses the happy-path index: groups and topic summaries', () => {
    const dto = contentIndexDtoFromJson(contentIndex);

    expect(dto.version).toBe(1);
    expect(dto.groups).toHaveLength(2);
    expect(dto.groups[0]).toEqual({
      id: 'foundations',
      title: 'Foundations',
      order: 1,
    });
    expect(dto.topics[0]).toEqual({
      id: 'go-language',
      title: 'Go Language',
      subtitle: 'Why Go on the server',
      group: 'foundations',
      order: 1,
      sectionCount: 8,
    });
    // Optional fields resolve to undefined when the wire omits them.
    expect(dto.topics[1]?.subtitle).toBeUndefined();
  });

  it('parses an EMPTY index (the registry today) as a valid state', () => {
    const dto = contentIndexDtoFromJson({ version: 1, groups: [], topics: [] });
    expect(toContentIndex(dto)).toEqual({ version: 1, groups: [], topics: [] });
  });

  it('whitelists: unknown fields are dropped, known fields survive', () => {
    const dto = contentIndexDtoFromJson({
      ...contentIndex,
      futureField: 'not in the contract',
      topics: [
        { ...(contentIndex.topics[0] ?? {}), extra: true, id: 'one-topic' },
      ],
    });

    expect(dto.topics[0]?.id).toBe('one-topic');
    expect(JSON.stringify(dto)).not.toContain('futureField');
    expect(JSON.stringify(dto)).not.toContain('"extra"');
  });

  it.each([
    ['not an object', 'nope'],
    ['groups not an array', { version: 1, groups: 'nope', topics: [] }],
    ['missing topics', { version: 1, groups: [] }],
    ['topic summary missing sectionCount', { version: 1, groups: [], topics: [{ id: 'a', title: 'A', group: 'g', order: 1 }] }],
    ['group order NaN', { version: 1, groups: [{ id: 'g', title: 'G', order: Number.NaN }], topics: [] }],
  ])('throws on malformed index: %s', (_label, body) => {
    expect(() => contentIndexDtoFromJson(body)).toThrow();
  });
});

describe('topicDetailDtoFromJson', () => {
  it('parses the happy-path detail with all four block kinds, in order', () => {
    const dto = topicDetailDtoFromJson(topicDetail);
    const blocks = dto.sections[0]?.blocks ?? [];

    expect(blocks.map((block) => block.kind)).toEqual(['text', 'code', 'demo', 'list']);
    expect(blocks[0]).toEqual({ kind: 'text', id: 'b-text', text: 'The body of the text block.' });
    expect(blocks[1]).toEqual({
      kind: 'code',
      id: 'b-code',
      language: 'py',
      caption: 'the caption',
      snippet: 'type Hello struct {\n    Name string\n}',
      source: 'server/guide/hello.py',
    });
    expect(blocks[2]).toEqual({
      kind: 'demo',
      id: 'runtime:lab',
      title: 'The goroutine lab',
      description: 'Watch validation coerce in real time.',
    });
    expect(blocks[3]).toEqual({
      kind: 'list',
      id: 'b-list',
      ordered: false,
      items: ['Field — declares a typed field.', 'Validator — custom checks.', 'Dump — serializes out.'],
    });
    expect(dto.sections[1]?.blocks).toEqual([]);
    expect(toTopicDetail(dto).id).toBe('go-runtime');
  });

  it.each([
    ['not an object', []],
    ['missing title', { version: 1, id: 'a', group: 'g', order: 1, sections: [] }],
    ['sections not an array', { version: 1, id: 'a', title: 'A', group: 'g', order: 1, sections: {} }],
    ['unknown block kind', { version: 1, id: 'a', title: 'A', group: 'g', order: 1, sections: [{ id: 's', title: 'S', blocks: [{ kind: 'video', id: 'b' }] }] }],
    ['text block missing text', { version: 1, id: 'a', title: 'A', group: 'g', order: 1, sections: [{ id: 's', title: 'S', blocks: [{ kind: 'text', id: 'b' }] }] }],
    ['code block missing snippet', { version: 1, id: 'a', title: 'A', group: 'g', order: 1, sections: [{ id: 's', title: 'S', blocks: [{ kind: 'code', id: 'b', language: 'py' }] }] }],
    ['demo block missing id', { version: 1, id: 'a', title: 'A', group: 'g', order: 1, sections: [{ id: 's', title: 'S', blocks: [{ kind: 'demo', title: 'T' }] }] }],
  ])('throws on malformed detail: %s', (_label, body) => {
    expect(() => topicDetailDtoFromJson(body)).toThrow();
  });

  it('whitelists: unknown keys are dropped', () => {
    const dto = topicDetailDtoFromJson({
      ...topicDetail,
      futureTopLevel: true,
    });
    expect(JSON.stringify(dto)).not.toContain('futureTopLevel');
  });
});

describe('list block validation (block-selection standard: 3–8 items)', () => {
  const listTopic = (listBlock: Record<string, unknown>) => ({
    version: 1,
    id: 'a',
    title: 'A',
    group: 'g',
    order: 1,
    sections: [{ id: 's', title: 'S', blocks: [listBlock] }],
  });

  it.each([
    ['missing ordered', { kind: 'list', id: 'b', items: ['one', 'two', 'three'] }],
    ['ordered not a boolean', { kind: 'list', id: 'b', ordered: 'yes', items: ['one', 'two', 'three'] }],
    ['items not an array', { kind: 'list', id: 'b', ordered: true, items: 'nope' }],
    ['item not a string', { kind: 'list', id: 'b', ordered: true, items: ['one', 2, 'three'] }],
    ['blank item', { kind: 'list', id: 'b', ordered: true, items: ['one', '   ', 'three'] }],
    ['too few items', { kind: 'list', id: 'b', ordered: true, items: ['one', 'two'] }],
    ['too many items', { kind: 'list', id: 'b', ordered: true, items: ['1', '2', '3', '4', '5', '6', '7', '8', '9'] }],
  ])('throws on malformed list block: %s', (_label, listBlock) => {
    expect(() => topicDetailDtoFromJson(listTopic(listBlock))).toThrow();
  });

  it('accepts both size bounds (3 and 8 items) with ordered true or false', () => {
    const atMin = ['one', 'two', 'three'];
    const atMax = ['1', '2', '3', '4', '5', '6', '7', '8'];
    for (const ordered of [true, false]) {
      for (const items of [atMin, atMax]) {
        const dto = topicDetailDtoFromJson(
          listTopic({ kind: 'list', id: 'b', ordered, items }),
        );
        expect(dto.sections[0]?.blocks[0]).toEqual({ kind: 'list', id: 'b', ordered, items });
      }
    }
  });
});
