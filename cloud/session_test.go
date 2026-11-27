package cloud

import "testing"

const (
	older = "0000007b-0000b26e-00001538-0c3e87bb37b3db5cc52eedb93cd3b96b"
	newer = "00000085-0000024c-00004c6d-8e61117bf38d7adb71b934ebbf891683"
)

// What a request brought is read back as it was written, and what is no bookmark of D1's starts nothing: a value
// that is malformed, someone's invention, or one of D1's own words for a start.
func TestParseBookmarks(t *testing.T) {
	both := Bookmarks{"DB": older, "LOG": newer}
	if got := ParseBookmarks(both.String()); len(got) != 2 || got["DB"] != older || got["LOG"] != newer {
		t.Errorf("%q read back as %v", both.String(), got)
	}
	if got, want := both.String(), "DB="+older+"&LOG="+newer; got != want {
		t.Errorf("written as %q, want %q", got, want)
	}
	for _, bad := range []string{"", "DB", "DB=first-primary", "DB=first-unconstrained", "DB=" + older + "x", "DB=../../etc",
		"DB=" + older + "&DB=" + newer, "=" + older, "%zz", "DB=0000007B-0000B26E-00001538-0C3E87BB37B3DB5CC52EEDB93CD3B96B"} {
		if got := ParseBookmarks(bad); len(got) != 0 {
			t.Errorf("%q was taken for bookmarks: %v", bad, got)
		}
	}
	// One that is none does not take the others with it.
	if got := ParseBookmarks("DB=" + older + "&LOG=nonsense"); len(got) != 1 || got["DB"] != older {
		t.Errorf("a good bookmark beside a bad one: %v", got)
	}
}

// A reader is never set back: of what their request brought and what the response has, the later of each.
func TestJoin(t *testing.T) {
	brought, has := Bookmarks{"DB": newer, "LOG": older}, Bookmarks{"DB": older, "LOG": newer, "NEW": older}
	got := has.Join(brought)
	if got["DB"] != newer || got["LOG"] != newer || got["NEW"] != older || len(got) != 3 {
		t.Errorf("joined: %v", got)
	}
	if Newer(older, "") != older || Newer("", "") != "" || Newer(newer, older) != newer {
		t.Error("Newer is not the later of two")
	}
}

// Where a session starts: the primary outside a request, whatever else is said, and in one that reads the newest,
// whatever it brought; then its bookmark; then anywhere.
func TestStartOf(t *testing.T) {
	for _, c := range []struct {
		inRequest, newest bool
		brought           string
		want              string
	}{
		{true, true, older, fromPrimary}, {true, true, "", fromPrimary}, {true, false, older, older}, {true, false, "", fromAnywhere},
		// A Cron Trigger, a queue's batch, an email, work after the answer.
		{false, false, "", fromPrimary}, {false, false, older, fromPrimary}, {false, true, "", fromPrimary},
	} {
		if got := startOf(c.inRequest, c.newest, c.brought); got != c.want {
			t.Errorf("startOf(%v, %v, %q) = %q, want %q", c.inRequest, c.newest, c.brought, got, c.want)
		}
	}
}
