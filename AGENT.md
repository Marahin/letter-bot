# letter-bot

This project is a Go Discord bot, which handles bookings and reservations for Tibia players. 

## Build & commands

Avoid using `go` directly, instead use `make` commands to ensure consistency across environments.

- Run tests: `make test`
- Run build: `make build`

## Code style

It's written in a flavour of hexagonal architecture, with the following layers:

* `cmd/` - contains the main application entry point,
* `internal/infra` - contains infrastructure code, such as Discord client, database implementations, etc.
* `internal/core` - contains core business logic. It is given access to infrastructure layer through interfaces (dependency injection).
* `internal/common` - shared code, including:
  * `dto` - our shared models,
  * `test/mocks` - implementations of interfaces for testing purposes, mocks for dependency injection,
* `internal/ports` - contains interfaces that define how the core interacts with the infrastructure. This allows for easy swapping of implementations, such as using a mock for testing.

Each functionality should be tested thoroughly with unit tests. We are using dependency injection to make testing easier. 

### Tests

Tests should be written using [testify](https://github.com/stretchr/testify). We are following the pattern of given-when-then: 

```go
// Example of a test using testify
func Test() {
	// given
	mySetup := setup()
	
	// when
	mySetup.DoAction()
	
	// then
	assert.Equal(t, expected, mySetup.Result)
}
```

**Create mocks by writing an interface, and running `mockery`**. Then, you can import the mocks from `internal/common/test/mocks`. Avoid creating your own mocks manually in tests.
