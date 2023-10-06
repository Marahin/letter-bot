package summary

import (
	"github.com/stretchr/testify/mock"
)

type MockChartAdapter struct {
	mock.Mock
}

func (a *MockChartAdapter) NewChart(values []float64, legend []string) ([]byte, error) {
	args := a.Called(values, legend)
	return args.Get(0).([]byte), args.Error(1)
}
