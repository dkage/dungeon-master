#!/usr/bin/env node
// Builds the styles-only Claude Design bundle from the app's real stylesheet.
// Usage: node .design-sync/build.mjs <out-dir>   (run from the repo root)
import { copyFileSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { basename, join } from 'node:path';

const OUT = process.argv[2];
if (!OUT) { console.error('usage: node .design-sync/build.mjs <out-dir>'); process.exit(1); }
const SRC = 'web/src';
const CARDS = '.design-sync/cards';
rmSync(OUT, { recursive: true, force: true });
mkdirSync(join(OUT, 'fonts'), { recursive: true });
mkdirSync(join(OUT, 'assets'), { recursive: true });

for (const f of readdirSync(join(SRC, 'assets/fonts'))) copyFileSync(join(SRC, 'assets/fonts', f), join(OUT, 'fonts', f));
for (const f of ['parchment.png', 'slate.jpg']) copyFileSync(join(SRC, 'assets', f), join(OUT, 'assets', f));

// Component-scoped rules first (as the app loads them), then app.css so its theme overrides win.
const scoped = [];
for (const f of ['lib/QuestDetail.svelte']) {
  const m = /<style>([\s\S]*?)<\/style>/.exec(readFileSync(join(SRC, f), 'utf8'));
  if (m) scoped.push(`/* from ${basename(f)} */\n${m[1].trim()}`);
}
const app = readFileSync(join(SRC, 'app.css'), 'utf8').replaceAll("url('./assets/fonts/", "url('./fonts/");
writeFileSync(join(OUT, '_ds_bundle.css'), `${scoped.join('\n\n')}\n\n/* from app.css */\n${app}`);
writeFileSync(join(OUT, 'styles.css'), '@import "./_ds_bundle.css";\n');

const header = { namespace: 'DungeonMaster', components: [], sourceHashes: {}, inlinedExternals: [], builtBy: 'cc-design-sync' };
writeFileSync(join(OUT, '_ds_bundle.js'), `/* @ds-bundle: ${JSON.stringify(header)} */\nwindow.DungeonMaster = window.DungeonMaster || {};\n`);

const index = [];
let count = 0;
for (const group of readdirSync(CARDS).sort()) {
  for (const f of readdirSync(join(CARDS, group)).filter((n) => n.endsWith('.html')).sort()) {
    const name = f.slice(0, -5);
    const dir = join(OUT, 'components', group, name);
    mkdirSync(dir, { recursive: true });
    const body = readFileSync(join(CARDS, group, f), 'utf8');
    writeFileSync(join(dir, `${name}.html`), `<!-- @dsCard group="${group}" -->
<!doctype html>
<html><head><meta charset="utf-8"><title>${name}</title>
  <link rel="stylesheet" href="../../../styles.css">
  <style>html,body{height:auto;background:#100d0b}</style>
</head><body>
<div class="app" data-theme="dungeon" data-font="almendra" style="display:block;height:auto;min-height:100vh;padding:12px">
${body}</div>
</body></html>
`);
    const prompt = readFileSync(join(CARDS, group, `${name}.prompt.md`), 'utf8');
    writeFileSync(join(dir, `${name}.prompt.md`), prompt);
    index.push(`- **${group}/${name}**: ${prompt.split('\n', 1)[0]} (\`components/${group}/${name}/${name}.prompt.md\`)`);
    count++;
  }
}

const conventions = readFileSync('.design-sync/conventions.md', 'utf8');
writeFileSync(join(OUT, 'README.md'), `${conventions.trim()}\n\n## Element index\n\n${index.join('\n')}\n`);
writeFileSync(join(OUT, '.ds-build-meta.json'), JSON.stringify({ shape: 'styles-only', componentCount: count }));
writeFileSync(join(OUT, '_ds_needs_recompile'), JSON.stringify({ by: 'design-sync-cli' }));
console.error(`built ${count} cards into ${OUT}`);
