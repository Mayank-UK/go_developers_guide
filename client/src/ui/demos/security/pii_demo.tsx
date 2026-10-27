import { SECURITY_PII_DEMO_USE_CASE } from '../../../domain/demo/get-security-story-demo-usecase';
import { SecurityStoryView } from './security_story_view';

/**
 * SecurityPIIDemo — the security topic's PII/GDPR demo (demo block id
 * `security:pii`), a thin named wrapper: the server serves both
 * security stories in one shared shape, so this file only resolves its
 * own pipeline (container token → use case) and hands the shared
 * SecurityStoryView its testid prefix. One request walks one fixture
 * data subject end to end: field classifications (the signup IP among
 * the PII), masked serving, access-log lines with no room for values,
 * erasure that persists, the tombstoned view after, and the audit
 * sweep that proves the original values absent everywhere.
 */

export function SecurityPIIDemo() {
  return (
    <SecurityStoryView
      token={SECURITY_PII_DEMO_USE_CASE}
      testidPrefix="security-pii"
      loadingSentence="classifying, masking, logging, and erasing a fixture data subject…"
      subject="the security PII demo"
    />
  );
}

export default SecurityPIIDemo;
