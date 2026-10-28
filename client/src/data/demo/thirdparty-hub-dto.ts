import type {
  ThirdPartyCountRow,
  ThirdPartyHubDemoData,
  ThirdPartyLivePanel,
  ThirdPartyPortMethod,
  ThirdPartyPortPanel,
  ThirdPartyRunRow,
  ThirdPartySandboxPanel,
  ThirdPartyTaxonomyRow,
  ThirdPartyVendorPanel,
  ThirdPartyWiringPanel,
  ThirdPartyWiringRow,
} from '../../domain/demo/thirdparty-hub';
import {
  isRecord,
  optionalNumber,
  optionalString,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ThirdPartyHubDTO — the wire/transport shape of the hub payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException.
 *
 * Optional wire fields mirror the server's omitempty exactly: a
 * SUCCESS row carries no vendorCode / vendorStatus / kind / httpStatus
 * (there was no vendor failure to classify), so those decode as
 * undefined — present-but-empty is real data on the strings that can
 * be empty, which therefore use requiredText, never requiredString.
 */

export type ThirdPartyHubDemoDto = ThirdPartyHubDemoData;

function runRowFromJson(value: unknown): ThirdPartyRunRow {
  if (!isRecord(value)) throw new Error('run row must be an object');
  return {
    step: requiredNumber(value, 'step'),
    call: requiredString(value, 'call'),
    input: requiredText(value, 'input'),
    outcome: requiredString(value, 'outcome'),
    vendorCode: optionalString(value, 'vendorCode'),
    vendorStatus: optionalNumber(value, 'vendorStatus'),
    kind: optionalString(value, 'kind'),
    httpStatus: optionalNumber(value, 'httpStatus'),
    retryable: requiredBoolean(value, 'retryable'),
    detail: requiredText(value, 'detail'),
  };
}

function countRowFromJson(value: unknown): ThirdPartyCountRow {
  if (!isRecord(value)) throw new Error('count row must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredNumber(value, 'value'),
  };
}

function portMethodFromJson(value: unknown): ThirdPartyPortMethod {
  if (!isRecord(value)) throw new Error('port method must be an object');
  return {
    name: requiredString(value, 'name'),
    signature: requiredString(value, 'signature'),
  };
}

function portFromJson(value: unknown): ThirdPartyPortPanel {
  if (!isRecord(value)) throw new Error('port panel must be an object');
  return {
    name: requiredString(value, 'name'),
    methods: requiredArray(value, 'methods').map(portMethodFromJson),
    implementations: requiredArray(value, 'implementations').map(String),
    note: requiredText(value, 'note'),
  };
}

function sandboxFromJson(value: unknown): ThirdPartySandboxPanel {
  if (!isRecord(value)) throw new Error('sandbox panel must be an object');
  return {
    config: requiredArray(value, 'config').map(String),
    rows: requiredArray(value, 'rows').map(runRowFromJson),
    counts: requiredArray(value, 'counts').map(countRowFromJson),
  };
}

function liveFromJson(value: unknown): ThirdPartyLivePanel {
  if (!isRecord(value)) throw new Error('live panel must be an object');
  return {
    vendor: requiredString(value, 'vendor'),
    apiBaseUrl: requiredString(value, 'apiBaseUrl'),
    keyEnvVar: requiredString(value, 'keyEnvVar'),
    callSite: requiredString(value, 'callSite'),
    outcome: runRowFromJson(requiredRecord(value, 'outcome')),
    note: requiredText(value, 'note'),
  };
}

function vendorFromJson(value: unknown): ThirdPartyVendorPanel {
  if (!isRecord(value)) throw new Error('vendor panel must be an object');
  return {
    id: requiredString(value, 'id'),
    title: requiredString(value, 'title'),
    lead: requiredText(value, 'lead'),
    port: portFromJson(requiredRecord(value, 'port')),
    sandbox: sandboxFromJson(requiredRecord(value, 'sandbox')),
    live: liveFromJson(requiredRecord(value, 'live')),
  };
}

function wiringRowFromJson(value: unknown): ThirdPartyWiringRow {
  if (!isRecord(value)) throw new Error('wiring row must be an object');
  return {
    category: requiredString(value, 'category'),
    port: requiredString(value, 'port'),
    fakeAdapter: requiredString(value, 'fakeAdapter'),
    liveAdapter: requiredString(value, 'liveAdapter'),
    active: requiredString(value, 'active'),
    activeOutcome: requiredText(value, 'activeOutcome'),
  };
}

function wiringFromJson(value: unknown): ThirdPartyWiringPanel {
  if (!isRecord(value)) throw new Error('wiring panel must be an object');
  return {
    source: requiredString(value, 'source'),
    current: requiredString(value, 'current'),
    getEndpoint: requiredString(value, 'getEndpoint'),
    patchEndpoint: requiredString(value, 'patchEndpoint'),
    rows: requiredArray(value, 'rows').map(wiringRowFromJson),
    note: requiredText(value, 'note'),
  };
}

function taxonomyRowFromJson(value: unknown): ThirdPartyTaxonomyRow {
  if (!isRecord(value)) throw new Error('taxonomy row must be an object');
  return {
    vendorCode: requiredString(value, 'vendorCode'),
    vendorStatus: requiredNumber(value, 'vendorStatus'),
    kind: requiredString(value, 'kind'),
    httpStatus: requiredNumber(value, 'httpStatus'),
    retryable: requiredBoolean(value, 'retryable'),
    viaRawStatus: requiredBoolean(value, 'viaRawStatus'),
    note: requiredText(value, 'note'),
  };
}

/** Deserialize + validate the GET /api/demo/thirdparty/hub payload. */
export function thirdPartyHubDtoFromJson(body: unknown): ThirdPartyHubDemoDto {
  if (!isRecord(body)) throw new Error('thirdparty-hub payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    honestyNote: requiredText(body, 'honestyNote'),
    wiring: wiringFromJson(requiredRecord(body, 'wiring')),
    vendors: requiredArray(body, 'vendors').map(vendorFromJson),
    taxonomyMap: requiredArray(body, 'taxonomyMap').map(taxonomyRowFromJson),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toThirdPartyHubDemo(dto: ThirdPartyHubDemoDto): ThirdPartyHubDemoData {
  return dto;
}
