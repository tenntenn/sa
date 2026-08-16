import { useRef, useState } from 'react'

interface Props {
  /** onDrag receives the pointer position while the divider is dragged. */
  onDrag: (clientX: number) => void
  /** onReset is called on a double click, to go back to a sensible width. */
  onReset?: () => void
  /** onNudge moves the divider by a step with the arrow keys. */
  onNudge?: (direction: -1 | 1) => void
  label: string
}

/**
 * Divider is the draggable edge between two panes. Every pane in sa is
 * resized with one, so they all behave the same: drag to resize, double click
 * to put the pane away or bring it back, arrow keys once it has focus.
 *
 * The pointer is captured for the whole drag. Without that, dragging towards
 * the Markdown preview stops the moment the pointer crosses into its iframe,
 * because the events are delivered to the framed document instead.
 */
export function Divider({ onDrag, onReset, onNudge, label }: Props) {
  const [dragging, setDragging] = useState(false)
  const pointerID = useRef<number | null>(null)

  const stop = (el: HTMLElement) => {
    if (pointerID.current !== null && el.hasPointerCapture(pointerID.current)) {
      el.releasePointerCapture(pointerID.current)
    }
    pointerID.current = null
    setDragging(false)
    document.body.classList.remove('resizing')
  }

  return (
    <div
      className={`pane-divider${dragging ? ' dragging' : ''}`}
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      tabIndex={0}
      onPointerDown={(ev) => {
        ev.preventDefault()
        ev.currentTarget.setPointerCapture(ev.pointerId)
        pointerID.current = ev.pointerId
        setDragging(true)
        // The cursor and the suppressed text selection hold over the whole
        // window while the drag lasts, not just over the divider.
        document.body.classList.add('resizing')
      }}
      onPointerMove={(ev) => {
        if (pointerID.current === null) return
        ev.preventDefault()
        onDrag(ev.clientX)
      }}
      onPointerUp={(ev) => stop(ev.currentTarget)}
      onPointerCancel={(ev) => stop(ev.currentTarget)}
      onLostPointerCapture={(ev) => stop(ev.currentTarget)}
      onDoubleClick={onReset}
      onKeyDown={(ev) => {
        if (ev.key === 'ArrowLeft') {
          ev.preventDefault()
          onNudge?.(-1)
        }
        if (ev.key === 'ArrowRight') {
          ev.preventDefault()
          onNudge?.(1)
        }
        if (ev.key === 'Enter' || ev.key === ' ') {
          ev.preventDefault()
          onReset?.()
        }
      }}
    />
  )
}
