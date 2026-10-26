import type { SQLLabDemoData, SQLLabQuery } from '../../domain/demo/sql-lab';
import { isRecord, requiredArray, requiredString } from '../content/validate';

/**
 * SQLLabDTO — the wire/transport shape of the SQL lab payload. Row cells
 * are validated as string-or-number scalars (the "—" null rendering is
 * a string cell the server already made).
 */

export type SQLLabDemoDto = SQLLabDemoData;

/** Deserialize + validate the GET /api/demo/db/sql-lab payload. */
export function sqlLabDtoFromJson(body: unknown): SQLLabDemoDto {
  if (!isRecord(body)) throw new Error('sql-lab payload must be an object');
  return { queries: requiredArray(body, 'queries').map(queryFromJson) };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toSQLLabDemo(dto: SQLLabDemoDto): SQLLabDemoData {
  return dto;
}

function queryFromJson(body: unknown): SQLLabQuery {
  if (!isRecord(body)) throw new Error('exhibit must be an object');
  return {
    id: requiredString(body, 'id'),
    title: requiredString(body, 'title'),
    sql: requiredString(body, 'sql'),
    teach: requiredString(body, 'teach'),
    columns: requiredArray(body, 'columns').map((column) => {
      if (typeof column !== 'string' || column === '') {
        throw new Error('expected a non-empty column name');
      }
      return column;
    }),
    rows: requiredArray(body, 'rows').map((row) => {
      if (!Array.isArray(row)) throw new Error('exhibit row must be an array');
      return row.map((cell) => {
        if (typeof cell !== 'string' && typeof cell !== 'number') {
          throw new Error('exhibit cells must be strings or numbers');
        }
        return cell;
      });
    }),
  };
}
