package drand

import (
	"encoding/json"
	"fmt"
	"testing"

	dclient "github.com/drand/drand/client"
	"github.com/drand/drand/key"
	"github.com/stretchr/testify/assert"
)

func TestPrintGroupInfo(t *testing.T) {
	c, err := dclient.NewHTTPClient(drandServers[0], nil, nil)
	assert.NoError(t, err)
	cg := c.(interface {
		FetchGroupInfo(groupHash []byte) (*key.Group, error)
	})
	group, err := cg.FetchGroupInfo(nil)
	assert.NoError(t, err)
	gbytes, err := json.Marshal(group.ToProto())
	assert.NoError(t, err)
	fmt.Printf("%s\n", gbytes)
}
