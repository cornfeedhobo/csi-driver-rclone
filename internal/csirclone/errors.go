package csirclone

import "errors"

var (
	ErrNotFound          = errors.New("metadata file not found")
	ErrRemoteNotFound    = errors.New("didn't find section in config file")
	ErrMetaWrongID       = errors.New("different id found in metadata file")
	ErrMetaWrongCapacity = errors.New("different capacity found in metadata file")
)
