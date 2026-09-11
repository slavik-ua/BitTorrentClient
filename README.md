# torrentClient

BitTorrent client in Go. Stdlib only - bencode, HTTP/UDP trackers, and the peer
wire protocol are hand-rolled. Parallel worker-pool downloader with per-piece
SHA-1 verification, resume, and 1 Hz progress logging.

## Quickstart

```bash
# 1. local swarm: opentracker on 6969 (TCP+UDP) + 3 Transmission seeders
make swarm-up

# 2. seed data (20M+40M urandom + readme into seed/data)
./generate_seed.sh

# 3. download (the .torrent must announce the local tracker, e.g.
#    http://localhost:6969/announce from the host)
go run ./cmd -torrent file.torrent -out ./files
```

Re-running the same command resumes: verified pieces are kept, and a complete
download exits without contacting any peer.

## CLI

```bash
go run ./cmd -h
```

| Flag        | Default        | Meaning                          |
|-------------|----------------|----------------------------------|
| `-torrent`  | `file.torrent` | path to .torrent file            |
| `-out`      | `files`        | output directory                 |
| `-port`     | `6881`         | listen port advertised to tracker|
| `-maxPeers` | `20`           | max parallel peer connections    |

Compare concurrency with the same binary: `-maxPeers 1` (sequential control)
vs `-maxPeers 8`.

## Layout

```bash
cmd/                  thin driver: flags, signal handling, progress ticker
internal/parser/      hand-rolled bencode, torrent parsing, MapRange
internal/tracker/     Tracker interface, HTTP + UDP announce
internal/peer/        wire protocol (Conn interface + Peer impl)
internal/store/       preallocated disk I/O, per-piece read/write
internal/downloader/  worker pool, resume, progress snapshot
```

## Develop

```bash
make build       # bin/torrent
make fmt vet     # must stay green
make test        # fast focused suite with -race
make test-race   # full gate
```

`peer.Conn` + `store.Store` are the test seams — the pool is unit-tested with
scripted fake peers (split-brain, corrupt-retry, cancel), no network needed.

## Roadmap

Worker pool is in; next: rarest-first piece selection, request pipelining.
Non-goals: DHT/magnets, seeding/upload, uTP/MSE, IPv6, endgame.

## License

MIT — see [LICENSE](LICENSE).
