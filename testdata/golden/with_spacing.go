package testpkg

type SpacedService struct {
	count int
}

func (ss *SpacedService) Start() {
	ss.count++
}
func (ss *SpacedService) Stop() {
	ss.count = 0

	// blank line inside body above
}


func (ss *SpacedService) cleanup() {
	ss.count = 0
}



func (ss *SpacedService) validate() bool {
	return ss.count > 0
}

