import { marked } from 'marked'
import { escapeHTML, sanitize } from './markdown'

const MARKED_OPTIONS = { async: false, gfm: true, breaks: false } as const

interface NotebookOutput {
  output_type?: string
  text?: string | string[]
  data?: Record<string, string | string[]>
  ename?: string
  evalue?: string
  traceback?: string[]
}

interface NotebookCell {
  cell_type?: string
  source?: string | string[]
  execution_count?: number | null
  outputs?: NotebookOutput[]
}

interface NotebookDoc {
  cells?: unknown
}

/**
 * renderNotebook renders the cells of a Jupyter notebook (nbformat 4):
 * Markdown cells as Markdown, code cells as their source plus outputs,
 * raw cells verbatim.
 *
 * Unlike renderMarkdown, cells are not wrapped with the source lines they
 * came from: a notebook's cell content does not correspond to the raw
 * .ipynb JSON's line numbers the way a plain Markdown file's blocks do, so a
 * selection inside a notebook preview cannot be turned into a line-anchored
 * comment.
 *
 * The result is sanitised the same way renderMarkdown's is: the notebook
 * comes from a diff, which is not trusted input.
 */
export function renderNotebook(source: string): string {
  let doc: NotebookDoc
  try {
    doc = JSON.parse(source) as NotebookDoc
  } catch (err) {
    return errorBlock(`This notebook is not valid JSON: ${err instanceof Error ? err.message : String(err)}`)
  }
  if (!Array.isArray(doc.cells)) {
    return errorBlock('This notebook has no cells to show.')
  }
  return (doc.cells as NotebookCell[]).map(renderCell).join('')
}

function renderCell(cell: NotebookCell): string {
  switch (cell.cell_type) {
    case 'markdown':
      return `<div class="nb-cell nb-cell-markdown">${renderMarkdownSource(joinText(cell.source))}</div>`
    case 'code':
      return renderCodeCell(cell)
    default:
      return `<pre class="nb-cell nb-cell-raw">${escapeHTML(joinText(cell.source))}</pre>`
  }
}

function renderMarkdownSource(source: string): string {
  const html = marked.parse(source, MARKED_OPTIONS)
  return sanitize(typeof html === 'string' ? html : '')
}

function renderCodeCell(cell: NotebookCell): string {
  const label = typeof cell.execution_count === 'number' ? `In [${cell.execution_count}]:` : 'In [ ]:'
  const outputs = (cell.outputs ?? []).map(renderOutput).join('')
  return (
    `<div class="nb-cell nb-cell-code">` +
    `<div class="nb-cell-label">${escapeHTML(label)}</div>` +
    `<pre class="nb-code"><code>${escapeHTML(joinText(cell.source))}</code></pre>` +
    outputs +
    `</div>`
  )
}

function renderOutput(out: NotebookOutput): string {
  switch (out.output_type) {
    case 'stream':
      return `<pre class="nb-output nb-stream">${escapeHTML(joinText(out.text))}</pre>`
    case 'error': {
      const traceback = stripAnsi((out.traceback ?? []).join('\n'))
      const text = traceback || `${out.ename ?? 'Error'}: ${out.evalue ?? ''}`
      return `<pre class="nb-output nb-error">${escapeHTML(text)}</pre>`
    }
    case 'execute_result':
    case 'display_data':
      return renderDataBundle(out.data ?? {})
    default:
      return ''
  }
}

// renderDataBundle picks the richest representation out of a MIME bundle,
// the way Jupyter's own front ends do: rendered HTML over an image over
// plain text.
function renderDataBundle(data: Record<string, string | string[]>): string {
  if (data['text/html']) {
    return `<div class="nb-output nb-html">${sanitize(joinText(data['text/html']))}</div>`
  }
  for (const mime of ['image/png', 'image/jpeg', 'image/svg+xml']) {
    const raw = data[mime]
    if (!raw) continue
    const content = joinText(raw)
    const src = mime === 'image/svg+xml' ? `data:${mime};utf8,${encodeURIComponent(content)}` : `data:${mime};base64,${content}`
    // The bundle comes from a diff and is not trusted: escaping the src
    // keeps a hostile "base64" payload from breaking out of the attribute.
    return `<div class="nb-output nb-image"><img src="${escapeHTML(src)}" alt="notebook output" /></div>`
  }
  if (data['text/plain']) {
    return `<pre class="nb-output">${escapeHTML(joinText(data['text/plain']))}</pre>`
  }
  return ''
}

function joinText(text: string | string[] | undefined): string {
  return Array.isArray(text) ? text.join('') : (text ?? '')
}

// stripAnsi removes the color escapes Jupyter tracebacks are typically sent
// with, which render as literal noise outside a terminal.
function stripAnsi(s: string): string {
  // eslint-disable-next-line no-control-regex
  return s.replace(/\x1b\[[0-9;]*[A-Za-z]/g, '')
}

function errorBlock(message: string): string {
  return `<p class="error">${escapeHTML(message)}</p>`
}
