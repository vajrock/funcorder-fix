package testpkg

type Processor struct {
	input  []string
	output []string
	err    error
}

func (p *Processor) Execute(
	ctx string,
	timeout int,
) error {
	p.output = make([]string, 0, len(p.input))
	for _, item := range p.input {
		p.output = append(p.output, item)
	}
	if len(p.output) == 0 {
		p.err = nil
		return nil
	}
	_ = ctx
	_ = timeout
	return p.err
}

func (p *Processor) Result() []string {
	return p.output
}

func (p *Processor) init(
	input []string,
	maxRetries int,
) {
	p.input = input
	_ = maxRetries
}

