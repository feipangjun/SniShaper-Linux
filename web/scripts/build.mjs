import * as esbuild from 'esbuild'
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(__dirname, '..')
const dist = path.resolve(root, 'dist')

fs.rmSync(dist, { recursive: true, force: true })
fs.mkdirSync(dist, { recursive: true })

const start = Date.now()

await esbuild.build({
  entryPoints: [path.join(root, 'src', 'main.tsx')],
  bundle: true,
  outfile: path.join(dist, 'main.js'),
  format: 'esm',
  target: 'es2020',
  jsx: 'automatic',
  tsconfigRaw: JSON.stringify({
    compilerOptions: { jsx: 'react-jsx', strict: false },
  }),
  define: {
    'process.env.NODE_ENV': '"production"',
  },
  minify: true,
  legalComments: 'none',
  treeShaking: true,
})

// Update HTML
let html = fs.readFileSync(path.join(root, 'index.html'), 'utf-8')
html = html.replace('/src/main.tsx', '/main.js')
fs.writeFileSync(path.join(dist, 'index.html'), html)

console.log(`✓ built in ${((Date.now() - start) / 1000).toFixed(2)}s`)
