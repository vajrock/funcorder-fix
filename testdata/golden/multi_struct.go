package testpkg

type Alpha struct {
	name string
}

func (a *Alpha) Show() {
	_ = a.name
}

func (a *Alpha) hide() {
	_ = a.name
}

func helperBetween() {}

type Beta struct {
	id int
}

func (b *Beta) NewCopy() *Beta {
	return &Beta{id: b.id}
}

func (b *Beta) Process() {}

func (b *Beta) internal() {
	_ = b.id
}

