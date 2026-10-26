import type { PostsWorkbenchDemoData } from '../../domain/demo/posts-workbench';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * PostsWorkbenchDTO — the wire/transport shape of the contract card.
 * Mirrors the domain model 1:1 (the interface is the seam where wire and
 * domain may diverge); the `fromJson` builds ONLY the listed fields
 * (whitelist) and throws on any shape violation, which parseDto upgrades
 * to an InvalidResponseException.
 *
 * `purpose`, `note`, and the other free-prose fields are `requiredText`
 * (present-but-empty passes) — prose emptiness is a server bug the DTO
 * need not pre-judge. Status codes and limits are numbers; the methods
 * and paths are the closed five-route table the workbench renders.
 */

export type PostsWorkbenchDemoDto = PostsWorkbenchDemoData;

/** Deserialize + validate the GET /api/demo/api/posts-workbench payload. */
export function postsWorkbenchDtoFromJson(body: unknown): PostsWorkbenchDemoDto {
  if (!isRecord(body)) throw new Error('posts-workbench payload must be an object');
  return {
    resource: resourceFromJson(body.resource),
    routes: requiredArray(body, 'routes').map(routeFromJson),
    pagination: paginationFromJson(body.pagination),
    filtering: filteringFromJson(body.filtering),
    versioning: versioningFromJson(body.versioning),
    deleteSemantics: deleteSemanticsFromJson(body.deleteSemantics),
    envelope: envelopeFromJson(body.envelope),
  };
}

/* ---- nested payload validators (throw on shape violation) ---- */

function resourceFromJson(body: unknown): PostsWorkbenchDemoData['resource'] {
  if (!isRecord(body)) throw new Error('workbench resource must be an object');
  return {
    name: requiredString(body, 'name'),
    collectionPath: requiredString(body, 'collectionPath'),
    itemPath: requiredString(body, 'itemPath'),
    wireShape: requiredString(body, 'wireShape'),
  };
}

function routeFromJson(body: unknown): PostsWorkbenchDemoData['routes'][number] {
  if (!isRecord(body)) throw new Error('workbench route must be an object');
  return {
    id: requiredString(body, 'id'),
    method: requiredString(body, 'method'),
    path: requiredString(body, 'path'),
    purpose: requiredText(body, 'purpose'),
    success: outcomeFromJson(body.success),
    errors: requiredArray(body, 'errors').map((entry) => {
      if (!isRecord(entry)) throw new Error('workbench route error must be an object');
      return {
        status: requiredNumber(entry, 'status'),
        code: requiredString(entry, 'code'),
        when: requiredText(entry, 'when'),
      };
    }),
  };
}

function outcomeFromJson(body: unknown): PostsWorkbenchDemoData['routes'][number]['success'] {
  if (!isRecord(body)) throw new Error('workbench outcome must be an object');
  return {
    status: requiredNumber(body, 'status'),
    note: requiredText(body, 'note'),
  };
}

function paginationFromJson(body: unknown): PostsWorkbenchDemoData['pagination'] {
  if (!isRecord(body)) throw new Error('workbench pagination must be an object');
  return {
    defaultLimit: requiredNumber(body, 'defaultLimit'),
    maxLimit: requiredNumber(body, 'maxLimit'),
    defaultPage: requiredNumber(body, 'defaultPage'),
    defaultSort: requiredString(body, 'defaultSort'),
    sortOptions: requiredArray(body, 'sortOptions').map((option) => {
      if (typeof option !== 'string' || option === '') {
        throw new Error('expected a non-empty sort option');
      }
      return option;
    }),
    envelopeFields: requiredArray(body, 'envelopeFields').map((field) => {
      if (typeof field !== 'string' || field === '') {
        throw new Error('expected a non-empty envelope field');
      }
      return field;
    }),
    emptyPageRule: requiredText(body, 'emptyPageRule'),
  };
}

function filteringFromJson(body: unknown): PostsWorkbenchDemoData['filtering'] {
  if (!isRecord(body)) throw new Error('workbench filtering must be an object');
  return {
    param: requiredString(body, 'param'),
    semantics: requiredText(body, 'semantics'),
    example: requiredString(body, 'example'),
  };
}

function versioningFromJson(body: unknown): PostsWorkbenchDemoData['versioning'] {
  if (!isRecord(body)) throw new Error('workbench versioning must be an object');
  return {
    prefix: requiredString(body, 'prefix'),
    strategy: requiredText(body, 'strategy'),
    contentExample: requiredText(body, 'contentExample'),
    whenToVersion: requiredText(body, 'whenToVersion'),
  };
}

function deleteSemanticsFromJson(body: unknown): PostsWorkbenchDemoData['deleteSemantics'] {
  if (!isRecord(body)) throw new Error('workbench deleteSemantics must be an object');
  return {
    onMiss: requiredNumber(body, 'onMiss'),
    chosen: requiredString(body, 'chosen'),
    rationale: requiredText(body, 'rationale'),
    alternative: requiredText(body, 'alternative'),
  };
}

function envelopeFromJson(body: unknown): PostsWorkbenchDemoData['envelope'] {
  if (!isRecord(body)) throw new Error('workbench envelope must be an object');
  return {
    wireShape: requiredString(body, 'wireShape'),
    members: requiredArray(body, 'members').map((entry) => {
      if (!isRecord(entry)) throw new Error('envelope member must be an object');
      return {
        name: requiredString(entry, 'name'),
        meaning: requiredText(entry, 'meaning'),
      };
    }),
    rfc9457Verdict: requiredText(body, 'rfc9457Verdict'),
  };
}
