package marinemeteo

import (
	"strings"
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// The client's HTTP behaviour is covered in marinemeteo_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "marinemeteo" {
		t.Errorf("Scheme = %q, want marinemeteo", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "marinemeteo" {
		t.Errorf("Identity.Binary = %q, want marinemeteo", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"48.8,-4.5", "latlon", "48.8,-4.5"},
		{"-33.86,151.21", "latlon", "-33.86,151.21"},
		{"atlantic ocean", "query", "atlantic ocean"},
		{"48.8", "query", "48.8"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("latlon", "48.8,-4.5")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if !strings.Contains(got, "latitude=48.8") || !strings.Contains(got, "longitude=-4.5") {
		t.Errorf("Locate = %q, expected latitude=48.8 and longitude=-4.5", got)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown URI type")
	}
}
