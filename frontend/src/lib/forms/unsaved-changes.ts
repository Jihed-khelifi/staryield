"use client"

import { useEffect, useRef } from "react"

/**
 * Warn before losing unsaved form changes:
 *  - closing/reloading the tab → the browser's own "Leave site?" prompt;
 *  - following an in-app link → a confirm dialog; cancelling keeps the user
 *    on the page (the click is stopped before Next's router sees it).
 * Links with a target, modifier keys or downloads are left alone.
 */
export function useUnsavedChangesGuard(dirty: boolean, message: string) {
  const latest = useRef({ dirty, message })
  useEffect(() => {
    latest.current = { dirty, message }
  })

  useEffect(() => {
    if (!dirty) return
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      // Legacy browsers need returnValue set to show the prompt.
      event.returnValue = ""
    }
    const onClick = (event: MouseEvent) => {
      if (!latest.current.dirty || event.defaultPrevented) return
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
      const anchor = (event.target as Element | null)?.closest?.("a[href]") as HTMLAnchorElement | null
      if (!anchor || anchor.target === "_blank" || anchor.hasAttribute("download")) return
      const url = new URL(anchor.href, window.location.href)
      if (url.origin !== window.location.origin) return
      if (url.pathname === window.location.pathname && url.search === window.location.search) return
      if (!window.confirm(latest.current.message)) {
        event.preventDefault()
        event.stopImmediatePropagation()
      }
    }
    window.addEventListener("beforeunload", onBeforeUnload)
    document.addEventListener("click", onClick, true)
    return () => {
      window.removeEventListener("beforeunload", onBeforeUnload)
      document.removeEventListener("click", onClick, true)
    }
  }, [dirty])
}
