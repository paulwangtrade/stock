const fs = require('fs')
const path = require('path')
const vue = fs.readFileSync('D:/stock/frontend/src/components/allStockList.vue', 'utf8')
const app = fs.readFileSync('D:/stock/frontend/wailsjs/go/main/App.js', 'utf8')
const m = vue.match(/import \{([\s\S]*?)\} from "\.\.\/\.\.\/wailsjs\/go\/main\/App"/)
const names = m[1].split(',').map((s) => s.trim()).filter(Boolean)
const missing = names.filter((n) => !app.includes('export function ' + n + '('))
console.log('imports', names.join(','))
console.log('missing_app_exports', missing)
const utils = [...vue.matchAll(/from "(\.\.\/utils\/[^"]+)"/g)].map((x) => x[1])
const base = 'D:/stock/frontend/src/components'
for (const u of utils) {
  const full = path.resolve(base, u)
  const ok = fs.existsSync(full) || fs.existsSync(full + '.js')
  console.log(ok ? 'ok_util' : 'MISSING_UTIL', u)
}
