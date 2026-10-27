import { SECURITY_LAB_DEMO_USE_CASE } from '../../../domain/demo/get-security-story-demo-usecase';
import { SecurityStoryView } from './security_story_view';

/**
 * SecurityLabDemo — the security topic's lab demo (demo block id
 * `security:lab`), a thin named wrapper: the server serves both
 * security stories in one shared shape, so this file only resolves its
 * own pipeline (container token → use case) and hands the shared
 * SecurityStoryView its testid prefix. One request walks the whole
 * edge: the five-header policy this very response carries, a FRESH
 * token bucket spent in front of you (burst, the 429 boundary with its
 * honest retry hint, recovery at the sustained rate), per-client
 * isolation, and the route's own limiter named for the curl tour.
 */

export function SecurityLabDemo() {
  return (
    <SecurityStoryView
      token={SECURITY_LAB_DEMO_USE_CASE}
      testidPrefix="security-lab"
      loadingSentence="stamping the headers and spending a live rate-limit window against the engines…"
      subject="the security lab demo"
    />
  );
}

export default SecurityLabDemo;
