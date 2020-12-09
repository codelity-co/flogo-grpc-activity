package grpc

import (
	"testing"

	// "context"
	"github.com/project-flogo/core/support/test"
	"github.com/project-flogo/core/activity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ActivityTestSuite struct {
	suite.Suite
	// testConfig string
}

func (suite *ActivityTestSuite) TestActivityRegister() {
	t := suite.T()

	ref := activity.GetRef(&Activity{})
	act := activity.Get(ref)

	assert.NotNil(t, act)
}

func (suite *ActivityTestSuite) TestActivityNew() {
	t := suite.T()

	s := &Settings{}
	ctx := test.NewActivityInitContext(s, nil)
	assert.NotNil(t, ctx)

	act, err := New(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, act)
}

func (suite *ActivityTestSuite) TestActivityMetadata() {
	t := suite.T()

	s := &Settings{}
	ctx := test.NewActivityInitContext(s, nil)
	assert.NotNil(t, ctx)

	act, err := New(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, act)

	m := act.Metadata()
	assert.NotNil(t, m)
}

func TestActivityTestSuite(t *testing.T) {
	suite.Run(t, new(ActivityTestSuite))
}