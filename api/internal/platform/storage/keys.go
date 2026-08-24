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

// The image types a photograph may be stored and served as, and the extension
// each is written under.
//
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
// The server mints it and the client never chooses it: the prefix is the
// patient's id, and both tables holding a key constrain it by a CHECK naming
// that id. A client-supplied key would make that CHECK the only thing between a
// patient and another patient's prefix; minting here means it is never reached.
//
// The prefix must be a single identifier, and that is checked here rather than
// assumed. On the upload path this is the only gate: a signed PUT is handed out
// without opening a transaction, so the shape check inside database.WithCaller
// never runs, and a subject carrying a separator — «other-patient/x/..» — would
// be signed into somebody else's prefix once a client normalised the path.
// Today no such subject exists, because the identity provider issues a uuid;
// this makes that a property of ours rather than of a component we do not own.
//
// database.IsUUIDShaped and not a parser of this package's own, because that
// function's own doc says why: two definitions of «UUID-shaped» in one process
// is one too many, and this is a context writing an identifier into those
// tables. uuid.Parse is the looser one — it takes urn:uuid:, braces and the
// undashed form, each of which mints a key no patient's CHECK can match.
//
// Lower-cased, because that predicate compares case-insensitively while the
// CHECK compares the key against patient_id::text, which is canonical. Since the
// predicate passed, lowering yields exactly that canonical rendering.
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
