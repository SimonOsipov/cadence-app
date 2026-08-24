package dosing

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
)

// The enum huma validates the body against and the set the store can mint a key
// for are the same set, written twice — so this reconciles them.
//
// Apart, they fail in two silent ways: a type added to the store alone is
// refused at the door although the API can hold it, and one added to the tag
// alone reaches a handler that answers 422 from a branch nothing else reaches.
func TestTheAdvertisedTypesAreTheOnesTheStoreCanKeep(t *testing.T) {
	field, ok := reflect.TypeOf(PhotoUploadInput{}.Body).FieldByName("ContentType")
	if !ok {
		t.Fatal("PhotoUploadInput has no ContentType field for the enum to sit on")
	}

	advertised := strings.Split(field.Tag.Get("enum"), ",")
	kept := storage.ImageTypes()

	if len(advertised) != len(kept) {
		t.Fatalf("the tag advertises %v, the store keeps %v", advertised, kept)
	}
	for _, contentType := range kept {
		if !slices.Contains(advertised, contentType) {
			t.Errorf("the store keeps %s and the tag does not advertise it", contentType)
		}
	}
	for _, contentType := range advertised {
		if !slices.Contains(kept, contentType) {
			t.Errorf("the tag advertises %s and the store cannot keep it", contentType)
		}
	}
}
