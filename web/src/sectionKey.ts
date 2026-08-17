/**
 * sectionKey identifies one file within one review round.
 *
 * A FileDiff's own id is only unique within the diff it came from (the
 * server derives it from the file's path and its position in that one
 * diff), so two rounds that both touch, say, README.md as their first file
 * carry the identical fileId. Anything that tracks per-file UI state across
 * a page holding several rounds at once - a fold override, a scroll anchor,
 * an active-file pointer - has to key on the pair, never on fileId alone.
 */
export function sectionKey(diffId: string, fileId: string): string {
  return `${diffId}:${fileId}`
}
