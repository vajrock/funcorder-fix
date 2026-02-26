package testpkg

// CommentService handles operations with comments.
type CommentService struct {
	data []byte
}

// Run starts the service.
//
//nolint:errcheck
func (cs *CommentService) Run() error {
	return nil
}

// Stop gracefully shuts down the service.
func (cs *CommentService) Stop() {
	cs.data = nil // inline trailing comment
}

// prepare initializes internal state.
// This is a multi-line doc comment
// that spans three lines.
func (cs *CommentService) prepare() {
	// inline comment inside body
	_ = cs.data
}

/* blockDocComment is documented with a block comment. */
func (cs *CommentService) blockDocComment() {
	/* block comment inside body */
}

