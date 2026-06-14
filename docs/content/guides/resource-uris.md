---
title: "Resource URIs"
description: "Use dadjoke as a database/sql-style driver so a host program can address dadjoke as dadjoke:// URIs."
weight: 20
---

`dadjoke` is a command line, but the `dadjoke` Go package is also a
small driver that makes dadjoke addressable as a resource URI. A host
program registers it the way a program registers a database driver with
`database/sql`, then dereferences `dadjoke://` URIs without knowing
anything about how dadjoke is fetched.

The host that does this today is [ant](https://github.com/tamnd/ant), a single
binary that puts one URI namespace over a family of site tools. The examples
below use `ant`; any program that links the package gets the same behaviour.

## Mounting the driver

A host enables the driver with one blank import, exactly like `import _
"github.com/lib/pq"`:

```go
import _ "github.com/tamnd/dadjoke-cli/dadjoke"
```

The package's `init` registers a domain with the scheme `dadjoke` for the
host `dadjoke.com`. The standalone `dadjoke` binary does not change.

## Addressing records

A URI is `scheme://authority/id`. The scaffold ships one type:

| URI                              | What it is                              |
| -------------------------------- | --------------------------------------- |
| `dadjoke://page/<path>`    | a page, keyed by its path on dadjoke.com |

```bash
ant get dadjoke://page/<path>    # the page record
ant cat dadjoke://page/<path>    # just the body text
ant url dadjoke://page/<path>    # the live https URL
ant resolve https://dadjoke.com/<path> # a pasted link, back to its URI
```

As you add resolver operations in `dadjoke/domain.go`, each new `URIType`
becomes another addressable authority here, with no extra wiring. See
[add a command](/guides/adding-a-command/).

## Walking the graph

`ls` lists the members of a collection, and every member is itself an
addressable URI, so a host can follow the graph and write it to disk:

```bash
ant ls     dadjoke://page/<path>             # the pages this one links to
ant export dadjoke://page/<path> --follow 1 --to ./data
```

The example `links` op emits page stubs, so each listed member is a
`dadjoke://page/` URI in its own right. When you model edges between your
real records with `kit:"link"` tags, `ant export --follow` and `ant graph` walk
those edges too, across tools when a link points at another site's scheme.

## Why this is the same code

The driver and the binary share one definition per operation. A resolver op
answers both `dadjoke page` on the command line and `ant get
dadjoke://page/...` through a host, from the same handler and the same
client. There is no second implementation to keep in step.
