package model

import (
	"testing"
)

func TestDrawGraph(t *testing.T) {
	node := Node{}
	if err := node.Compose("ethereum", "0x28c79f7607cfbafcdfbc88606767333cb5aabdad"); err != nil {
		panic(err)
	}
	DrawGraph("ethereum", node)
}
