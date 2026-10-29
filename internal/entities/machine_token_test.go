package entities

import "testing"

func TestParseMachineToken(t *testing.T) {
	token, ok := ParseMachineToken("paas_pat_0192-abc.s3cret")
	if !ok || token.ClientID != "paas_pat_0192-abc" || token.Secret != "s3cret" {
		t.Fatalf("разбор нового формата: %+v, %v", token, ok)
	}
	for _, legacy := range []string{"paas_pat_0123abcd", ".x", "x.", ""} {
		if _, ok := ParseMachineToken(legacy); ok {
			t.Fatalf("%q не должен считаться токеном нового формата", legacy)
		}
	}
}
