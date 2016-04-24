package transport_test

import (
	"roolet/transport"
	"testing"
)

func TestSimpleCmdGet(t *testing.T) {
	// eyJzdGF0dXMiOiAyfQ== -> {"status": 2}
	cmd := transport.NewCommand(0, "some-id", "method_get", "eyJzdGF0dXMiOiAyfQ==")
	if jsonCmd, err := cmd.Dump(); err == nil {
		t.Logf("New command: %s", *jsonCmd)
	} else {
		t.Errorf("Serialization problem: %s", err)
	}
}

func TestSimpleCmdLoad(t *testing.T) {
	data := ("{\"jsonrpc\":\"2.0\",\"id\":0,\"method\":\"method_get\",\"params\":" +
		"{\"cid\":\"some-id\",\"data\":\"eyJzdGF0dXMiOiAyfQ==\",\"json\":\"{}\"}}")
	cmd := transport.Command{}
	dat := []byte(data)
	if cmd.Load(&dat) == nil && cmd.Params.Cid == "some-id" && cmd.Params.Json == "{}" {
		t.Logf("Load command: %s", cmd)
	} else {
		t.Error("Load json object error.")
	}
}
