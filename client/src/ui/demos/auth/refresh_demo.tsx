import { AUTH_REFRESH_DEMO_USE_CASE } from '../../../domain/demo/get-auth-story-demo-usecase';
import { AuthStoryView } from './auth_story_view';

/**
 * AuthRefreshDemo — the authentication topic's refresh-rotation demo (demo block id
 * `auth:refresh`), a thin named wrapper: the server serves all seven auth
 * stories in one shared shape, so this file only resolves its own
 * pipeline (container token → use case) and hands the shared
 * AuthStoryView its testid prefix. One request walks one family
 * from birth through two rotations into the reuse alarm — and the newest
 * token dying with the family is fail-closed working.
 */

export function AuthRefreshDemo() {
  return (
    <AuthStoryView
      token={AUTH_REFRESH_DEMO_USE_CASE}
      testidPrefix="auth-refresh"
      loadingSentence="walking a refresh family through rotation and the reuse alarm…"
      subject="the refresh demo"
    />
  );
}

export default AuthRefreshDemo;
