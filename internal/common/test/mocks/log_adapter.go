package mocks

import "github.com/stretchr/testify/mock"

type MockLogAdapter struct {
	mock.Mock
}

func (a *MockLogAdapter) Error(inputArgs ...interface{}) {
	a.Called(inputArgs...)
}
