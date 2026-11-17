package provider

import "testing"

func TestParseLabels(t *testing.T) {
	c := Container{ID: "a", Labels: map[string]string{"pocketid.enable": "true", "pocketid.client.id": "paperless", "pocketid.client.callback": "https://example.org/oidc/callback", "pocketid.groups": "family, admins"}}
	d, ok, err := parseLabels(c)
	if err != nil || !ok {
		t.Fatalf("parse failed %v", err)
	}
	if d.Client.ID != "paperless" || !d.Client.IsGroupRestricted || len(d.Groups) != 2 || d.Groups[0] != "admins" {
		t.Fatalf("unexpected %#v", d)
	}
}
func TestDeniesEmptyGroups(t *testing.T) {
	_, _, err := parseLabels(Container{Labels: map[string]string{"pocketid.enable": "true", "pocketid.client.id": "test", "pocketid.client.callback": "https://example.org/callback"}})
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestRejectsHTTPCallback(t *testing.T) {
	_, _, err := parseLabels(Container{Labels: map[string]string{"pocketid.enable": "true", "pocketid.client.id": "test", "pocketid.client.callback": "http://example.org/callback", "pocketid.groups": "admin"}})
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestForwardAuthIgnored(t *testing.T) {
	_, ok, err := parseLabels(Container{Labels: map[string]string{"pocketid.enable": "true", "pocketid.mode": "forwardauth"}})
	if err != nil || ok {
		t.Fatalf("forwardauth should not be enabled yet: %v", err)
	}
}
