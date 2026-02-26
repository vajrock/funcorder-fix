package testpkg

type Worker struct{ active bool }

func NewWorker() *Worker   { return &Worker{} }
func (w *Worker) prepare() {}
func (w *Worker) process() {}
func (w *Worker) Start()   {}
func (w *Worker) Stop()    {}
