package service

// AddPurger makes account erasure also run p, after the purgers already
// wired, in the same transaction. Growth features add their own rows this
// way instead of taking a slot of EraserDeps: p goes to its own list, so
// several features can add theirs without knowing about each other, and a
// later SetCalculators (or any slot change) never drops it.
func (e *AccountEraser) AddPurger(p UserDataPurger) {
	if p == nil {
		return
	}
	e.d.Extra = append(e.d.Extra, p)
}
