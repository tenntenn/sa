interface Props {
  small?: boolean
}

/** MoIcon is mo's own mark (https://github.com/k1LoW/mo, MIT), not a generic
 * icon, so the "mo" toggle reads as mo rather than as some plugin. */
export function MoIcon({ small }: Props) {
  const size = small ? 14 : 16
  return (
    <svg
      className="icon"
      width={size}
      height={size}
      viewBox="0 0 1200 1200"
      fill="currentColor"
      aria-hidden="true"
      style={{ flex: 'none' }}
    >
      <path d="M248,200l51.642,0l214.359,800l-99.436,0l-207.994,-776.243c8.338,-14.209 23.777,-23.757 41.428,-23.757Zm107.706,0l99.436,0l214.359,800l-99.436,0l-214.359,-800Zm155.5,0l440.794,0c26.492,0 48,21.508 48,48l0,704c0,26.492 -21.508,48 -48,48l-226.435,0l-214.359,-800Zm-152.704,800l-110.502,0c-26.492,0 -48,-21.508 -48,-48l0,-543.536l158.502,591.536Z" />
    </svg>
  )
}
