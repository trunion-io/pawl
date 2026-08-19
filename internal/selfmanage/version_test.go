package selfmanage

// PAWL-023 AC3 — a development build reports as unverifiable, not as failed.
//
// The regression this pins: `git describe --tags --always` returns a bare commit
// SHA when no tag is reachable, which the previous test ("", "dev", "-dirty")
// did not recognise as a development build. Every local build then asked GitHub
// for a release named after a commit and reported UNCHECKED.

import "testing"

func TestIsReleaseVersion(t *testing.T) {
	releases := []string{
		"0.1.0", "v0.1.0", "1.2.3", "10.20.30",
		"1.0.0-rc.1", "1.0.0+build.5", "0.0.0",
	}
	for _, v := range releases {
		if !isReleaseVersion(v) {
			t.Errorf("isReleaseVersion(%q) = false, want true", v)
		}
	}

	development := []string{
		"",
		"dev",
		"d8c2a18",       // the spelling that caused the bug: git describe --always
		"d8c2a184d585",  // longer abbreviation
		"0.1.0-dirty",   // tagged but modified
		"d8c2a18-dirty", // untagged and modified
		"v0.1.0-2-gd8c2a18-dirty",
		"1.2",     // not three components
		"1.2.3.4", // too many
		"1.2.x",   // non-numeric
		"main",
		"latest",
	}
	for _, v := range development {
		if isReleaseVersion(v) {
			t.Errorf("isReleaseVersion(%q) = true, want false", v)
		}
	}
}

// A development build must not reach the network. That is the user-visible half
// of AC3: the old behaviour was a 404 on every developer's machine on every
// build, which is the noise PAWL-025 warns trains people to ignore the output.
func TestVerifyDevelopmentBuildIsUnverifiable(t *testing.T) {
	for _, v := range []string{"", "dev", "d8c2a18", "0.1.0-dirty"} {
		got := Verify(v)
		if got.Status != Unverifiable {
			t.Errorf("Verify(%q).Status = %q, want %q (detail: %s)",
				v, got.Status, Unverifiable, got.Detail)
		}
	}
}
