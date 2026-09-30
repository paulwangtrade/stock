/**
 * Node 直接跑前端单测时，补上 Vite 会自动加上的 .js 后缀。
 */
export async function resolve(specifier, context, nextResolve) {
  if (
    (specifier.startsWith('./') || specifier.startsWith('../')) &&
    !/\.[a-z0-9]+$/i.test(specifier)
  ) {
    return nextResolve(`${specifier}.js`, context)
  }
  return nextResolve(specifier, context)
}
