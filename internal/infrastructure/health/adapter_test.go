package health

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mocks "spot-assistant/internal/common/test/mocks"
)

func TestLive_ReflectsRuntimeStatus(t *testing.T) {
	// given
	rt := &mocks.MockRuntimeStatus{}
	adapter := NewAdapter(nil, rt)

	// when: not running
	rt.On("IsRunning").Return(false)
	err := adapter.Live()
	// then
	assert.Error(t, err)

	// when: running, gateway healthy
	rt.ExpectedCalls = nil // reset expectations for next call
	rt.On("IsRunning").Return(true)
	rt.On("GatewayHealthy").Return(nil)
	err = adapter.Live()
	// then
	assert.NoError(t, err)

	// when: running, gateway frozen
	rt.ExpectedCalls = nil
	rt.On("IsRunning").Return(true)
	rt.On("GatewayHealthy").Return(errors.New("gateway heartbeat ack is 123s stale"))
	err = adapter.Live()
	// then
	assert.Error(t, err)
}

func TestReady_ChecksDBAndRuntime(t *testing.T) {
	// given
	db := &mocks.MockDBPinger{}
	rt := &mocks.MockRuntimeStatus{}
	adapter := NewAdapter(db, rt)

	// when: db ok, runtime ok
	db.On("Ping", mock.Anything).Return(nil)
	rt.On("IsRunning").Return(true)
	rt.On("GatewayHealthy").Return(nil)
	err := adapter.Ready()
	// then
	assert.NoError(t, err)

	// when: db fail
	db.ExpectedCalls = nil
	rt.ExpectedCalls = nil
	db.On("Ping", mock.Anything).Return(errors.New("db down"))
	err = adapter.Ready()
	// then
	assert.Error(t, err)

	// when: db ok, runtime false
	db.ExpectedCalls = nil
	rt.ExpectedCalls = nil
	db.On("Ping", mock.Anything).Return(nil)
	rt.On("IsRunning").Return(false)
	err = adapter.Ready()
	// then
	assert.Error(t, err)

	// when: db ok, runtime running, gateway frozen
	db.ExpectedCalls = nil
	rt.ExpectedCalls = nil
	db.On("Ping", mock.Anything).Return(nil)
	rt.On("IsRunning").Return(true)
	rt.On("GatewayHealthy").Return(errors.New("gateway heartbeat ack is 123s stale"))
	err = adapter.Ready()
	// then
	assert.Error(t, err)
}
