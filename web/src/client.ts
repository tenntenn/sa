import * as api from './api'
import type { Comment, Diff, Status, Verdict } from './types'
import { renderMarkdown } from './markdown'
import { buildPrompt } from './prompt'
import { suggestions } from './suggestion'
import { readSetting, writeSetting } from './storage'

/** PreviewResult is either an embedded mo page or Markdown rendered here. */
export type PreviewResult =
  | {
      kind: 'frame'
      url: string
      moUrl: string
      path: string
      source: string
      complete: boolean
    }
  | {
      kind: 'html'
      html: string
      path: string
      source: string
      complete: boolean
    }

export interface GroupData {
  diffs: Diff[]
  comments: Comment[]
  status: Status | null
  /** reviewedAt is when the review was last submitted, if it was. */
  reviewedAt?: string
  /** reviewVerdict is what that review decided. */
  reviewVerdict?: Verdict
}

/**
 * SaClient is what the UI talks to. The live client uses the sa server; the
 * static one is used by pages written with `sa export`, which have no server
 * behind them and keep comments in the browser.
 */
export interface SaClient {
  readonly isStatic: boolean
  /** exportedAt is set on static pages and tells when the diff was frozen. */
  readonly exportedAt?: string
  load(group: string): Promise<GroupData>
  addComment(group: string, comment: api.NewComment): Promise<void>
  updateComment(group: string, id: string, patch: api.CommentPatch): Promise<void>
  deleteComment(group: string, id: string): Promise<void>
  deleteDiff(group: string, diffId: string): Promise<void>
  /** closeReview drops the whole review: its diffs, comments and hooks. */
  closeReview(group: string): Promise<void>
  prompt(group: string): Promise<string>
  /** submitReview tells everyone waiting that the review is done. It only
   * exists where there is a server to tell. */
  submitReview(group: string, note: string, verdict: Verdict): Promise<void>
  /** preview returns the mo page for a file, embedded or linked. */
  preview(group: string, diffId: string, fileId: string): Promise<PreviewResult>
  /** previewMarkdown renders the Markdown in this page instead, which is
   * what a window too narrow for mo's own layout uses. */
  previewMarkdown(group: string, diffId: string, fileId: string): Promise<PreviewResult>
  subscribe(group: string, onChange: () => void): () => void
}

/** StaticPayload is the data `sa export` embeds into the page. */
export interface StaticPayload {
  version: number
  saVersion?: string
  generatedAt: string
  group: string
  diffs: Diff[]
  comments: Comment[]
  previews: Record<string, { content: string; source: string; complete: boolean; path?: string }>
}

declare global {
  interface Window {
    __SA_DATA__?: StaticPayload
  }
}

function createLiveClient(): SaClient {
  return {
    isStatic: false,
    async load(group) {
      const [g, status] = await Promise.all([api.getGroup(group), api.getStatus()])
      return {
        diffs: g.diffs ?? [],
        comments: g.comments ?? [],
        reviewedAt: g.reviewedAt,
        reviewVerdict: g.reviewVerdict,
        status,
      }
    },
    async addComment(group, comment) {
      await api.addComment(group, comment)
    },
    async updateComment(group, id, patch) {
      await api.updateComment(group, id, patch)
    },
    async deleteComment(group, id) {
      await api.deleteComment(group, id)
    },
    async deleteDiff(group, diffId) {
      await api.deleteDiff(group, diffId)
    },
    async closeReview(group) {
      await api.deleteGroup(group)
    },
    prompt(group) {
      return api.getPrompt(group)
    },
    async submitReview(group, note, verdict) {
      await api.submitReview(group, note, verdict)
    },
    async preview(group, diffId, fileId) {
      const p = await api.getPreview(group, diffId, fileId)
      return {
        kind: 'frame',
        url: p.url,
        moUrl: p.moUrl,
        path: p.path,
        source: p.source,
        complete: p.complete,
      }
    },
    async previewMarkdown(group, diffId, fileId) {
      const file = await api.getFileContent(group, diffId, fileId)
      return {
        kind: 'html',
        html: renderMarkdown(file.content),
        path: file.path,
        source: file.source,
        complete: file.complete,
      }
    },
    subscribe: api.subscribe,
  }
}

function createStaticClient(data: StaticPayload): SaClient {
  const storageKey = `sa:comments:${data.group}:${data.generatedAt}`
  const listeners = new Set<() => void>()

  const read = (): Comment[] => {
    const stored = readSetting(storageKey)
    if (stored) {
      try {
        return JSON.parse(stored) as Comment[]
      } catch {
        // Something else wrote there, or the entry was truncated.
      }
    }
    return data.comments ?? []
  }
  const write = (comments: Comment[]) => {
    writeSetting(storageKey, JSON.stringify(comments))
    listeners.forEach((fn) => fn())
  }

  let nextID = 1
  const newID = () => {
    const existing = new Set(read().map((c) => c.id))
    while (existing.has(`local-${nextID}`)) nextID++
    return `local-${nextID++}`
  }

  return {
    isStatic: true,
    exportedAt: data.generatedAt,
    async load() {
      return { diffs: data.diffs ?? [], comments: read(), status: null }
    },
    async addComment(group, comment) {
      const now = new Date().toISOString()
      const created: Comment = {
        id: newID(),
        group,
        diffId: comment.diffId,
        fileId: comment.fileId,
        path: comment.path,
        side: comment.side,
        startLine: comment.startLine,
        endLine: comment.endLine,
        body: comment.body,
        snippet: comment.snippet,
        suggestions: suggestions(comment.body),
        resolved: false,
        createdAt: now,
        updatedAt: now,
      }
      write([...read(), created])
    },
    async updateComment(_group, id, patch) {
      write(
        read().map((c) =>
          c.id === id
            ? {
                ...c,
                body: patch.body ?? c.body,
                suggestions: suggestions(patch.body ?? c.body),
                resolved: patch.resolved ?? c.resolved,
                updatedAt: new Date().toISOString(),
              }
            : c,
        ),
      )
    },
    async deleteComment(_group, id) {
      write(read().filter((c) => c.id !== id))
    },
    async deleteDiff() {
      throw new Error('an exported page cannot drop a diff')
    },
    async closeReview() {
      throw new Error('an exported page has no review to close')
    },
    async prompt(group) {
      return buildPrompt(group, data.diffs ?? [], read())
    },
    async submitReview() {
      throw new Error('an exported page has no server to submit the review to')
    },
    async preview(_group, diffId, fileId) {
      const entry = data.previews?.[`${diffId}:${fileId}`]
      if (!entry) throw new Error('this page carries no preview for that file')
      return {
        kind: 'html',
        html: renderMarkdown(entry.content),
        path: entry.path ?? '',
        source: entry.source,
        complete: entry.complete,
      }
    },
    async previewMarkdown(group, diffId, fileId) {
      // An exported page renders its own Markdown either way.
      return this.preview(group, diffId, fileId)
    },
    subscribe(_group, onChange) {
      listeners.add(onChange)
      return () => listeners.delete(onChange)
    },
  }
}

export const client: SaClient = window.__SA_DATA__
  ? createStaticClient(window.__SA_DATA__)
  : createLiveClient()

export type { NewComment } from './api'
