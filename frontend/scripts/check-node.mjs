/**
 * 安装期 Node 版本门禁：仅支持 Node 22 LTS。
 * 由 package.json preinstall 调用；npm install 时若版本不匹配会失败并给出明确提示。
 */
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const pkg = JSON.parse(readFileSync(join(__dirname, '..', 'package.json'), 'utf8'))
const required = pkg.engines?.node ?? '>=22 <23'

const major = Number(process.versions.node.split('.')[0])
const ok = major === 22

if (!ok) {
  console.error(
    [
      '',
      '╔══════════════════════════════════════════════════════════╗',
      '║  Unsupported Node.js version                             ║',
      '╠══════════════════════════════════════════════════════════╣',
      `║  Current : v${process.versions.node.padEnd(45)}║`,
      `║  Required: ${String(required).padEnd(46)}║`,
      '║  Tip     : nvm use / fnm use (see frontend/.nvmrc)       ║',
      '╚══════════════════════════════════════════════════════════╝',
      '',
    ].join('\n'),
  )
  process.exit(1)
}
