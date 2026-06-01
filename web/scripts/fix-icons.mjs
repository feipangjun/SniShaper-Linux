import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const src = path.resolve(__dirname, '..', 'src')

function convertImports(code) {
  // Match: import { ... } from '@mui/icons-material'
  const regex = /import\s*\{([^}]+)\}\s*from\s+['"`]@mui\/icons-material['"`]\s*/g
  return code.replace(regex, (match, body) => {
    const names = body.split(',').map(s => s.trim()).filter(Boolean)
    const lines = names.map(n => {
      const parts = n.split(/\s+as\s+/i)
      const orig = parts[0].trim()
      const alias = parts[1] ? parts[1].trim() : orig
      return `import ${alias} from '@mui/icons-material/${orig}'`
    })
    // Preserve trailing content by appending what was consumed
    return lines.join('\n')
  })
}

const files = fs.readdirSync(src, { recursive: true }).filter(f => f.endsWith('.tsx') || f.endsWith('.ts'))
for (const file of files) {
  const fp = path.join(src, file)
  let code = fs.readFileSync(fp, 'utf-8')
  const converted = convertImports(code)
  if (converted !== code) {
    fs.writeFileSync(fp, converted)
    console.log(`  converted: ${file}`)
  }
}
console.log('done')
