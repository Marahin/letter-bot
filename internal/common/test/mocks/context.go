package mocks

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/mock"
)

var ContextMock = mock.AnythingOfType(fmt.Sprintf("%T", context.Background()))
