import type { ExtensionPingResponse, PlatformIntegration, PlatformType } from '@cpbridge/contracts';
import { api } from '$lib/api/client';

const SUPPORTED_PLATFORMS: PlatformType[] = ['CODEFORCES', 'ATCODER'];

/**
 * Keep cpbridge's verified platform identity aligned with the account that is
 * active in the browser extension. The external submission is still verified
 * against the official platform before it can count toward standings.
 */
export async function syncActivePlatformIdentities(
  info: ExtensionPingResponse,
  platforms: PlatformType[] = SUPPORTED_PLATFORMS
): Promise<void> {
  const sessions = platforms
    .map((platform) => ({ platform, session: info.platforms[platform] }))
    .filter(({ session }) => session?.loggedIn && session.username?.trim());

  if (sessions.length === 0) return;

  const existing = await api.get<PlatformIntegration[]>('/integrations');
  await Promise.all(sessions.map(async ({ platform, session }) => {
    const username = session?.username?.trim();
    if (!username) return;

    const linked = existing.find((integration) => integration.platform === platform);
    if (
      linked?.connectionStatus === 'CONNECTED'
      && linked.externalUsername.toLowerCase() === username.toLowerCase()
    ) {
      return;
    }

    await api.put(`/integrations/${platform}`, {
      externalUsername: username,
      connectionStatus: 'CONNECTED'
    });
  }));
}
