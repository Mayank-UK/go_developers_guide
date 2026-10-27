import { AUTH_JWT_DEMO_USE_CASE } from '../../../domain/demo/get-auth-story-demo-usecase';
import { AuthStoryView } from './auth_story_view';

/**
 * AuthJWTDemo — the authentication topic's JWT demo (demo block id
 * `auth:jwt`), a thin named wrapper: the server serves all seven auth
 * stories in one shared shape, so this file only resolves its own
 * pipeline (container token → use case) and hands the shared
 * AuthStoryView its testid prefix. One request mints an HS256
 * token, shows its anatomy, and presents six hostile tokens — forged,
 * tampered, alg=none, expired — each classified by the real verifier.
 */

export function AuthJWTDemo() {
  return (
    <AuthStoryView
      token={AUTH_JWT_DEMO_USE_CASE}
      testidPrefix="auth-jwt"
      loadingSentence="minting a token and attacking it six ways…"
      subject="the jwt demo"
    />
  );
}

export default AuthJWTDemo;
