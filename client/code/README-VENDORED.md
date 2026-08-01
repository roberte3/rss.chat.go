# Vendored web client

The files in this directory are **not** part of the Go server. They are the
web client from Dave Winer's rss.chat, vendored here so that `git clone` plus
`go run . -setup` gives you a working site with no extra steps.

- **Upstream:** https://github.com/scripting/rss.chat
- **Commit:** `124487cb6008296396a1d8d176dc951466cd7d96`
- **License:** MIT, Copyright (c) 2026 Dave Winer — see `LICENSE` in this
  directory. That license covers everything here; the Go server's own license
  does not.

## Local changes

None. These files are byte-for-byte upstream, which is deliberate: it keeps
re-syncing a copy rather than a merge. Please keep it that way — if the server
needs the client to do something different, prefer adding a macro on the Go
side (see below) or sending the change upstream.

## Excluded from the vendored copy

Two upstream files are not runtime assets and are omitted:

- `source.opml` (312 KB) — the outline `index.html` is generated from
- `worknotes.md` — upstream development notes

## How the server couples to these files

`client/client.go` serves this directory and substitutes `[%macro%]`
placeholders in `index.html` only. The server must supply every macro
`index.html` references, currently:

    [%productName%]  [%productNameForDisplay%]  [%version%]
    [%flEnableLogin%]  [%urlServerForClient%]
    [%urlWebsocketServerForClient%]  [%flWebsocketEnabled%]
    [%feedUrlEveryone%]  [%urlMenuOpml%]

`TestVendoredClientMacrosAreSatisfied` in `client/client_test.go` fails if
`index.html` ever references a macro the server does not define, which is the
thing most likely to break on a re-sync.

## Re-syncing

    git clone https://github.com/scripting/rss.chat /tmp/rss.chat
    git -C /tmp/rss.chat archive main client/code | tar -x -C .
    rm -f client/code/source.opml client/code/worknotes.md
    go test ./client/

Then update the commit hash recorded above.
