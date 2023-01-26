package packageloader

import "errors"

var ErrDuplicateConfig = errors.New("config raw and object fields are both set")
