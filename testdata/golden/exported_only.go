package testpkg

type Worker struct{ active bool }

func NewWorker() *Worker   { return &Worker{} }
func (w *Worker) Start()   {}
func (w *Worker) Stop()    {}
func (w *Worker) prepare() {}
func (w *Worker) process() {}
