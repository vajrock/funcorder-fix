package testpkg

type Engine struct{ running bool }

func (e *Engine) NewInstance() *Engine { return &Engine{} }
func (e *Engine) Run()                {}
func (e *Engine) Stop()               {}
func (e *Engine) warmUp()             {}
func (e *Engine) status() bool        { return e.running }
