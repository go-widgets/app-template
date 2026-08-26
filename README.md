# app-template

[![CI](https://github.com/go-widgets/app-template/actions/workflows/ci.yml/badge.svg)](https://github.com/go-widgets/app-template/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-widgets/app-template.svg)](https://pkg.go.dev/github.com/go-widgets/app-template)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)

**This is the starting point for a [go-widgets](https://github.com/go-widgets)
app.** State goes through [go-widgets/mvvm](https://github.com/go-widgets/mvvm)
(bound to widgets via [go-widgets/mvvmtk](https://github.com/go-widgets/mvvmtk));
CI enforces it via [go-widgets/mvvmlint](https://github.com/go-widgets/mvvmlint).
**Copy it, rename the module, build your VM + view.**

It is a tiny but real browser-wasm canvas app: a `SearchEntry` (name filter), a
`DropDown` (category filter), a `ListBox` of the matching rows and a status
`Label` — where the filter state lives in observables and the visible list is
*derived* from them. Everything a bigger app needs (a testable ViewModel, a
bound View, a wasm host, a 100%-coverage gate and the MVVM gate) is already
wired, so you start MVVM-compliant and green by default.

## The architecture

```
┌── viewmodel.go ──────────────┐        ┌── scene.go ──────────────────┐
│  ViewModel (imports mvvm,     │  bind  │  View (imports toolkit +      │
│  never a widget)              │◀──────▶│  mvvmtk)                      │
│                               │        │                               │
│  Query    Observable[string]  │        │  SearchEntry ◀ BindText       │
│  Category Observable[int]     │        │  DropDown    ◀ BindSelectedIndex
│  Visible  ObservableList[item]│        │  ListBox     ◀ BindListItems  │
│  Status   Observable[string]  │        │  Label       ◀ BindLabel      │
│  Clear    Command             │        │  Button      ◀ BindCommand    │
└───────────────────────────────┘        └───────────────────────────────┘
```

- **`viewmodel.go` — the ViewModel.** Holds *all* state as `mvvm` primitives and
  references no widget. `recompute` is the only writer of the derived `Visible`
  list and `Status`; it runs whenever `Query` or `Category` changes. Trivially
  unit-testable with no canvas.
- **`scene.go` — the View.** Builds the widgets and binds each one to the VM
  through `mvvmtk` (`BindText`, `BindSelectedIndex`, `BindListItems`,
  `BindLabel`, `BindCommand`). It **never mutates a widget's state field
  directly** — the binders own that — so it passes `mvvmlint`.
- **`main.go` — the wasm host.** A `//go:build js && wasm` canvas driver: it
  forwards mouse / wheel / keyboard input as toolkit events and blits
  `state.draw` onto a `<canvas>`. This is the only file with a build tag, so it
  is excluded from native tests and coverage (like `go-widgets/gallery` and
  `go-pkgx/registry-viewer`). `native.go` is a tiny native stub so
  `go build ./...` links off-wasm.

The toolkit's native scrolling (wheel, keyboard, draggable scrollbar) works
through the `ListBox` with no app code — `main.go` just forwards the wheel delta.

## Use it

Click **“Use this template”** on GitHub (this repo is a template repository), or:

```sh
git clone https://github.com/go-widgets/app-template my-app
cd my-app
# rename the module and its internal import path if you split packages:
go mod edit -module github.com/you/my-app
```

Then edit `viewmodel.go` (your state + logic) and `scene.go` (your widgets +
bindings). Keep new widget-state changes flowing through `mvvmtk`/`mvvm` so the
gate stays green.

## Build & run

`build.sh` cross-compiles the wasm bundle into `dist/`:

```sh
./build.sh
```

It produces `dist/{app.wasm, wasm_exec.js, index.html}`. Serve `dist/` over HTTP
(a `file://` open will not instantiate the wasm):

```sh
./build.sh && (cd dist && python3 -m http.server 8080)
# open http://localhost:8080
```

`index.html` is the wasm host page: it shows a download progress bar, boots
`app.wasm` via `wasm_exec.js`, and installs a defensive `globalThis.fs` shim.
Its element ids avoid `fs` / `process` / `path` so they never shadow the Go-wasm
runtime globals.

## Test

Native tests exercise the ViewModel and the View (construct, bind, dispatch,
draw) with **100% statement coverage** — the wasm `main` is exempt by build tag:

```sh
go test -cover ./...        # 100.0% of statements
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build .   # wasm build smoke
```

## The MVVM gate

CI (`.github/workflows/ci.yml`) runs two jobs on every push / PR:

- **build** — `go vet`, native `go test ./...` at 100% coverage, and a
  `GOOS=js GOARCH=wasm` build smoke.
- **mvvm** — the shared gate via the reusable workflow
  `go-widgets/mvvmlint/.github/workflows/mvvmlint.yml@main`, which fails the
  build on any direct widget-state mutation. Mark it a **required** status check
  in branch protection.

Run the gate locally exactly as CI does:

```sh
go install github.com/go-widgets/mvvmlint/cmd/mvvmlint@latest
go vet -vettool="$(go env GOPATH)/bin/mvvmlint" ./...   # exits 0 on this template
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright (c) 2026 the
go-widgets/app-template authors.
