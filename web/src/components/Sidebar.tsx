import type { Comment, Diff, Status } from '../types'
import { filePath } from '../types'
import { client } from '../client'

interface Props {
  /** width in pixels; 0 collapses the file list out of the way and null lets
   * it fill the space, which is what a phone does. */
  width: number | null
  group: string
  diffs: Diff[]
  comments: Comment[]
  status: Status | null
  selected: { diffId: string; fileId: string } | null
  onSelect: (diffId: string, fileId: string) => void
  onChanged: () => void
}

export function Sidebar({
  width,
  group,
  diffs,
  comments,
  status,
  selected,
  onSelect,
  onChanged,
}: Props) {
  const commentCount = (diffId: string, fileId: string) =>
    comments.filter((c) => c.diffId === diffId && c.fileId === fileId && !c.resolved).length

  return (
    <aside
      className={`sidebar${width === 0 ? ' collapsed' : ''}${width === null ? ' fill' : ''}`}
      style={width === null ? undefined : { width }}
      aria-hidden={width === 0}
    >
      {diffs.length === 0 && <p className="empty">No diff yet.</p>}

      {diffs.map((diff) => (
        <div className="diff-group" key={diff.id}>
          <div className="diff-group-header">
            <span className="diff-group-title" title={new Date(diff.createdAt).toLocaleString()}>
              {diff.title}
            </span>
            {!client.isStatic && (
              <button
                className="ghost danger"
                title="Remove this diff"
                onClick={() => {
                  void client.deleteDiff(group, diff.id).then(onChanged)
                }}
              >
                ×
              </button>
            )}
          </div>
          <ul className="file-list">
            {diff.files.map((file) => {
              const active = selected?.diffId === diff.id && selected.fileId === file.id
              const count = commentCount(diff.id, file.id)
              return (
                <li key={file.id}>
                  <button
                    className={`file-item${active ? ' active' : ''}`}
                    onClick={() => onSelect(diff.id, file.id)}
                  >
                    <span className={`dot status-${file.status}`} title={file.status} />
                    <span className="file-path" title={filePath(file)}>
                      {filePath(file)}
                    </span>
                    {file.isMarkdown && <span className="badge sm" title="Previewable with mo">md</span>}
                    {count > 0 && <span className="badge sm warn">{count}</span>}
                    <span className="stat add">+{file.additions}</span>
                    <span className="stat del">-{file.deletions}</span>
                  </button>
                </li>
              )
            })}
          </ul>
        </div>
      ))}

      {status && status.groups.length > 1 && (
        <div className="groups">
          <div className="groups-title">Groups</div>
          <ul>
            {status.groups.map((g) => (
              <li key={g.name}>
                <a className={g.name === group ? 'active' : ''} href={g.url}>
                  {g.name}
                  <span className="hint">
                    {g.diffs} diff(s){g.unresolved > 0 ? `, ${g.unresolved} open` : ''}
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </div>
      )}
    </aside>
  )
}
