import { afterEach, describe, expect, it, vi } from 'vitest';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { createAppConfig } from '../../core/config/app-config';
import { NetworkFailure, ServerFailure } from '../../core/error-handling/failures';
import { ContentApi } from './content-api';
import { ContentRepositoryImpl } from './content-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/** Builds the real api → repository graph the container wires in the app. */
function makeRepository(): ContentRepositoryImpl {
  return new ContentRepositoryImpl(new ContentApi(createAppConfig()));
}

const contentIndex = {
  version: 1,
  groups: [{ id: 'foundations', title: 'Foundations', order: 1 }],
  topics: [
    {
      id: 'go-http-fundamentals',
      title: 'Go HTTP Fundamentals',
      group: 'foundations',
      order: 1,
      sectionCount: 4,
    },
  ],
};

const topicDetail = {
  version: 1,
  id: 'go-http-fundamentals',
  title: 'Go HTTP Fundamentals',
  group: 'foundations',
  order: 1,
  sections: [
    { id: 's1', title: 'The section', blocks: [{ kind: 'text', id: 'b1', text: 'The body.' }] },
  ],
};

describe('ContentRepositoryImpl', () => {
  it('returns the mapped domain content index on success', async () => {
    stubApiFetch([{ url: '/api/content', body: contentIndex }]);

    const result = await makeRepository().list();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.version).toBe(1);
      expect(result.value.groups[0]).toEqual({
        id: 'foundations',
        title: 'Foundations',
        order: 1,
      });
      expect(result.value.topics[0]?.sectionCount).toBe(4);
    }
  });

  it('returns the mapped domain topic detail on success', async () => {
    stubApiFetch([
      { url: '/api/content/go-http-fundamentals', body: topicDetail },
    ]);

    const result = await makeRepository().byId('go-http-fundamentals');

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.id).toBe('go-http-fundamentals');
      expect(result.value.sections[0]?.blocks[0]).toEqual({
        kind: 'text',
        id: 'b1',
        text: 'The body.',
      });
    }
  });

  it('URL-encodes the topic id into the detail path', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => topicDetail,
    }));
    vi.stubGlobal('fetch', fetchMock);

    await makeRepository().byId('weird id/slash');

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/content/weird%20id%2Fslash',
      expect.anything(),
    );
  });

  it('maps a network failure to a network-kind Failed result (status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().list();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure).toBeInstanceOf(NetworkFailure);
      expect(result.failure.kind).toBe('network');
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
      expect(result.failure.message).toBe('Could not reach the server.');
    }
  });

  it('maps a 404 error envelope to a server failure carrying code, status AND requestId', async () => {
    stubApiFetch([
      {
        url: '/api/content/missing',
        status: 404,
        body: {
          error: {
            code: 'not_found',
            message: "Topic 'missing' not found",
            requestId: 'req-1234',
          },
        },
      },
    ]);

    const result = await makeRepository().byId('missing');

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure).toBeInstanceOf(ServerFailure);
      expect(result.failure.kind).toBe('server');
      expect(result.failure.code).toBe('not_found');
      expect(result.failure.status).toBe(404);
      expect(result.failure.message).toBe("Topic 'missing' not found");
      expect(result.failure.requestId).toBe('req-1234');
    }
  });

  it('maps a non-JSON body to a server failure (unknown_error, status kept)', async () => {
    stubApiFetch([{ url: '/api/content', brokenJson: true, status: 502 }]);

    const result = await makeRepository().list();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure).toBeInstanceOf(ServerFailure);
      expect(result.failure.code).toBe('unknown_error');
      expect(result.failure.status).toBe(502);
    }
  });

  it('maps a malformed 2xx payload to a server failure with the invalid_response code', async () => {
    stubApiFetch([
      { url: '/api/content', body: { version: 1, groups: 'nope', topics: [] } },
    ]);

    const result = await makeRepository().list();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure).toBeInstanceOf(ServerFailure);
      expect(result.failure.code).toBe('invalid_response');
      expect(result.failure.status).toBe(200);
      expect(result.failure.message).toBe(
        'The server response did not match the expected shape.',
      );
    }
  });
});
