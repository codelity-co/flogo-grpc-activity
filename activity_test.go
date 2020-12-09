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

func (suite *ActivityTestSuite) TestActivityEval() {
	t := suite.T()

	s := &Settings{}
	ctx := test.NewActivityInitContext(s, nil)
	assert.NotNil(t, ctx)

	act, err := New(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, act)

	m := act.Metadata()
	assert.NotNil(t, m)

	tc := test.NewActivityContext(m)
	assert.NotNil(t, tc)

	// TODO: add correct input data 
	input := &Input{}
	err = tc.SetInputObject(input)

	// TODO: update input test
	assert.Nil(t, input.Data, "Input data is not nil")
	assert.Nil(t, err)

	done, err := act.Eval(tc)
	assert.Nil(t, err)
	assert.Equal(t, true, done, "Eval method has not yet done")

	output := &Output{}
	err = tc.GetOutputObject(output)
	assert.Nil(t, err)
	assert.Equal(t, 0, output.Code, "Output code is not zero")
	
	// TODO: update output test
	assert.Nil(t, output.Data, "Output data is not nil")
}


func TestActivityTestSuite(t *testing.T) {
	suite.Run(t, new(ActivityTestSuite))
}