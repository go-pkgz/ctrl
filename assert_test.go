package ctrl

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AssertTestSuite struct {
	suite.Suite
}

func TestAssertSuite(t *testing.T) {
	suite.Run(t, new(AssertTestSuite))
}

func (s *AssertTestSuite) TestAssert() {
	s.NotPanics(func() {
		Assert(true)
	})

	s.PanicsWithValue("assertion failed", func() {
		Assert(false)
	})
}

func (s *AssertTestSuite) TestAssertf() {
	s.NotPanics(func() {
		Assertf(true, "this should not panic")
	})

	msg := "test message"
	s.PanicsWithValue("assertion failed: "+msg, func() {
		Assertf(false, msg)
	})

	s.PanicsWithValue("assertion failed: value is 42", func() {
		Assertf(false, "value is %d", 42)
	})
}

func (s *AssertTestSuite) TestAssertFunc() {
	s.NotPanics(func() {
		AssertFunc(func() bool { return true })
	})

	s.PanicsWithValue("assertion failed", func() {
		AssertFunc(func() bool { return false })
	})

	counter := 0
	s.NotPanics(func() {
		AssertFunc(func() bool {
			counter++
			return counter > 0
		})
	})
}

func (s *AssertTestSuite) TestAssertFuncf() {
	s.NotPanics(func() {
		AssertFuncf(func() bool { return true }, "this should not panic")
	})

	msg := "custom func message"
	s.PanicsWithValue("assertion failed: "+msg, func() {
		AssertFuncf(func() bool { return false }, msg)
	})

	s.PanicsWithValue("assertion failed: value is 42", func() {
		AssertFuncf(func() bool { return false }, "value is %d", 42)
	})
}

// Additional test for complex formatting scenarios
func (s *AssertTestSuite) TestComplexFormatting() {
	type testStruct struct {
		Name  string
		Value int
	}

	test := testStruct{Name: "test", Value: 42}
	s.PanicsWithValue("assertion failed: struct value - Name: test, Value: 42", func() {
		Assertf(false, "struct value - Name: %s, Value: %d", test.Name, test.Value)
	})

	s.PanicsWithValue("assertion failed: multiple values: 1, 2, 3", func() {
		Assertf(false, "multiple values: %d, %d, %d", 1, 2, 3)
	})
}

// Test boundary cases
func (s *AssertTestSuite) TestBoundaryCases() {
	s.PanicsWithValue("assertion failed: ", func() {
		Assertf(false, "")
	})

	s.PanicsWithValue("assertion failed: test", func() {
		Assertf(false, "test")
	})
}
