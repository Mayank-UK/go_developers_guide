import { AUTH_SESSIONS_DEMO_USE_CASE } from '../../../domain/demo/get-auth-story-demo-usecase';
import { AuthStoryView } from './auth_story_view';

/**
 * AuthSessionsDemo — the authentication topic's sessions demo (demo block id
 * `auth:sessions`), a thin named wrapper: the server serves all seven auth
 * stories in one shared shape, so this file only resolves its own
 * pipeline (container token → use case) and hands the shared
 * AuthStoryView its testid prefix. One request walks the cookie
 * lifecycle whole: login (argon2id verify, sealed HttpOnly cookie), the /me
 * inspect, a one-character tamper refused by the seal, uniform 401s with
 * their measured costs, and the logout that deletes the row.
 */

export function AuthSessionsDemo() {
  return (
    <AuthStoryView
      token={AUTH_SESSIONS_DEMO_USE_CASE}
      testidPrefix="auth-sessions"
      loadingSentence="walking the login → sealed cookie → logout lifecycle against the live engines…"
      subject="the sessions demo"
    />
  );
}

export default AuthSessionsDemo;
