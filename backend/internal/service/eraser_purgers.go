package service

// AddPurger makes account erasure also run p, after the purgers already
// wired, in the same transaction. Growth features add their own rows this
// way instead of replacing a slot of EraserDeps (several features can add
// theirs without knowing about each other).
func (e *AccountEraser) AddPurger(p UserDataPurger) {
	if p == nil {
		return
	}
	if e.d.Calculators == nil {
		e.d.Calculators = p
		return
	}
	e.d.Calculators = PurgeAll(e.d.Calculators, p)
}
