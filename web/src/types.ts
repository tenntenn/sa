export type LineKind = 'context' | 'add' | 'delete'

export type FileStatus = 'added' | 'deleted' | 'modified' | 'renamed' | 'copied' | 'mode'

export type ViewMode = 'unified' | 'split'

export interface Line {
  kind: LineKind
  oldNumber: number
  newNumber: number
  content: string
  noNewline?: boolean
}

export interface Hunk {
  header: string
  oldStart: number
  oldLines: number
  newStart: number
  newLines: number
  section?: string
  lines: Line[]
}

export interface FileDiff {
  id: string
  oldPath: string
  newPath: string
  status: FileStatus
  isBinary: boolean
  oldMode?: string
  newMode?: string
  additions: number
  deletions: number
  viewMode: ViewMode
  isMarkdown: boolean
  hunks: Hunk[]
}

export interface Diff {
  id: string
  title: string
  baseDir: string
  createdAt: string
  raw: string
  files: FileDiff[]
}

export interface Comment {
  id: string
  group: string
  diffId: string
  fileId: string
  path: string
  side: 'new' | 'old'
  startLine: number
  endLine: number
  body: string
  snippet: string
  /** suggestion is the replacement proposed for the commented lines. */
  suggestion?: string
  resolved: boolean
  createdAt: string
  updatedAt: string
}

export interface Group {
  name: string
  diffs: Diff[] | null
  comments: Comment[] | null
}

export interface GroupSummary {
  name: string
  url: string
  diffs: number
  files: number
  comments: number
  unresolved: number
}

export interface Status {
  app: string
  version: string
  revision?: string
  pid: number
  url: string
  moUrl: string
  moProxyUrl?: string
  moAvailable: boolean
  moError?: string
  groups: GroupSummary[]
}

export interface Preview {
  url: string
  moUrl: string
  path: string
  source: 'worktree' | 'reconstructed'
  complete: boolean
}

/** filePath returns the path a file is identified by. */
export function filePath(file: FileDiff): string {
  return file.newPath || file.oldPath
}
