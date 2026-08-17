import { useEffect, useRef, useState } from 'react'
import type { FileDiff, Status } from '../types'
import { filePath } from '../types'
import { client, type PreviewResult } from '../client'
import { readSetting, writeSetting } from '../storage'
import { Icon } from './Icon'
import { MoIcon } from './MoIcon'
import { CommentForm } from './CommentThread'

interface Props {
  group: string
  diffId: string | null
  file: FileDiff | null
  status: Status | null
  /** narrow is set on a phone, where mo's own layout has no room. */
  narrow?: boolean
  /** scrollTo is the fraction of the diff that has been scrolled past, or
   * null when the diff is not driving the preview. */
  scrollTo?: number | null
  /** sync is whether the preview is following the diff, and onSync turns
   * that off and on where the reader is looking. */
  sync?: boolean
  onSync?: (on: boolean) => void
  onChanged: () => void
}

/** SelectionMenu is the small floating toolbar a text selection in the
 * preview opens, and then the comment form it turns into. top and left are
 * viewport coordinates, taken from the selection itself, so the menu tracks
 * it regardless of how the preview is scrolled. It sits right where the
 * selection is - centred over it, not off at whichever end a drag happened
 * to start from - so a finger or a pointer never has to travel far to reach
 * it, and placement flips below the selection when there is no room above. */
interface SelectionMenu {
  startLine: number
  endLine: number
  text: string
  top: number
  left: number
  placement: 'above' | 'below'
  drafting: boolean
}

/** blockLines finds the nearest ancestor of node carrying data-ln - the
 * source line range renderMarkdown attached to that block - without walking
 * past root. */
function blockLines(node: Node | null, root: HTMLElement): [number, number] | null {
  let el: Element | null = node instanceof Element ? node : (node?.parentElement ?? null)
  while (el && el !== root) {
    const ln = el.getAttribute('data-ln')
    if (ln) {
      const [start, end] = ln.split('-').map(Number)
      return [start, Number.isFinite(end) ? end : start]
    }
    el = el.parentElement
  }
  return null
}

/** PreviewKind is which of the two previews is showing. sbnn renders one
 * itself; mo is the other, richer one, in a frame. */
export type PreviewKind = 'preview' | 'mo'

const RENDERER_KEY = 'sbnn.preview.renderer'

/**
 * PreviewPane shows the Markdown preview next to the diff.
 *
 * In the live app the preview is rendered by mo. mo forbids framing with
 * "frame-ancestors 'none'", so sbnn serves it through its own loopback proxy,
 * which relaxes that one directive for sbnn's origin. An exported page has no
 * mo behind it and renders the frozen Markdown itself.
 *
 * A phone does the same: mo keeps its own sidebar inside the frame, which
 * would leave a column too narrow to read, so the Markdown is rendered here
 * and mo is one tap away in its own window.
 */
