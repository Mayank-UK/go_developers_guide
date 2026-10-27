import { AUTH_OAUTH_DEMO_USE_CASE } from '../../../domain/demo/get-auth-story-demo-usecase';
import { AuthStoryView } from './auth_story_view';

/**
 * AuthOAuthDemo — the authentication topic's OAuth2 demo (demo block id
 * `auth:oauth`), a thin named wrapper: the server serves all seven auth
 * stories in one shared shape, so this file only resolves its own
 * pipeline (container token → use case) and hands the shared
 * AuthStoryView its testid prefix. One request runs the whole
 * grant: PKCE pair, authorize redirect with the one-time code, exchange,
 * stateless verify, and the refusals that prove codes burn on first use.
 */

export function AuthOAuthDemo() {
  return (
    <AuthStoryView
      token={AUTH_OAUTH_DEMO_USE_CASE}
      testidPrefix="auth-oauth"
      loadingSentence="running the OAuth2 code grant with PKCE end to end…"
      subject="the oauth demo"
    />
  );
}

export default AuthOAuthDemo;
