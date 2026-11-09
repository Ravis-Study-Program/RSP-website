import { describe, expect, it } from 'vitest';
import {
  captureDiscordProfile,
  discordAvatarUrl,
  oauthRequestContext,
} from '../src/identity/discord-profile.js';

const profile = {
  id: '123456789012345678',
  avatar: 'a_0123456789abcdef0123456789abcdef',
};

describe('Discord avatar provenance', () => {
  it('builds a static sized CDN URL from the authenticated Discord profile', () => {
    expect(discordAvatarUrl(profile)).toBe(
      'https://cdn.discordapp.com/avatars/123456789012345678/a_0123456789abcdef0123456789abcdef.png?size=256',
    );
    expect(discordAvatarUrl({ ...profile, avatar: null })).toBeNull();
    expect(() => discordAvatarUrl({ ...profile, id: '../other' })).toThrow();
    expect(() =>
      discordAvatarUrl({ ...profile, avatar: 'https://example.com/photo.png' }),
    ).toThrow();
  });

  it('keeps concurrent OAuth responses isolated', async () => {
    const results = await Promise.all(
      ['first', 'second'].map((id) =>
        oauthRequestContext.run({}, async () => {
          captureDiscordProfile(id, profile);
          await Promise.resolve();
          return oauthRequestContext.getStore()?.update?.authUserId;
        }),
      ),
    );
    expect(results).toEqual(['first', 'second']);
    expect(oauthRequestContext.getStore()).toBeUndefined();
  });
});
