package journal

import "slices"

// The two closed sets, each declared once and read by both the parser and the test that
// reconciles it against the schema. A parser written from its own literal list is a second
// copy, and the two drift the first time a value is added.

func Tags() []Tag {
	return []Tag{
		TagAppetite, TagBloating, TagFatigue, TagHeadache, TagInsomnia, TagNausea, TagSite,
	}
}

func Sources() []Source { return []Source{SourceDose, SourceManual} }

// ParseTag and ParseSource are the seam where a string becomes a member of a set: the
// transport of step 8, and the seed. `(T, bool)` and not an error — the caller knows which
// field it was reading, and the field's name is the whole of the message.
func ParseTag(s string) (Tag, bool) { return parse(s, Tags()) }

func ParseSource(s string) (Source, bool) { return parse(s, Sources()) }

func parse[T ~string](s string, set []T) (T, bool) {
	if i := slices.Index(set, T(s)); i >= 0 {
		return set[i], true
	}

	var none T

	return none, false
}
