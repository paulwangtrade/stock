/** Node ESM：为相对路径无后缀 import 补上 .js；解析 wailsjs 目录导入（仅测试用） */
import { pathToFileURL } from 'node:url'
import { dirname, join, normalize } from 'node:path'
import { fileURLToPath } from 'node:url'
import { existsSync } from 'node:fs'

export async function resolve(specifier, context, nextResolve) {
  // Directory package imports used by Wails bindings
  if (
    typeof specifier === 'string'
    && (specifier.endsWith('/wailsjs/runtime') || specifier.endsWith('\\wailsjs\\runtime'))
  ) {
    const parentPath = context.parentURL ? fileURLToPath(context.parentURL) : ''
    const abs = normalize(join(dirname(parentPath), specifier, 'runtime.js'))
    if (existsSync(abs)) {
      return { shortCircuit: true, url: pathToFileURL(abs).href }
    }
  }

  if (
    specifier.startsWith('.')
    && !specifier.endsWith('.js')
    && !specifier.endsWith('.mjs')
    && !specifier.endsWith('.json')
    && !specifier.includes('?')
  ) {
    // Prefer explicit file: foo → foo.js
    try {
      return await nextResolve(`${specifier}.js`, context)
    } catch {
      // Prefer package directory main: foo/ → foo/runtime.js | foo/index.js
      if (context.parentURL) {
        const parentPath = fileURLToPath(context.parentURL)
        const absDir = normalize(join(dirname(parentPath), specifier))
        for (const name of ['runtime.js', 'index.js', 'App.js']) {
          const candidate = join(absDir, name)
          if (existsSync(candidate)) {
            return { shortCircuit: true, url: pathToFileURL(candidate).href }
          }
        }
      }
    }
  }
  return nextResolve(specifier, context)
}
