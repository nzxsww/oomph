# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Lint

```sh
make lint        # run golangci-lint (govet, staticcheck, ineffassign)
make lint-fix    # auto-fix lint issues
go build ./...   # compile everything
```

The CI pipeline runs `make lint` on push/PR to `stable`. The project uses Go 1.26.0.

## Project Overview

Oomph is a **Minecraft: Bedrock Edition anti-cheat proxy** that sits between clients and a game server. It intercepts every packet, runs detections, and enforces server-authoritative movement and combat — meaning Oomph re-simulates physics/hit validation on the proxy and only forwards actions that pass checks.

There are three deployment models:

1. **Example/default** (standalone proxy) — uses gophertunnel directly. Simple, useful for development.
2. **Example/spectrum** (Spectrum-based proxy) — integrates with the [Spectrum](https://github.com/oomph-ac/spectrum) proxy ecosystem. Recommended for production.
3. **Example/portal** (portal-based proxy) — uses the `lib/portal` library for a more modern deployment pattern with remote event/management infrastructure.

All three deploy the same `player.Player` anti-cheat core.

## Architecture

### Packet flow

```
Client → gophertunnel Conn → player.Player.HandleClientPacket() → (detections check, components update) → ServerConn.WritePacket()
Server → gophertunnel Conn → player.Player.HandleServerPacket() → (world updates, entity tracking) → Client Conn.WritePacket()
```

Packets are processed under `procMu` — a mutex that serializes all packet handling per player. The player also runs a tick loop (`StartTicking()`) at 20Hz for server ticks.

### Core packages

| Package | Role |
|---|---|
| `player/` | Anti-cheat core: `Player` struct manages connections, tick loop, detections, and all component interfaces. |
| `player/component/` | Component implementations: movement simulation, combat validation, entity tracking, ACKs, inventory, world updates, clicks, effects, gamemode. Each is an interface in `player/*.go` with its implementation in `player/component/`. |
| `player/component/acknowledgement/` | Per-component ACK tracking — ensures client and server state stays in sync after Oomph modifications. |
| `player/detection/` | Cheat detection implementations (ReachA/B, KillauraA, AimA, AutoclickerA, BadPacketA-G, EditionFakerA-C, HitboxA, InvMoveA, ScaffoldA, NukerA). Each detection registers hooks into the relevant component (e.g., combat hooks for reach/aim). |
| `player/simulation/` | Movement simulation engine — re-simulates player physics server-side. |
| `player/command/` | `/ac` command system — sub-commands (alerts, debug, logs) register via `command.RegisterSubCommand()`. |
| `player/context/` | Packet handling context with cancel support. |
| `player/event/` | Remote event definitions (`Flagged`, `MitigationAlert`) sent to the backend server via `ScriptMessage` packets. |
| `entity/` | Entity state tracking with interpolation and tick-based position history for rewind (`entity.Rewind()` — looks up an entity's position at a given tick for lag-compensated combat checks). |
| `world/` | Server-side world model: chunk storage, block state lookups, collision box queries. Uses an xxh3-based subchunk caching system. |
| `world/block/` | Custom block model overrides (fences, walls, buttons, candles, etc.) that register into Dragonfly's block registry. |
| `world/blockmodel/` | Collision model helpers for multi-shape blocks. |
| `game/` | Vanilla-accurate Minecraft constants and math — sin/cos tables, movement physics constants (gravity, friction, jump height, step height), rotation-to-point calculations, raytracing. |
| `utils/` | Chunk encoding helpers, circular queue, evicting list, encryption, bit utilities, face/normal helpers, item interaction helpers. |
| `utils/collisions/` | Block collision shape registry loaded from embedded JSON data files (blockCollisionShapes.json, blockStates.json). Maps block name + state hash → bounding boxes. |
| `transport/` | (WIP) Alternative TCP transport for server connections. |
| `protocol/` | Custom protocol implementations (`proto924`, `proto975`) for specific MC version support. |
| `oerror/` | Custom error type for structured error handling. |
| `internal/` | Internal utilities (pool). |
| `assert/` | Test assertions. |
| `lib/portal/` | Portal library — multi-server proxy infrastructure with chunk caching, load balancing, whitelist, socket-based remote auth. |

### Server-Authoritative Movement

Oomph runs its own movement simulation inside `player/component/movement.go`. On every `PlayerAuthInput` from the client:

1. The server re-simulates the client's movement tick.
2. If Oomph's predicted position deviates from the client's report beyond `CorrectionThreshold`, it sends a position correction (rubberband) to the client.
3. Below the threshold, it can optionally "persuade" the server position toward the client or accept the client position/velocity (configurable in `oconfig` via `AcceptClientPosition`, `AcceptClientVelocity`, `PersuasionThreshold`).

This also mitigates timer cheats (any tick rate above 20 TPS is detected).

### Server-Authoritative Combat

Oomph validates all combat interactions:

1. Hooks into `InventoryTransaction` (attack packets) and `PlayerAuthInput` (missed swings).
2. Rewinds entities to their position at the client's tick using `entity.Rewind()` — a `CircularQueue` of `HistoricalPosition` entries per entity.
3. Performs multiple raycasts simulating Bedrock's frame-based combat.
4. Validates reach, angle, block occlusion, and bounding box intersection.
5. If valid, forwards the attack to the server; if invalid, drops it.

Two parallel combat systems run:
- **Server-state combat** (`CombatComponent`, backed by `entTracker`) — authoritative, uses server-tracked entity positions.
- **Client-state combat** (`ClientCombatComponent`, backed by `clientEntTracker`) — uses client-ACK'd entity positions for flagging-only detections (like reach).

### Detection System

Detections implement the `player.Detection` interface:

```go
type Detection interface {
    Type() string           // e.g. "Reach", "Killaura"
    SubType() string        // e.g. "A", "B"
    Description() string
    Punishable() bool
    Metadata() *DetectionMetadata  // Violations, Buffer, MaxViolations, etc.
    Detect(pk packet.Packet)
}
```

Most detections hook into combat/movement components rather than polling packets directly. They register via `player/component/register.go` hooks.

Detection flow: `player.PassDetection()` (decrements buffer) / `player.FailDetection()` (increments buffer → increments violations → at `MaxViolations` → disconnect). Buffer system provides leniency against false flags.

### Configuration

Configuration comes from `oconfig` (`github.com/oomph-ac/oconfig`), not local files. Key groups:
- `oconfig.Combat()` — maximum attack angle, client entity tracking toggle.
- `oconfig.Movement()` — acceptance/persuasion/correction thresholds.
- `oconfig.Network()` — latency, rewind, entity tracking settings.
- `oconfig.Global` — prefix, command name, legacy events toggle.

Per-detection thresholds are read by key `"Type_SubType"` via `oconfig.DtcOpts()`.

The `player.Opts` struct wraps `oconfig` values plus `LocalCombatOpts` for strictness-vs-leniency tuning of combat validation.

### Key design decisions

- **Forked Dragonfly**: The project replaces `github.com/df-mc/dragonfly` with `github.com/oomph-ac/dragonfly` (see `replace` directive in `go.mod`). This forked Dragonfly provides additional hooks needed for proxy-side block registration and chunk handling.
- **Multi-version support**: Via `lib/portal` session translator and custom protocol implementations. The `example/default` example uses a single protocol (`proto924`), while `example/spectrum` uses `legacyver` auto-negotiation.
- **No test suite**: The `tests/` directory is gitignored. There's no test framework setup.
- **Double combat components**: Both server-state and client-state entity trackers/combat systems run in parallel. Server-state is authoritative (blocks the attack), client-state is informative (flags but doesn't block) — allowing for detections that need client-perceived entity positions.

## Common tasks

- **Add a new detection**: Create `player/detection/new_type.go` implementing `Detection`, register in `player/detection/register.go`, and hook into the relevant component.
- **Add a new component**: Create the interface in `player/*.go` + implementation in `player/component/`, register in `player/component/register.go`.
- **Run the proxy**: `go run ./example/default <local_port> <remote_addr>` (or `./example/spectrum` with 3 args for Spectrum mode).
- **Set environment**: `PPROF_ENABLED=1` for profiling UI, `OOMPH_SENTRY_DSN` for Sentry error reporting, `OOMPH_GAMEMODE_TEST_BECAUSE_DEV` for a creative-mode dev command.
