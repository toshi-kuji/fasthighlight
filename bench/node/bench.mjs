import { readFileSync, writeFileSync } from 'node:fs'
const tStart = performance.now()
const [, , variant, input, out] = process.argv
const src = readFileSync(input, 'utf8')

const t0 = performance.now()
const { default: MarkdownIt } = await import('markdown-it')
let md
if (variant === 'hljs') {
  const { default: hljs } = await import('highlight.js')
  md = new MarkdownIt({
    html: true,
    highlight(code, lang) {
      if (lang && hljs.getLanguage(lang)) {
        return '<pre><code class="hljs">' + hljs.highlight(code, { language: lang, ignoreIllegals: true }).value + '</code></pre>'
      }
      return ''
    },
  })
} else {
  // Load only the languages used by the code blocks of the input (ts and js for the default input).
  const { createHighlighter, bundledLanguages } = await import('shiki')
  const langs = [...new Set([...src.matchAll(/^```(\S+)/gm)].map((m) => m[1]))].filter((l) => l in bundledLanguages)
  const hl = await createHighlighter({ themes: ['github-light'], langs })
  md = new MarkdownIt({
    html: true,
    highlight(code, lang) {
      const l = langs.includes(lang) ? lang : 'text'
      return hl.codeToHtml(code, { lang: l, theme: 'github-light' })
    },
  })
}
const tInit = performance.now() - t0

const t1 = performance.now()
let html = md.render(src)
const tFirst = performance.now() - t1
writeFileSync(out, html)
if (process.env.ONCE) process.exit(0)

for (let i = 0; i < 20; i++) md.render(src)
const n = 300, d = []
for (let i = 0; i < n; i++) {
  const t = performance.now()
  html = md.render(src)
  d.push(performance.now() - t)
}
d.sort((a, b) => a - b)
console.log(`${variant} init=${tInit.toFixed(2)}ms first=${tFirst.toFixed(2)}ms warm_median=${d[n / 2].toFixed(3)}ms warm_p95=${d[Math.floor(n * 0.95)].toFixed(3)}ms since_start=${(performance.now() - tStart).toFixed(2)}ms bytes=${Buffer.byteLength(html)}`)
