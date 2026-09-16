package provider

import "testing"

func TestJSONSubsetEqual(t *testing.T) {
	cfg := []byte(`[{"type":"agent_toolset_20260401","configs":[{"name":"bash","permission_policy":{"type":"always_allow"}}]}]`)
	api := []byte(`[{"type":"agent_toolset_20260401","default_config":{"enabled":true,"permission_policy":{"type":"always_ask"}},"configs":[{"type":"bash","name":"bash","enabled":true,"permission_policy":{"type":"always_allow"}}]}]`)
	if ok, err := jsonSubsetEqual(cfg, api); err != nil || !ok {
		t.Fatalf("server defaults must be ignored: %v %v", ok, err)
	}
	drift := []byte(`[{"type":"agent_toolset_20260401","default_config":{"enabled":true},"configs":[{"type":"bash","name":"bash","enabled":true,"permission_policy":{"type":"auto"}}]}]`)
	if ok, _ := jsonSubsetEqual(cfg, drift); ok {
		t.Fatal("changed nested value must be detected")
	}
	if ok, _ := jsonSubsetEqual(cfg, []byte(`[]`)); ok {
		t.Fatal("array length mismatch must be detected")
	}
	if s, _ := canonicalJSON([]byte(`{ "b": 1, "a": [1, 2] }`)); s != `{"a":[1,2],"b":1}` {
		t.Fatalf("canonicalJSON: %s", s)
	}
}
