#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { Resvg } = require('@resvg/resvg-js');

const projectRoot = path.resolve(__dirname, '..');
const sourceDir = path.join(projectRoot, 'node_modules', 'simple-icons', 'icons');
const targetDir = path.join(projectRoot, 'internal', 'ui', 'assets', 'icons');

const icons = {
  claudecode: 'claude.svg',
  codex: 'openai.svg',
  geminicli: 'googlegemini.svg',
  githubcopilot: 'githubcopilot.svg',
};

if (!fs.existsSync(sourceDir)) {
  console.error('Missing source icons. Run `npm install` first.');
  process.exit(1);
}

fs.mkdirSync(targetDir, { recursive: true });
for (const stale of ['lobehub.svg', 'lobehub.png']) {
  const stalePath = path.join(targetDir, stale);
  if (fs.existsSync(stalePath)) {
    fs.unlinkSync(stalePath);
    console.log(`Removed ${path.relative(projectRoot, stalePath)}`);
  }
}

for (const [targetName, sourceFile] of Object.entries(icons)) {
  const from = path.join(sourceDir, sourceFile);
  const toSVG = path.join(targetDir, `${targetName}.svg`);
  const toPNG = path.join(targetDir, `${targetName}.png`);

  if (!fs.existsSync(from)) {
    console.error(`Missing icon source: ${from}`);
    process.exit(1);
  }

  const svg = fs.readFileSync(from, 'utf8');
  fs.writeFileSync(toSVG, svg);

  const normalized = svg
    .replaceAll('currentColor', '#D6DEE8')
    .replace(/fill="[^"]*"/g, 'fill="#D6DEE8"');
  const resvg = new Resvg(normalized, {
    fitTo: { mode: 'width', value: 128 },
    background: 'rgba(0,0,0,0)',
  });
  const png = resvg.render().asPng();
  fs.writeFileSync(toPNG, png);

  console.log(`Generated ${path.relative(projectRoot, toSVG)}`);
  console.log(`Generated ${path.relative(projectRoot, toPNG)}`);
}
