package alaya

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)
import "github.com/sincerexia/gocrush"

const (
	ROOT        = 0
	DATA_CENTER = 1
	RACK        = 2
	NODE        = 3
	DISK        = 4
)

func TestAlaya_RecordObjectMeta(t *testing.T) {
	tree := makeStrawTree()
	nodes := gocrush.Select(tree, 868, 3, NODE, nil)
	for _, n := range nodes {
		t.Logf("node: %v", n.GetId())
	}
	checkUnique(t, nodes)
	nodes = gocrush.Select(tree, 11, 3, NODE, nil)
	for _, n := range nodes {
		t.Logf("node: %v", n.GetId())
	}
	checkUnique(t, nodes)
}

func checkUnique(t *testing.T, nodes []gocrush.Node) {
	m := make(map[string]int)
	for _, n := range nodes {
		m[n.GetId()] = 1
	}
	assert.Equal(t, len(m), len(nodes))
}

func makeStrawTree() *gocrush.TestingNode {

	var parent = new(gocrush.TestingNode)
	parent.Id = "ROOT"
	parent.Type = ROOT
	parent.Weight = 0
	parent.Children = make([]gocrush.Node, 50)
	for dc := 0; dc < 50; dc++ {
		var node = new(gocrush.TestingNode)
		node.Parent = parent
		node.Weight = 10
		node.Type = NODE
		node.Id = parent.Id + ":NODE" + strconv.Itoa(dc)

		parent.Children[dc] = node
		node.Selector = gocrush.NewStrawSelector(node)
	}
	parent.Selector = gocrush.NewStrawSelector(parent)
	return parent
}
