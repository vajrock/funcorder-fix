package testpkg

type MyService struct{ value int }

func (ms *MyService) NewSnapshot() *MyService { return &MyService{} }
func (ms *MyService) Run()                   {}
func (ms *MyService) Stop()                  {}
func (ms *MyService) helper()                {}
