import { describe, expect, it } from 'vitest';
import { routingExplorerDtoFromJson, toRoutingExplorerDemo } from './routing-explorer-dto';
import type { RoutingExplorerDemoData } from '../../domain/demo/routing-explorer';

/**
 * RoutingExplorerDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). Optional wire
 * keys (`handler` on exhibit-only patterns, `matchedPattern`, `allow`,
 * `redirectTarget`, the value arrays) are OMITTED by the server when
 * absent — they must land as undefined, never as null. Closed
 * vocabularies (`kind`, `outcome`, `source`) and the truthful statuses
 * validate against the Go builder's contract.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const wellFormed: RoutingExplorerDemoData = {
  routes: [
    { method: 'GET', pattern: '/api/health', kind: 'exact', handler: 'presentation.NewHealthHandler' },
    { method: 'GET', pattern: '/api/content', kind: 'exact', handler: 'presentation.NewContentListHandler' },
    {
      method: 'GET',
      pattern: '/api/content/{topicId}',
      kind: 'wildcard',
      handler: 'presentation.NewContentTopicHandler',
    },
    { method: 'GET', pattern: '/api/demo/scenario', kind: 'exact', handler: 'presentation.NewScenarioGetHandler' },
    { method: 'PATCH', pattern: '/api/demo/scenario', kind: 'exact', handler: 'presentation.NewScenarioPatchHandler' },
    { method: 'GET', pattern: '/api/demo/conc/race', kind: 'exact', handler: 'presentation.NewRaceDetectorHandler' },
  ],
  probes: [
    {
      method: 'GET',
      path: '/api/content',
      outcome: 'matched',
      status: 200,
      matchedPattern: 'GET /api/content',
      reasoning: 'Every segment is literal, so the router captured nothing.',
    },
    {
      method: 'HEAD',
      path: '/api/content/routing',
      outcome: 'matched',
      status: 200,
      matchedPattern: 'GET /api/content/{topicId}',
      pathValues: [{ name: 'topicId', values: ['routing'], source: 'path' }],
      reasoning: 'A GET pattern also serves HEAD.',
    },
    {
      method: 'GET',
      path: '/api/content/routing?verbose=1&tag=go&tag=routing',
      outcome: 'matched',
      status: 200,
      matchedPattern: 'GET /api/content/{topicId}',
      pathValues: [{ name: 'topicId', values: ['routing'], source: 'path' }],
      queryValues: [
        { name: 'verbose', values: ['1'], source: 'query' },
        { name: 'tag', values: ['go', 'routing'], source: 'query' },
      ],
      reasoning: 'The query never participates in matching.',
    },
    {
      method: 'DELETE',
      path: '/api/demo/scenario',
      outcome: 'method_not_allowed',
      status: 405,
      allow: ['GET', 'HEAD', 'PATCH'],
      reasoning: 'Allow unites both patterns and expands the implied HEAD.',
    },
    {
      method: 'GET',
      path: '/api/content/',
      outcome: 'not_found',
      status: 404,
      reasoning: 'There is no reverse-slash redirect.',
    },
    {
      method: 'GET',
      path: '/api/content/abc%2Fdef',
      outcome: 'matched',
      status: 200,
      matchedPattern: 'GET /api/content/{topicId}',
      pathValues: [{ name: 'topicId', values: ['abc/def'], source: 'path' }],
      reasoning: 'Matching runs on the escaped path.',
    },
  ],
  exhibits: [
    {
      id: 'precedence',
      title: 'Literal beats wildcard — the one overlap the table cannot show',
      note: 'A hypothetical literal beside the real wildcard.',
      patterns: [
        { method: 'GET', pattern: '/api/content/new', kind: 'exact' },
        {
          method: 'GET',
          pattern: '/api/content/{topicId}',
          kind: 'wildcard',
          handler: 'presentation.NewContentTopicHandler',
        },
      ],
      probes: [
        {
          method: 'GET',
          path: '/api/content/new',
          outcome: 'matched',
          status: 200,
          matchedPattern: 'GET /api/content/new',
          reasoning: 'Most-specific-wins, literal over wildcard.',
        },
      ],
    },
    {
      id: 'subtree',
      title: 'The trailing-slash subtree and its 307',
      note: 'A pattern ending in / matches everything under the prefix.',
      patterns: [{ method: 'GET', pattern: '/assets/', kind: 'subtree' }],
      probes: [
        {
          method: 'GET',
          path: '/assets',
          outcome: 'redirect',
          status: 307,
          redirectTarget: '/assets/',
          reasoning: 'The subtree root without its slash answers 307.',
        },
        {
          method: 'GET',
          path: '/assets/css/app.css',
          outcome: 'matched',
          status: 200,
          matchedPattern: 'GET /assets/',
          reasoning: 'The anonymous trailing wildcard swallowed the rest.',
        },
      ],
    },
  ],
  semantics: {
    getAlsoServesHead: true,
    methodlessMatchesAll: true,
    subtreeRedirectStatus: 307,
    reverseSlashRedirect: false,
    matchesEscapedPath: true,
    pathValueIsUnescaped: true,
  },
};

describe('routingExplorerDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(routingExplorerDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps absent facts omitted (undefined, never null or [])', () => {
    const parsed = routingExplorerDtoFromJson(wellFormed);

    // Exhibit-only pattern: no handler on the wire → undefined.
    expect(parsed.exhibits[0]?.patterns[0]?.handler).toBeUndefined();
    // A probe without extras carries none of the optional keys.
    expect(parsed.probes[4]?.matchedPattern).toBeUndefined();
    expect(parsed.probes[4]?.allow).toBeUndefined();
    expect(parsed.probes[4]?.redirectTarget).toBeUndefined();
  });

  it('keeps an empty reasoning sentence as data (present-but-empty passes)', () => {
    const parsed = routingExplorerDtoFromJson({
      ...wellFormed,
      probes: [{ ...wellFormed.probes[4], reasoning: '' }],
    });

    expect(parsed.probes[0]?.reasoning).toBe('');
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing routes', { ...wellFormed, routes: undefined }],
    ['routes not an array', { ...wellFormed, routes: 'nope' }],
    ['a non-object route', { ...wellFormed, routes: ['nope'] }],
    ['missing method on a route', { ...wellFormed, routes: [{ pattern: '/x', kind: 'exact' }] }],
    ['missing kind on a route', { ...wellFormed, routes: [{ method: 'GET', pattern: '/x' }] }],
    ['unknown kind vocabulary', { ...wellFormed, routes: [{ method: 'GET', pattern: '/x', kind: 'cosmic' }] }],
    ['missing probes', { ...wellFormed, probes: undefined }],
    ['a non-object probe', { ...wellFormed, probes: ['nope'] }],
    ['missing outcome', { ...wellFormed, probes: [{ method: 'GET', path: '/x', status: 200, reasoning: 'r' }] }],
    ['unknown outcome vocabulary', { ...wellFormed, probes: [{ ...wellFormed.probes[4], outcome: 'teapot' }] }],
    ['missing status', { ...wellFormed, probes: [{ ...wellFormed.probes[4], status: undefined }] }],
    ['a non-truthful status', { ...wellFormed, probes: [{ ...wellFormed.probes[4], status: 418 }] }],
    ['a non-integer status', { ...wellFormed, probes: [{ ...wellFormed.probes[4], status: 200.5 }] }],
    ['missing reasoning', { ...wellFormed, probes: [{ ...wellFormed.probes[4], reasoning: undefined }] }],
    ['a null allow list (null is not omission)', { ...wellFormed, probes: [{ ...wellFormed.probes[4], allow: null }] }],
    ['a non-string allow entry', { ...wellFormed, probes: [{ ...wellFormed.probes[3], allow: ['GET', 42] }] }],
    ['a routing value with no occurrences', { ...wellFormed, probes: [{ ...wellFormed.probes[1], pathValues: [{ name: 'topicId', values: [], source: 'path' }] }] }],
    ['unknown value source', { ...wellFormed, probes: [{ ...wellFormed.probes[1], pathValues: [{ name: 'topicId', values: ['x'], source: 'cookie' }] }] }],
    ['missing exhibits', { ...wellFormed, exhibits: undefined }],
    ['an exhibit without its note', { ...wellFormed, exhibits: [{ ...wellFormed.exhibits[0], note: undefined }] }],
    ['missing semantics', { ...wellFormed, semantics: undefined }],
    ['semantics not an object', { ...wellFormed, semantics: 'nope' }],
    ['a non-boolean semantics fact', { ...wellFormed, semantics: { ...wellFormed.semantics, getAlsoServesHead: 'yes' } }],
    ['a non-integer redirect status', { ...wellFormed, semantics: { ...wellFormed.semantics, subtreeRedirectStatus: '307' } }],
  ])('throws for %s', (_label, body) => {
    expect(() => routingExplorerDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = routingExplorerDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      probes: wellFormed.probes.map((probe) => ({ ...probe, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toRoutingExplorerDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toRoutingExplorerDemo(wellFormed)).toBe(wellFormed);
  });
});
