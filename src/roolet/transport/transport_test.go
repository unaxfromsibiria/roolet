package transport_test

import (
	"roolet/transport"
	"testing"
)


func TestSimpleCmdGet(t *testing.T) {
	// eyJzdGF0dXMiOiAyfQ== -> {"status": 2}
	cmd := transport.NewCommand("some-id", "method_get", "eyJzdGF0dXMiOiAyfQ==")
	if jsonCmd, err := cmd.Dump(); err == nil {
		t.Logf("New command: %s", *jsonCmd)
	} else {
		t.Errorf("Serialization problem: %s", err)
	}
}
