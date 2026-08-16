import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'

interface Props {
  left: ReactNode
  right: ReactNode
  /** When false only the left pane is shown. */
  showRight: boolean
  storageKey?: string
}

const MIN_RATIO = 0.2
const MAX_RATIO = 0.8

/** SplitPane shows the diff and the preview side by side with a draggable divider. */
export function SplitPane({ left, right, showRight, storageKey = 'sa.split' }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [ratio, setRatio] = useState<number>(() => {
    const stored = Number(window.localStorage.getItem(storageKey))
    return stored >= MIN_RATIO && stored <= MAX_RATIO ? stored : 0.55
  })
  const [dragging, setDragging] = useState(false)

  useEffect(() => {
    window.localStorage.setItem(storageKey, String(ratio))
  }, [ratio, storageKey])

  const onPointerMove = useCallback((ev: PointerEvent) => {
    const container = containerRef.current
    if (!container) return
    const rect = container.getBoundingClientRect()
    if (rect.width === 0) return
    const next = (ev.clientX - rect.left) / rect.width
    setRatio(Math.min(MAX_RATIO, Math.max(MIN_RATIO, next)))
  }, [])

  useEffect(() => {
    if (!dragging) return
    const stop = () => setDragging(false)
    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', stop)
    return () => {
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('pointerup', stop)
    }
  }, [dragging, onPointerMove])

  if (!showRight) {
    return (
      <div className="split" ref={containerRef}>
        <div className="split-pane" style={{ width: '100%' }}>
          {left}
        </div>
      </div>
    )
  }

  return (
    <div className={`split${dragging ? ' dragging' : ''}`} ref={containerRef}>
      <div className="split-pane" style={{ width: `${ratio * 100}%` }}>
        {left}
      </div>
      <div
        className="split-divider"
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize the preview"
        tabIndex={0}
        onPointerDown={(ev) => {
          ev.preventDefault()
          setDragging(true)
        }}
        onKeyDown={(ev) => {
          if (ev.key === 'ArrowLeft') setRatio((r) => Math.max(MIN_RATIO, r - 0.02))
          if (ev.key === 'ArrowRight') setRatio((r) => Math.min(MAX_RATIO, r + 0.02))
        }}
      />
      <div className="split-pane" style={{ width: `${(1 - ratio) * 100}%` }}>
        {right}
      </div>
    </div>
  )
}
