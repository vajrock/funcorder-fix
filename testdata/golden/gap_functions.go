package testpkg

type Server struct {
	port int
}

func NewServer(port int) *Server {
	return &Server{port: port}
}

func (s *Server) Listen() {
	_ = s.port
}

func defaultPort() int {
	return 8080
}

func (s *Server) Handle(path string) {
	_ = path
}

func formatAddr(host string, port int) string {
	_ = host
	_ = port
	return ""
}

func (s *Server) shutdown() {
	s.port = 0
}

