package testpkg

type Service struct{ id int }

func NewService(id int) *Service { return &Service{id: id} }
func (s *Service) Run()          {}
func (s *Service) Stop()         {}
func (s *Service) helper()       {}
func (s *Service) reset()        {}
