package common

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMapper_MapIDtoPG(t *testing.T) {
	cases := []struct {
		ID string
	}{
		{"testID1"}, {"zhang"}, {"Bear"}, {"Qiu"},
		{"/path/to/objectA"}, {"604f8056bc9dcd748bd2169496d69c91afe36ad3ba9860ffaefcaa0bf9176c45"},
	}

	mapper := NewMapper(1000)

	for _, c := range cases {
		t.Run("MapID: "+c.ID, func(t *testing.T) {
			pgID := mapper.MapIDtoPG(c.ID)
			t.Logf("ID: %v -> PG: %v", c.ID, pgID)
			assert.NotZero(t, pgID)
		})
	}
}
