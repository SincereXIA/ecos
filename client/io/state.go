package io

import "ecos/edge-node/object"

type EcosWriterState struct {
	Status         string             `json:"status"`
	Meta           object.ObjectMeta  `json:"meta"`
	Blocks         []object.BlockInfo `json:"blocks"`
	FinishedBlocks []string           `json:"finishedBlocks"`
}
