import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { Divider } from './Divider'

interface Props {
  left: ReactNode
  right: ReactNode
  /** When false only the left pane is shown. */
  showRight: boolean
  storageKey?: string
}

// Either pane can be squeezed down to a sliver, so that a wide diff or a wide
// preview can take almost the whole window without being switched off.
const MIN_RATIO = 0.04
const MAX_RATIO = 0.96
const DEFAULT_RATIO = 0.55
const KEY_STEP = 0.02

/** SplitPane shows the diff and the preview side by side, with a draggable divider. */
export function SplitPane({ left, right, showRight, storageKey = 'sa.split' }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [ratio, setRatio] = useState<number>(() => {
    const stored = window.localStorage.getItem(storageKey)
    if (stored === null) return DEFAULT_RATIO
    const value = Number(stored)
    return value >= MIN_RATIO && value <= MAX_RATIO ? value : DEFAULT_RATIO
  })

  useEffect(() => {
    window.localStorage.setItem(storageKey, String(ratio))
  }, [ratio, storageKey])

  const clamp = (value: number) => Math.min(MAX_RATIO, Math.max(MIN_RATIO, value))

  const onDrag = useCallback((clientX: number) => {
    const container = containerRef.current
    if (!container) return
    const rect = container.getBoundingClientRect()
    if (rect.width === 0) return
    setRatio(clamp((clientX - rect.left) / rect.width))
  }, [])

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
    <div className="split" ref={containerRef}>
      <div className="split-pane" style={{ width: `${ratio * 100}%` }}>
        {left}
      </div>
      <Divider
        label="Resize the preview"
        onDrag={onDrag}
        onReset={() => setRatio(DEFAULT_RATIO)}
        onNudge={(direction) => setRatio((r) => clamp(r + direction * KEY_STEP))}
      />
      <div className="split-pane" style={{ width: `${(1 - ratio) * 100}%` }}>
        {right}
      </div>
    </div>
  )
}
