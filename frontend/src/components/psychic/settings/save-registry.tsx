"use client"

import * as React from "react"

import type { SaveState } from "@/components/feedback/save-status"

/**
 * One page-level "Save Changes" for several independent forms (Figma
 * 1219:1284 has a single button in the header). Each card registers how to
 * save itself and whether it has unsaved changes; `saveAll` saves the dirty
 * cards in page order. Only the first card that fails validation moves the
 * focus, so keyboard and screen-reader users land on the first problem.
 *
 * Shared settings sections can join the same button by calling
 * `useSettingsSection` with their own id.
 */
export type SaveOptions = { focus: boolean }
export type SectionSaver = (options: SaveOptions) => Promise<boolean>

type Section = { order: number; dirty: boolean; save: SectionSaver }

type Registry = {
  register: (id: string, order: number, save: SectionSaver) => () => void
  setDirty: (id: string, dirty: boolean) => void
  saveAll: () => Promise<boolean>
  dirty: boolean
  state: SaveState
}

const SaveRegistryContext = React.createContext<Registry | null>(null)

export function SettingsSaveProvider({ children }: { children: React.ReactNode }) {
  const sections = React.useRef(new Map<string, Section>())
  const [dirtyIds, setDirtyIds] = React.useState<readonly string[]>([])
  const [state, setState] = React.useState<SaveState>("idle")

  const setDirty = React.useCallback((id: string, dirty: boolean) => {
    const section = sections.current.get(id)
    if (section) section.dirty = dirty
    setDirtyIds((ids) => {
      const has = ids.includes(id)
      if (dirty === has) return ids
      return dirty ? [...ids, id] : ids.filter((x) => x !== id)
    })
    if (dirty) setState((s) => (s === "saving" ? s : "dirty"))
  }, [])

  const register = React.useCallback(
    (id: string, order: number, save: SectionSaver) => {
      const existing = sections.current.get(id)
      sections.current.set(id, { order, dirty: existing?.dirty ?? false, save })
      return () => {
        sections.current.delete(id)
        setDirtyIds((ids) => ids.filter((x) => x !== id))
      }
    },
    []
  )

  const saveAll = React.useCallback(async () => {
    const pending = [...sections.current.entries()].filter(([, s]) => s.dirty).sort(([, a], [, b]) => a.order - b.order)
    if (pending.length === 0) return true
    setState("saving")
    let focus = true
    let ok = true
    for (const [, section] of pending) {
      let saved = false
      try {
        saved = await section.save({ focus })
      } catch {
        saved = false
      }
      if (!saved) {
        ok = false
        focus = false
      }
    }
    setState(ok ? "saved" : "error")
    return ok
  }, [])

  const value = React.useMemo<Registry>(
    () => ({ register, setDirty, saveAll, dirty: dirtyIds.length > 0, state: dirtyIds.length > 0 && state === "saved" ? "dirty" : state }),
    [register, setDirty, saveAll, dirtyIds, state]
  )
  return <SaveRegistryContext.Provider value={value}>{children}</SaveRegistryContext.Provider>
}

/** Register a card with the page's "Save Changes" (no-op outside a provider). */
export function useSettingsSection(id: string, order: number, dirty: boolean, save: SectionSaver) {
  const registry = React.useContext(SaveRegistryContext)
  const latest = React.useRef(save)
  React.useEffect(() => {
    latest.current = save
  })
  const register = registry?.register
  const setDirty = registry?.setDirty
  React.useEffect(() => {
    if (!register) return
    return register(id, order, (options) => latest.current(options))
  }, [register, id, order])
  React.useEffect(() => {
    setDirty?.(id, dirty)
  }, [setDirty, id, dirty])
}

/** The page-level save state and action. */
export function useSettingsSave() {
  const registry = React.useContext(SaveRegistryContext)
  return {
    dirty: registry?.dirty ?? false,
    state: registry?.state ?? ("idle" as SaveState),
    saveAll: registry?.saveAll ?? (async () => true),
  }
}