export function PreviewPane({
  group,
  diffId,
  file,
  status,
  narrow = false,
  scrollTo = null,
  sync = false,
  onSync,
  onChanged,
}: Props) {
  const [preview, setPreview] = useState<PreviewResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const [openingMo, setOpeningMo] = useState(false)
  // selectionMenu is null whenever nothing is selected. Line numbers are
  // only trustworthy while the preview carries the whole file: a partial
  // preview marks the gaps it skipped rather than keeping the file's own
  // numbering past them, so commenting from a selection is limited to a
  // complete preview.
  const [selectionMenu, setSelectionMenu] = useState<SelectionMenu | null>(null)

  // Which preview to use is the reader's to choose. sbnn's own is the
  // default: it needs nothing installed, it follows the diff as it scrolls,
  // and it is drawn in this page rather than in a frame. mo renders more,
  // and is one click away. A phone and an exported page have no embedded mo
  // to choose between.
  const [chosen, setChosen] = useState<PreviewKind>(
    () => (readSetting(RENDERER_KEY) === 'mo' ? 'mo' : 'preview'),
  )
  const forced = narrow || client.isStatic
  const kind: PreviewKind = forced ? 'preview' : chosen
  const previewable = file !== null && file.isMarkdown && diffId !== null
  const renderHere = kind === 'preview'
  const body = useRef<HTMLDivElement>(null)
  // Whether a selection drag is in progress right now; see the effect below
  // that tracks it.
  const dragging = useRef(false)

  useEffect(() => {
    writeSetting(RENDERER_KEY, kind)
  }, [kind])

  // Follow the diff. Only the preview rendered here can be moved: mo is
  // framed from another origin, where a page may not touch its scrolling.
  useEffect(() => {
    if (scrollTo === null || renderHere !== true) return
    const el = body.current
    if (!el) return
    const room = el.scrollHeight - el.clientHeight
    if (room <= 0) return
    el.scrollTop = room * scrollTo
  }, [scrollTo, renderHere, preview])

  useEffect(() => {
    // A selection made against the previous file or the previous render of
    // this one no longer means anything once the preview underneath it
    // changes.
    setSelectionMenu(null)
    if (!previewable || !file || !diffId) {
      setPreview(null)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    const load = renderHere
      ? client.previewMarkdown(group, diffId, file.id)
      : client.preview(group, diffId, file.id)
    load
      .then((p) => {
        if (!cancelled) setPreview(p)
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setPreview(null)
          setError(err instanceof Error ? err.message : String(err))
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [group, diffId, file, previewable, renderHere, reloadKey])

  // mo is not embedded on a phone, but it is still the full preview: ask the
  // server for it only when someone actually wants it.
  const openInMo = async () => {
    if (!previewable || !file || !diffId) return
    setOpeningMo(true)
    setError(null)
    try {
      const result = await client.preview(group, diffId, file.id)
      if (result.kind === 'frame' && result.moUrl) {
        window.open(result.moUrl, '_blank', 'noreferrer')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setOpeningMo(false)
    }
  }

  // A selection in the preview opens the small toolbar; an empty one, or one
  // made somewhere else on the page, closes it again. This runs off
  // selectionchange rather than mouseup so that it also catches touch: a
  // phone keeps adjusting the selection by dragging its handles well after
  // the finger that started it has lifted, which fires no mouseup or
  // touchend of its own to hook into.
  const onPreviewSelect = () => {
    // Once the form is open, its own textarea takes focus, which collapses
    // the document selection on its own - that is not the reader clearing
    // anything, so it must not close the form out from under them.
    if (selectionMenu?.drafting) return
    // A real drag pauses - a human one does far more than the three jumps a
    // test sends - and a pause longer than the debounce below used to show
    // the button mid-drag, in the wrong place, only for it to jump again
    // once the mouse actually came up. Nothing is shown before that.
    if (dragging.current) return
    const root = body.current
    if (!root || !preview || preview.kind !== 'html' || !preview.complete) {
      setSelectionMenu(null)
      return
    }
    const sel = window.getSelection()
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) {
      setSelectionMenu(null)
      return
    }
    const range = sel.getRangeAt(0)
    const text = sel.toString()
    if (!root.contains(range.commonAncestorContainer) || text.trim() === '') {
      setSelectionMenu(null)
      return
    }
    const start = blockLines(range.startContainer, root)
    const end = blockLines(range.endContainer, root)
    if (!start || !end) {
      setSelectionMenu(null)
      return
    }
    // Centred over the selection itself, not over whichever end a drag
    // happened to start from - a pointer or a finger is always right next
    // to what it just selected, on either side, and this is never far from
    // it. Placement picks whichever side has more room, since the button
    // this opens on can grow into a whole comment form - there is no fixed
    // height to leave space for ahead of time.
    const rect = range.getBoundingClientRect()
    const above = rect.top > window.innerHeight - rect.bottom
    setSelectionMenu({
      startLine: Math.min(start[0], end[0]),
      endLine: Math.max(start[1], end[1]),
      text,
      top: above ? rect.top : rect.bottom,
      left: rect.left + rect.width / 2,
      placement: above ? 'above' : 'below',
      drafting: false,
    })
  }

  // The listener below is attached once, for the component's whole life, and
  // always calls through this ref rather than closing over onPreviewSelect
  // directly. Re-subscribing on every render raced against the textarea's
  // autoFocus: focusing it fires selectionchange synchronously, as part of
  // the same commit that opened the form, which could reach the outgoing
  // listener - still holding the previous render's closure, with drafting
  // still false in it - before its own cleanup had run and replaced it. That
  // stale closure saw a selection collapsed by nothing more than the
  // textarea taking focus and closed the form it had just opened. A ref is
  // always current the moment it is read, no matter when the event lands.
  const onPreviewSelectRef = useRef(onPreviewSelect)
  onPreviewSelectRef.current = onPreviewSelect

  // selectionchange fires on document, not on the preview element, and very
  // often mid-drag - debounced so the menu settles once the selection stops
  // moving instead of chasing every intermediate change. mousedown/mouseup
  // track whether a drag is in progress right now, at the document level
  // because a drag that started in the preview can end anywhere - the
  // pointer that lifts it is not always still over the pane it started in.
  // Both share one timer: a debounce left over from the last selectionchange
  // of the drag itself would otherwise still be waiting to fire once the
  // pointer comes up, landing after - and overwriting - whatever came of
  // that release, which is exactly the stale-menu bug this whole thing
  // exists to avoid.
  useEffect(() => {
    let timer: number | undefined
    const onChange = () => {
      window.clearTimeout(timer)
      if (dragging.current) return
      timer = window.setTimeout(() => onPreviewSelectRef.current(), 200)
    }
    // Scoped to the previewed text itself: pressing the "+" button (or
    // anything else of ours) is a mousedown too, and treating that as the
    // start of a drag made its own mouseup re-run onPreviewSelect a moment
    // before the click that mouseup was about to become - wiping the menu,
    // and the button along with it, out from under a click that had not
    // happened yet.
    const down = (ev: Event) => {
      const root = body.current
      if (root && ev.target instanceof Node && root.contains(ev.target)) {
        dragging.current = true
      }
    }
    const up = () => {
      if (!dragging.current) return
      dragging.current = false
      // Whatever selectionchange fired during the drag has nothing left to
      // say now that the selection itself is done moving.
      window.clearTimeout(timer)
      onPreviewSelectRef.current()
    }
    document.addEventListener('selectionchange', onChange)
    document.addEventListener('mousedown', down)
    document.addEventListener('touchstart', down)
    document.addEventListener('mouseup', up)
    document.addEventListener('touchend', up)
    return () => {
      document.removeEventListener('selectionchange', onChange)
      document.removeEventListener('mousedown', down)
      document.removeEventListener('touchstart', down)
      document.removeEventListener('mouseup', up)
      document.removeEventListener('touchend', up)
      window.clearTimeout(timer)
    }
  }, [])

  const submitSelectionComment = async (commentBody: string, question: boolean) => {
    if (!selectionMenu || !file || !diffId) return
    await client.addComment(group, {
      diffId,
      fileId: file.id,
      path: filePath(file),
      side: 'new',
      startLine: selectionMenu.startLine,
      endLine: selectionMenu.endLine,
      body: commentBody,
      question,
      snippet: selectionMenu.text,
    })
    setSelectionMenu(null)
    onChanged()
  }

  if (!file) {
    return (
      <section className="preview">
        <div className="preview-header">
          <span className="preview-title">Preview</span>
        </div>
        <p className="empty">Select a file.</p>
      </section>
    )
  }

  const selectionLabel = selectionMenu
    ? `${filePath(file)}:${selectionMenu.startLine}` +
      `${selectionMenu.endLine > selectionMenu.startLine ? `-${selectionMenu.endLine}` : ''}`
    : ''

  return (
    <section className="preview">
      <div className="preview-header">
        <span className="preview-title">Preview</span>
        <span className="path">{filePath(file)}</span>
        {preview && (
          <>
            <span
              className="badge"
              title={`${preview.path}\n${
                preview.source === 'worktree' ? 'the working tree file' : 'rebuilt from the diff'
              }`}
            >
              {preview.source === 'worktree' ? 'tree' : 'rebuilt'}
            </span>
            {!preview.complete && (
              <span className="badge warn" title="A unified diff only carries the changed hunks">
                partial
              </span>
            )}
          </>
        )}
        <span className="spacer" />
        {!forced && (
          <div className="toggle">
            <button
              className={kind === 'preview' ? 'active' : ''}
              onClick={() => setChosen('preview')}
              title="sbnn's own preview - needs nothing installed, follows the diff as it scrolls"
            >
              <Icon name="visibility" small />
              preview
            </button>
            <button
              className={kind === 'mo' ? 'active' : ''}
              onClick={() => setChosen('mo')}
              title="mo - renders more, in a frame, but does not follow the diff"
            >
              <MoIcon small />
              mo
            </button>
          </div>
        )}
        {/* A disabled button swallows hover, so the tooltip explaining why
            lives on a span around it instead - it stays reachable exactly
            when the button itself is not. */}
        {previewable && onSync && (
          <span
            title={
              !renderHere
                ? "Only sbnn's own preview can follow the diff: mo is framed from another origin, where a page may not touch its scrolling"
                : sync
                  ? 'The preview follows the diff; scrolling it yourself stops that'
                  : 'Follow the diff again'
            }
          >
            <button
              className={`ghost icon-only${sync && renderHere ? ' active' : ''}`}
              disabled={!renderHere}
              onClick={() => onSync(!sync)}
            >
              <Icon name="link" />
            </button>
          </span>
        )}
        {!client.isStatic && (
          <span title="Reload the preview">
            <button
              className="ghost icon-only"
              onClick={() => setReloadKey((k) => k + 1)}
              disabled={!previewable}
            >
              <Icon name="refresh" />
            </button>
          </span>
        )}
        {/* One slot, always present when not static: a direct link once mo's
            frame has actually loaded, a button that fetches and opens it
            otherwise. Two different elements swapping in and out by mode
            made the toolbar's width jump between "preview" and "mo". */}
        {!client.isStatic &&
          (preview?.kind === 'frame' && preview.moUrl ? (
            <a className="ghost button" href={preview.moUrl} target="_blank" rel="noreferrer">
              <MoIcon small />
              mo
              <Icon name="open_in_new" small />
            </a>
          ) : (
            <button
              className="ghost"
              onClick={() => void openInMo()}
              disabled={!previewable || openingMo}
            >
              <MoIcon small />
              {openingMo ? 'Opening…' : 'mo'}
              <Icon name="open_in_new" small />
            </button>
          ))}
      </div>

      {!file.isMarkdown ? (
        <p className="empty">{filePath(file)} is not Markdown, so there is nothing to preview.</p>
      ) : loading ? (
        <p className="empty">{renderHere ? 'Rendering…' : 'Asking mo for a preview…'}</p>
      ) : error ? (
        <div className="preview-error">
          <p className="error">{error}</p>
          {status && !status.moAvailable && (
            <p className="hint">
              mo renders a richer preview than sbnn's own, and it is not installed here. Install it
              with <code>brew install k1LoW/tap/mo</code> or grab a binary from{' '}
              <a href="https://github.com/k1LoW/mo/releases" target="_blank" rel="noreferrer">
                the releases page
              </a>
              , then reload — or switch back to <strong>preview</strong>, which needs nothing.
            </p>
          )}
        </div>
      ) : preview?.kind === 'html' ? (
        <>
          <div
            className="markdown"
            ref={body}
            onWheel={() => onSync?.(false)}
            onTouchMove={() => onSync?.(false)}
            onScroll={() => setSelectionMenu(null)}
            dangerouslySetInnerHTML={{ __html: preview.html }}
          />
          {selectionMenu && (
            <div
              className="selection-menu"
              data-placement={selectionMenu.placement}
              style={{ top: selectionMenu.top, left: selectionMenu.left }}
              // Pressing the toolbar must not be read as clicking away from
              // the selection, which would collapse it before Comment is
              // reached.
              onMouseDown={(ev) => ev.preventDefault()}
            >
              {!selectionMenu.drafting ? (
                <button
                  className="ghost selection-add"
                  title="Comment"
                  aria-label="Comment"
                  onClick={() => setSelectionMenu((m) => (m ? { ...m, drafting: true } : m))}
                >
                  +
                </button>
              ) : (
                <CommentForm
                  label={selectionLabel}
                  seed={selectionMenu.text}
                  canSuggest
                  hint="Selected from the preview"
                  onSubmit={submitSelectionComment}
                  onCancel={() => setSelectionMenu(null)}
                />
              )}
            </div>
          )}
        </>
      ) : preview?.kind === 'frame' && preview.url ? (
        <iframe className="preview-frame" src={preview.url} title="Markdown preview" />
      ) : preview?.kind === 'frame' && preview.moUrl ? (
        <div className="preview-error">
          <p className="error">The preview cannot be embedded here.</p>
          <p className="hint">
            <a href={preview.moUrl} target="_blank" rel="noreferrer">
              Open it in mo
            </a>{' '}
            instead.
          </p>
        </div>
      ) : (
        <p className="empty">No preview.</p>
      )}
    </section>
  )
}
