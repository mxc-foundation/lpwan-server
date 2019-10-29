package maccommand

import (
	"github.com/mxc-foundation/lpwan-server/internal/storage"
	"github.com/mxc-foundation/lpwan-server/internal/test"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestBase struct {
	suite.Suite
}

func (ts *TestBase) SetupSuite() {
	assert := require.New(ts.T())
	conf := test.GetConfig()
	assert.NoError(storage.Setup(conf))
}

func (ts *TestBase) SetupTest() {
	test.MustFlushRedis(storage.RedisPool())
}
