import type * as React from "react"

/**
 * `onMouseDown` for submit buttons of forms that validate on blur.
 *
 * Pressing the button would blur the field being edited; its error then
 * renders above the button and pushes it down before mouseup, so the click
 * never lands and the visitor has to click twice. Keeping focus in the field
 * lets the click submit, which validates every field at once and focuses the
 * first invalid one. Keyboard and assistive-technology activation are
 * unaffected (they do not send mousedown).
 */
export function keepFocusOnPress(event: React.MouseEvent<HTMLElement>) {
  if (event.button === 0) event.preventDefault()
}
