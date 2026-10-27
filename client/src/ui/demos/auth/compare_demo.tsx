import { AUTH_COMPARE_DEMO_USE_CASE } from '../../../domain/demo/get-auth-story-demo-usecase';
import { AuthStoryView } from './auth_story_view';

/**
 * AuthCompareDemo — the authentication topic's comparison demo (demo block id
 * `auth:compare`), a thin named wrapper: the server serves all seven auth
 * stories in one shared shape, so this file only resolves its own
 * pipeline (container token → use case) and hands the shared
 * AuthStoryView its testid prefix. One request runs the same
 * identity through all three carriers with live verify timings — and the
 * JWT still standing after every revocation is the lesson.
 */

export function AuthCompareDemo() {
  return (
    <AuthStoryView
      token={AUTH_COMPARE_DEMO_USE_CASE}
      testidPrefix="auth-compare"
      loadingSentence="running one identity through sessions, keys, and JWTs side by side…"
      subject="the compare demo"
    />
  );
}

export default AuthCompareDemo;
