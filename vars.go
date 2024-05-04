package dashdog

import "errors"

var (
	ErrUrlInvalid = errors.New("url is invalid")
	ErrNotFound   = errors.New("not found")
)
