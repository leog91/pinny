import { defineConfig } from 'vitest/config';
import { cloudflareTest, readD1Migrations } from '@cloudflare/vitest-plugin';

export default defineConfig({
  plugins: [cloudflareTest({
    wrangler: { configPath: './wrangler.jsonc' },
    miniflare: { bindings: {
      CAPTURE_TOKEN: 'test-capture-credential-32-characters',
      SYNC_TOKEN: 'test-desktop-credential-32-characters',
      TEST_MIGRATIONS: await readD1Migrations('./migrations'),
    } },
  })],
  test: { include: ['./test/**/*.test.ts'], setupFiles: ['./test/setup.ts'] },
});
