#!/usr/bin/env node
// Builds the styles-only Claude Design bundle from the app's real stylesheet.
// Usage: node .design-sync/build.mjs <out-dir>   (run from the repo root)
import { execFileSync } from 'node:child_process';
import { copyFileSync, cpSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
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
cpSync(join(SRC, 'assets/map'), join(OUT, 'assets/map'), { recursive: true });

// Prototype renders, downscaled to JPEG for the reference cards (macOS sips).
const REFS = { 'v4': 'prototype-map', 'v4-list': 'prototype-list', 'v4-list-2': 'prototype-locked' };
mkdirSync(join(OUT, 'assets/reference'), { recursive: true });
for (const [dir, name] of Object.entries(REFS)) {
  const src = readdirSync(join('design/stitch-prototype', dir)).find((f) => f.startsWith('screenshot.'));
  execFileSync('sips', ['-Z', '1600', '-s', 'format', 'jpeg', '-s', 'formatOptions', '80',
    join('design/stitch-prototype', dir, src), '--out', join(OUT, 'assets/reference', `${name}.jpg`)], { stdio: 'ignore' });
}

// Real component output rendered from fixtures (see capture/run.sh).
const CAP = join(OUT, '.capture');
mkdirSync(CAP, { recursive: true });
execFileSync('.design-sync/capture/run.sh', [CAP], { stdio: 'ignore' });
// A url() inside a custom property (--floor) resolves against the stylesheet that uses it, which sits at the root.
const captured = (name) => readFileSync(join(CAP, `${name}.html`), 'utf8').replaceAll(' class=""', '')
  .replace(/(--[\w-]+:\s*url\()ASSETS\//g, '$1assets/').replaceAll('ASSETS/', '../../../assets/');

// Svelte scopes component styles with a hashed class; bundled raw they would leak onto the app's own classes
// (.badge, .num, .link, ...), so prefix every selector with the component's root class instead.
function scope(css, root) {
  let out = '', i = 0;
  while (i < css.length) {
    const open = css.indexOf('{', i);
    if (open < 0) { out += css.slice(i); break; }
    const head = css.slice(i, open).trim();
    let depth = 1, j = open + 1;
    while (depth && j < css.length) { if (css[j] === '{') depth++; else if (css[j] === '}') depth--; j++; }
    const body = css.slice(open + 1, j - 1);
    const lead = css.slice(i, open).match(/^\s*((?:\/\*[\s\S]*?\*\/\s*)*)/)[1];
    const sel = head.replace(/^(?:\/\*[\s\S]*?\*\/\s*)*/, '');
    if (sel.startsWith('@keyframes')) out += `${lead}${sel} {${body}}\n`;
    else if (sel.startsWith('@')) out += `${lead}${sel} {\n${scope(body, root)}}\n`;
    else out += `${lead}${sel.split(',').map((x) => {
      x = x.trim();
      return new RegExp(`^\\${root}(?![\\w-])`).test(x) ? x : /^\.plain(?![\w-])/.test(x) ? `${root}${x}` : `${root} ${x}`;
    }).join(', ')} {${body}}\n`;
    i = j;
  }
  return out;
}

// Component-scoped rules first (as the app loads them), then app.css so its theme overrides win.
const scoped = [];
for (const [f, root] of [['lib/QuestDetail.svelte', null], ['lib/QuestMap.svelte', '.map']]) {
  const m = /<style>([\s\S]*?)<\/style>/.exec(readFileSync(join(SRC, f), 'utf8'));
  if (m) scoped.push(`/* from ${basename(f)} */\n${root ? scope(m[1], root) : m[1].trim()}`);
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
    const body = readFileSync(join(CARDS, group, f), 'utf8').replace(/<!-- capture:([\w-]+) -->/g, (_, n) => captured(n));
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
