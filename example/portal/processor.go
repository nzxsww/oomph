package main

import (
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/paroxity/portal/event"
	"github.com/paroxity/portal/server"
	"github.com/paroxity/portal/session"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var _ session.Handler = &Processor{}

// Processor implements session.Handler to bridge Portal's session events with Oomph's anti-cheat.
type Processor struct {
	session.NopHandler

	s            *session.Session
	pl           atomic.Pointer[player.Player]
	transferring atomic.Bool
}

// NewProcessor creates a new Processor for the given Portal session.
func NewProcessor(s *session.Session, listener *minecraft.Listener, log *slog.Logger) *Processor {
	pl := player.New(log, player.MonitoringState{
		IsReplay:    false,
		IsRecording: false,
		CurrentTime: time.Now(),
	}, listener)
	// Use EarlyConn — Conn() would deadlock waiting for loginMu, but the
	// underlying minecraft.Conn is set before the lock is acquired.
	pl.SetConn(s.EarlyConn())

	component.Register(pl)
	detection.Register(pl)

	go pl.StartTicking()
	p := &Processor{s: s}
	p.pl.Store(pl)
	return p
}

// HandleStartGame modifies the GameData before it is sent to the client, enabling server-authoritative
// movement with rewind.
func (p *Processor) HandleStartGame(_ *event.Context, gd *minecraft.GameData) {
	gd.PlayerMovementSettings.RewindHistorySize = 100
}

// HandleServerBoundPacket handles packets sent by the client (server-bound). It delegates to Oomph's
// player.HandleClientPacket for anti-cheat validation.
func (p *Processor) HandleServerBoundPacket(ctx *event.Context, pk packet.Packet) {
	pl := p.pl.Load()
	if pl == nil || p.transferring.Load() {
		return
	}

	pkCtx := context.NewHandlePacketContext(&pk)
	pl.HandleClientPacket(pkCtx)

	if pkCtx.Cancelled() {
		ctx.Cancel()
		return
	}
}

// HandleClientBoundPacket handles packets sent by the server (client-bound). It delegates to Oomph's
// player.HandleServerPacket for anti-cheat validation.
func (p *Processor) HandleClientBoundPacket(ctx *event.Context, pk packet.Packet) {
	pl := p.pl.Load()
	if pl == nil || p.transferring.Load() {
		return
	}

	pkCtx := context.NewHandlePacketContext(&pk)
	pl.HandleServerPacket(pkCtx)

	if pkCtx.Cancelled() {
		ctx.Cancel()
		return
	}
}

// HandleTransfer is called when a session is being transferred to another server. It pauses Oomph's
// processing until the transfer completes.
func (p *Processor) HandleTransfer(_ *event.Context, _ *server.Server) {
	p.transferring.Store(true)
	if pl := p.pl.Load(); pl != nil {
		pl.PauseProcessing()
	}
}

// HandlePostTransfer is called after a transfer completes and the server connection has been swapped.
// It updates Oomph's state with the new server connection.
func (p *Processor) HandlePostTransfer() {
	p.transferring.Store(false)
	if pl := p.pl.Load(); pl != nil {
		pl.SetServerConn(p.s.ServerConn())
		// Remove save-the-world state from the player's world, we can start removing chunks once this ACK is processed.
		if w := pl.World(); w != nil {
			w.SetSTWTicks(300) // 15 seconds
		}
		pl.Effects().RemoveAll()
		pl.ACKs().Invalidate() // Invalidate all pending UpdateBlock acknowledgments.
		pl.ResumeProcessing()
	}
}

// HandleServerDisconnect handles the server connection being closed.
func (p *Processor) HandleServerDisconnect(_ *event.Context, _ error) {
	// The Portal session will handle disconnect forwarding to the client.
}

// HandleQuit handles the closing of a session.
func (p *Processor) HandleQuit() {
	if pl := p.pl.Load(); pl != nil {
		_ = pl.Close()
		p.pl.Store(nil)
	}
}

// Player returns the Oomph player associated with this processor.
func (p *Processor) Player() *player.Player {
	return p.pl.Load()
}
