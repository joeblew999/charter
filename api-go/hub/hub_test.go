package hub

import (
	"context"
	"reflect"
	"testing"
)

type device struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// A hub carries whatever type it is made for, and two hubs are two feeds.
func TestMemoryDeliversToSubscribersOfItsOwnFeed(t *testing.T) {
	var devices Memory[device]
	var words Memory[string]
	var gotA, gotB []device
	var gotWords []string
	stopA, _ := devices.Subscribe(func(d device) { gotA = append(gotA, d) }, nil)
	devices.Subscribe(func(d device) { gotB = append(gotB, d) }, nil)
	words.Subscribe(func(s string) { gotWords = append(gotWords, s) }, nil)

	devices.Publish(context.Background(), device{1, "one"})
	stopA()
	devices.Publish(context.Background(), device{2, "two"})
	words.Publish(context.Background(), "hello")

	if want := []device{{1, "one"}}; !reflect.DeepEqual(gotA, want) {
		t.Errorf("the subscriber that left got %v, want %v", gotA, want)
	}
	if want := []device{{1, "one"}, {2, "two"}}; !reflect.DeepEqual(gotB, want) {
		t.Errorf("the subscriber that stayed got %v, want %v", gotB, want)
	}
	if !reflect.DeepEqual(gotWords, []string{"hello"}) {
		t.Errorf("the other feed got %v", gotWords)
	}
}

var _ Hub[device] = (*Memory[device])(nil)
