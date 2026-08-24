package storage

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

// The image types a photograph may be stored and served as, and the extension
// each is written under.
//
// A closed set in one direction and the other: the extension is what the key
// carries, and the key is all the read side has to decide what to serve the
// object as. A type absent here cannot be uploaded, so no key can name an
// extension this cannot map back.
var extensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/heic": "heic",
}

// ErrUnknownContentType is what NewKey answers for a type outside the set above.
var ErrUnknownContentType = errors.New("storage: not an image type this API stores")

// NewKey mints the key one object is stored under, below the given prefix.
//
// The server mints it and the client never chooses it: the prefix is the
// patient's id, and both tables holding a key constrain it by a CHECK naming
// that id. A client-supplied key would make that CHECK the only thing between a
// patient and another patient's prefix; minting here means it is never reached.
func NewKey(prefix, contentType string) (string, error) {
	if prefix == "" {
		return "", errors.New("storage: a prefix is required")
	}

	extension, ok := extensions[contentType]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnknownContentType, contentType)
	}

	return prefix + "/" + uuid.NewString() + "." + extension, nil
}

// ContentTypeFor answers what an object stored under this key is served as.
//
// Derived from the key rather than remembered beside it: the key is what the
// row holds, so a second column recording the type could disagree with it. An
// extension outside the set is not an error — a key predating this set, or one
// written by a seed, still has to be readable — and answers a type no browser
// renders, which is the safe end of the range.
func ContentTypeFor(key string) string {
	extension := strings.TrimPrefix(path.Ext(key), ".")
	for contentType, known := range extensions {
		if strings.EqualFold(extension, known) {
			return contentType
		}
	}

	return "application/octet-stream"
}
