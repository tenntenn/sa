interface Props {
  name: string
  small?: boolean
}

/** Icon renders one glyph from the subsetted icon font in styles.css. */
export function Icon({ name, small }: Props) {
  return (
    <span className={`icon${small ? ' sm' : ''}`} aria-hidden="true">
      {name}
    </span>
  )
}
