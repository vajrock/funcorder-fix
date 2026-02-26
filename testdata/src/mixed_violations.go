package testpkg

type Engine struct{ running bool }

func (e *Engine) warmUp()             {}
func (e *Engine) Run()                {}
func (e *Engine) NewInstance() *Engine { return &Engine{} }
func (e *Engine) status() bool        { return e.running }
func (e *Engine) Stop()               {}
