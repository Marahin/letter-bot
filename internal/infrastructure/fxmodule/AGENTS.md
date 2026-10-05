# fxmodule

`cmd/<bin>/main.go` is `fx.New(<bin>app.App()).Run()` and nothing else. All wiring
is in `internal/infrastructure/fxmodule`: the shared modules at the root (`Logger`,
`Config`, `Database`, `Repositories`, `WorldAPI`, `Registry`, `Metrics`), and one
subpackage for each binary (`webapp`, `botapp`) that exports only `App()`.

## Layout of a binary subpackage

- `app.go`: `App()` supplies `fxmodule.Binary` and adds the modules that need the
  environment (config loaders, the database) to `wiring()`. `wiring()` holds every
  provider and invoke that needs no environment. The tests build it over a fake
  environment.
- `deps.go`: the providers of the adapters and the core services.
- `server.go` or `run.go`: the lifecycle hooks.
- When more than one binary needs a module, put it at the `fxmodule` root, not in a
  subpackage.

## Rules

- Only `botapp` imports `internal/infrastructure/bot`. The root modules never load
  the bot config. A test in `webapp` fails when `cmd/web` depends on the bot adapter.
- A provider makes one thing and returns it. Logic that is more than construction
  goes to a package under `internal/`. Exceptions: the migrations, the pool connect
  and the shard manager, which ask the network at construction. Providers run in
  `fx.New`, so `StartTimeout` does not bound them.
- A side effect (serve, open the gateway, run a loop) is an `fx.Invoke` that appends
  `fx.Lifecycle` hooks. A long-running goroutine takes its own context, because a
  hook's context ends with the hook.
- Bind the listen address in `OnStart`, so a busy port fails the start. A server
  that fails later calls `fx.Shutdowner.Shutdown(fx.ExitCode(1))`.
- fx stops the hooks in the reverse order of the invokes. `fxmodule.Metrics` comes
  before the serve or run invoke, so the metrics server stops after it. The pool
  closes next, and the logger's Sync runs last.
- A stop past `StopTimeout` skips the remaining hooks and exits 1.
- Config loaders return `(T, error)` through `fxmodule.Load`. fx fails the start on
  the error.
- `fxmodule.Repositories` provides every repository as its concrete type. Providers
  take the concrete type; the core constructor's port type accepts it at the call.
- Each binary has its own `*prometheus.Registry` (`fxmodule.Registry`). Nothing
  registers on the global default registry.
- `fxmodule.Database` migrates, then connects: `connect` takes `Migrated`, so every
  consumer of the pool runs after the migrations. Do not build a pool elsewhere.
- fx makes a provider only when something asks for it. An unused loader or
  repository costs nothing.

## Tests

- `app_test.go` in each subpackage: `fx.ValidateApp(App())` must return nil.
- `fx.New(testEnvironment(...), wiring())` must build with `Err() == nil`. The
  environment has a pgxpool to `127.0.0.1:1` (it connects only when used) and, for
  the bot, a `*shards.Manager` that never dials Discord (`fx.Replace`).
- Unit-test every provider that has a branch, and the hooks (a fake `fx.Lifecycle`
  that records the appended hooks).
- `cmd/` has no test files.
