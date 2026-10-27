import { AUTH_API_KEYS_DEMO_USE_CASE } from '../../../domain/demo/get-auth-story-demo-usecase';
import { AuthStoryView } from './auth_story_view';

/**
 * AuthAPIKeysDemo — the authentication topic's API-keys demo (demo block id
 * `auth:api-keys`), a thin named wrapper: the server serves all seven auth
 * stories in one shared shape, so this file only resolves its own
 * pipeline (container token → use case) and hands the shared
 * AuthStoryView its testid prefix. One request walks the key
 * lifecycle whole: the raw key shown exactly once, the hashed-at-rest roster,
 * constant-time rejections, and the revocation that kills every copy.
 */

export function AuthAPIKeysDemo() {
  return (
    <AuthStoryView
      token={AUTH_API_KEYS_DEMO_USE_CASE}
      testidPrefix="auth-api-keys"
      loadingSentence="generating a key, hashing it at rest, and revoking it live…"
      subject="the api-keys demo"
    />
  );
}

export default AuthAPIKeysDemo;
