/**
 * Node 测试用：把相对路径的无后缀 import 补成 .js。
 * Vite / esbuild 本身会解析；本加载器只给 node --test。
 */
export async function resolve(specifier, context, nextResolve) {
  if (
    (specifier.startsWith('./') || specifier.startsWith('../')) &&
    !/\.[a-zA-Z0-9]+$/.test(specifier)
  ) {
    return nextResolve(`${specifier}.js`, context)
  }
  return nextResolve(specifier, context)
}
