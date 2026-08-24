package storage

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// The types an upload may declare, and the extension each is minted under. Not a
// closed set on the way out: ContentTypeFor has to answer for keys this map never
// minted.
var extensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/heic": "heic",
}

// ImageTypes answers the types an object may be stored and served as, sorted.
//
// Exported so a transport's enum can be reconciled against it rather than
// repeating it: the two apart means a type this API can store is refused at the
// door, or one it cannot store is accepted and then refused inside.
func ImageTypes() []string {
	types := make([]string, 0, len(extensions))
	for contentType := range extensions {
		types = append(types, contentType)
	}
	slices.Sort(types)

	return types
}

// ErrUnknownContentType is what NewKey answers for a type outside the set above.
var ErrUnknownContentType = errors.New("storage: not an image type this API stores")

// ErrPrefixNotAnIdentifier is what NewKey answers for a prefix that is not a
// single uuid — including one carrying a path separator.
var ErrPrefixNotAnIdentifier = errors.New("storage: a key's prefix must be one identifier")

// NewKey mints the key one object is stored under, below the given prefix.
//
// A client-supplied key would leave the tables' patient_id CHECK as the only thing
// between a patient and another's prefix; minting here means it is never reached.
//
// The prefix check is the named weakness of the upload path: a signed PUT opens no
// transaction, so database.WithCaller's own shape check never runs, and a subject
// carrying a separator would be signed into somebody else's prefix once a client
// normalised it. IsUUIDShaped and not a local parser — see its doc. Lower-cased
// because it compares case-insensitively and the CHECK compares against canonical.
func NewKey(prefix, contentType string) (string, error) {
	if !database.IsUUIDShaped(prefix) {
		return "", fmt.Errorf("%w: %q", ErrPrefixNotAnIdentifier, prefix)
	}

	extension, ok := extensions[contentType]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnknownContentType, contentType)
	}

	return strings.ToLower(prefix) + "/" + uuid.NewString() + "." + extension, nil
}

// ContentTypeFor answers what an object stored under this key is served as.
//
// Read off the key, because a second column recording the type could disagree
// with it. An extension outside the set answers a type no browser renders: a key
// from a seed still has to be readable, and unrenderable is the safe end.
func ContentTypeFor(key string) string {
	extension := strings.TrimPrefix(path.Ext(key), ".")
	for contentType, known := range extensions {
		if strings.EqualFold(extension, known) {
			return contentType
		}
	}

	return "application/octet-stream"
}
