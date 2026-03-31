package redis

import "testing"

func TestParseClusterNodes(t *testing.T) {
	raw := "07c37dfeb2352e0b6b488f7b8435c3017f8c4b8f 127.0.0.1:7000@17000 myself,master - 0 0 1 connected 0-5460\n" +
		"1c1f5a3f3f74fe0d7210e5b6b6125f0f0f0f0f0f 127.0.0.1:7001@17001 slave 07c37dfeb2352e0b6b488f7b8435c3017f8c4b8f 0 0 2 connected\n"

	nodes := parseClusterNodes(raw)
	if len(nodes) != 2 {
		t.Fatalf("expected two nodes, got %d", len(nodes))
	}
	if nodes[0].NodeID != "07c37dfeb2352e0b6b488f7b8435c3017f8c4b8f" {
		t.Fatalf("unexpected node id %q", nodes[0].NodeID)
	}
	if nodes[0].Addr != "127.0.0.1:7000" {
		t.Fatalf("expected normalized addr, got %q", nodes[0].Addr)
	}
	if nodes[0].Role != "master" {
		t.Fatalf("expected master role, got %q", nodes[0].Role)
	}
	if nodes[1].MasterID != nodes[0].NodeID {
		t.Fatalf("expected replica master id to reference first node")
	}
}
