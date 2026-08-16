import type { Comment, Group, Preview, Status } from './types'

/** groupFromLocation reads the group name out of the URL path. */
export function groupFromLocation(): string {
  const name = window.location.pathname.replace(/^\/+|\/+$/g, '')
  return name === '' ? 'default' : name
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(path, init)
  if (!resp.ok) {
    const text = (await resp.text()).trim()
    throw new Error(text || resp.statusText)
  }
  return (await resp.json()) as T
}

export function getStatus(): Promise<Status> {
  return request<Status>('/_/api/status')
}

export function getGroup(group: string): Promise<Group> {
  return request<Group>(`/_/api/groups/${encodeURIComponent(group)}`)
}

export function getPreview(group: string, diffId: string, fileId: string): Promise<Preview> {
  return request<Preview>(
    `/_/api/groups/${encodeURIComponent(group)}/diffs/${encodeURIComponent(diffId)}` +
      `/files/${encodeURIComponent(fileId)}/preview`,
  )
}

export interface NewComment {
  diffId: string
  fileId: string
  path: string
  side: 'new' | 'old'
  startLine: number
  endLine: number
  body: string
  snippet: string
}

export function addComment(group: string, comment: NewComment): Promise<Comment> {
  return request<Comment>(`/_/api/groups/${encodeURIComponent(group)}/comments`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(comment),
  })
}

export function updateComment(
  group: string,
  id: string,
  patch: { body?: string; resolved?: boolean },
): Promise<Comment> {
  return request<Comment>(`/_/api/groups/${encodeURIComponent(group)}/comments/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
}

export async function deleteComment(group: string, id: string): Promise<void> {
  const resp = await fetch(
    `/_/api/groups/${encodeURIComponent(group)}/comments/${encodeURIComponent(id)}`,
    { method: 'DELETE' },
  )
  if (!resp.ok) throw new Error((await resp.text()).trim() || resp.statusText)
}

export async function deleteDiff(group: string, diffId: string): Promise<void> {
  const resp = await fetch(
    `/_/api/groups/${encodeURIComponent(group)}/diffs/${encodeURIComponent(diffId)}`,
    { method: 'DELETE' },
  )
  if (!resp.ok) throw new Error((await resp.text()).trim() || resp.statusText)
}

export async function getPrompt(group: string): Promise<string> {
  const resp = await fetch(`/_/api/groups/${encodeURIComponent(group)}/prompt`)
  if (!resp.ok) throw new Error((await resp.text()).trim() || resp.statusText)
  return await resp.text()
}

/**
 * subscribe listens to server sent events and calls onChange whenever the
 * given group changed. It returns an unsubscribe function.
 */
export function subscribe(group: string, onChange: () => void): () => void {
  const source = new EventSource('/_/events')
  source.onmessage = (ev) => {
    try {
      const data = JSON.parse(ev.data) as { type?: string; group?: string }
      if (data.type === 'change' && (!data.group || data.group === group)) onChange()
    } catch {
      onChange()
    }
  }
  return () => source.close()
}
