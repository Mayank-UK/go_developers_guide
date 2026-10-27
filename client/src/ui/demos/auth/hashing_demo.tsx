import { AUTH_HASHING_DEMO_USE_CASE } from '../../../domain/demo/get-auth-story-demo-usecase';
import { AuthStoryView } from './auth_story_view';

/**
 * AuthHashingDemo — the authentication topic's hashing demo (demo block id
 * `auth:hashing`), a thin named wrapper: the server serves all seven auth
 * stories in one shared shape, so this file only resolves its own
 * pipeline (container token → use case) and hands the shared
 * AuthStoryView its testid prefix. One request proves the
 * argon2id disciplines live: fresh salts, full-cost wrong passwords, the
 * rehash signal on legacy cost, and the uniform decoy burn.
 */

export function AuthHashingDemo() {
  return (
    <AuthStoryView
      token={AUTH_HASHING_DEMO_USE_CASE}
      testidPrefix="auth-hashing"
      loadingSentence="hashing, verifying, and rehashing argon2id credentials live…"
      subject="the hashing demo"
    />
  );
}

export default AuthHashingDemo;
