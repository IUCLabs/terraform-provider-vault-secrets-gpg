package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestToInt64(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int64
		ok   bool
	}{
		{json.Number("3"), 3, true},
		{json.Number("3.0"), 3, true},
		{float64(5), 5, true},
		{int(7), 7, true},
		{int64(9), 9, true},
		{"nope", 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got, ok := toInt64(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("toInt64(%v) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestResolveCommand(t *testing.T) {
	// explicit command wins
	m := secretBackendModel{
		Plugin:  types.StringValue("vault-plugin-secrets-gpg"),
		Command: types.StringValue("custom-binary"),
	}
	if got := resolveCommand(m); got != "custom-binary" {
		t.Errorf("resolveCommand explicit = %q", got)
	}
	// falls back to plugin name when command is null
	m = secretBackendModel{
		Plugin:  types.StringValue("vault-plugin-secrets-gpg"),
		Command: types.StringNull(),
	}
	if got := resolveCommand(m); got != "vault-plugin-secrets-gpg" {
		t.Errorf("resolveCommand fallback = %q", got)
	}
}

func TestManagesPlugin(t *testing.T) {
	if managesPlugin(secretBackendModel{SHA256: types.StringNull()}) {
		t.Error("null sha256 should not manage the plugin")
	}
	if managesPlugin(secretBackendModel{SHA256: types.StringValue("")}) {
		t.Error("empty sha256 should not manage the plugin")
	}
	if !managesPlugin(secretBackendModel{SHA256: types.StringValue("abc123")}) {
		t.Error("set sha256 should manage the plugin")
	}
}

func TestKeyIDForLatest(t *testing.T) {
	data := map[string]interface{}{
		"latest_version": json.Number("2"),
		"keys": map[string]interface{}{
			"1": map[string]interface{}{"key_id": "AAAA1111"},
			"2": map[string]interface{}{"key_id": "BBBB2222"},
		},
	}
	if got := keyIDForLatest(data); got != "BBBB2222" {
		t.Errorf("keyIDForLatest = %q, want BBBB2222", got)
	}

	// missing data yields empty string, not a panic
	if got := keyIDForLatest(map[string]interface{}{}); got != "" {
		t.Errorf("keyIDForLatest(empty) = %q, want \"\"", got)
	}
}

func TestApplyKeyData(t *testing.T) {
	m := &keyModel{}
	applyKeyData(m, map[string]interface{}{
		"algorithm":              "ed25519",
		"identity":               "Me <me@x>",
		"exportable":             true,
		"deletion_allowed":       false,
		"min_decryption_version": json.Number("1"),
		"latest_version":         json.Number("1"),
		"fingerprint":            "DEADBEEF",
		"public_key":             "-----BEGIN PGP PUBLIC KEY BLOCK-----",
		"keys": map[string]interface{}{
			"1": map[string]interface{}{"key_id": "CAFED00D"},
		},
	})
	if m.Algorithm.ValueString() != "ed25519" {
		t.Errorf("algorithm = %q", m.Algorithm.ValueString())
	}
	if !m.Exportable.ValueBool() {
		t.Error("exportable should be true")
	}
	if m.Fingerprint.ValueString() != "DEADBEEF" {
		t.Errorf("fingerprint = %q", m.Fingerprint.ValueString())
	}
	if m.KeyID.ValueString() != "CAFED00D" {
		t.Errorf("key_id = %q", m.KeyID.ValueString())
	}
}
