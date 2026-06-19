package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-echarts/statsview"
	"github.com/go-echarts/statsview/viewer"
	"github.com/oomph-ac/oconfig"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/protocol/proto924"
	"github.com/oomph-ac/oomph/world"
	oomphutils "github.com/oomph-ac/oomph/utils"
	"github.com/paroxity/portal"
	"github.com/paroxity/portal/server"
	"github.com/paroxity/portal/session"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"

	_ "net/http/pprof"

	_ "github.com/oomph-ac/oomph/utils/collisions"
)

func init() {
	if dsn := os.Getenv("OOMPH_SENTRY_DSN"); dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{Dsn: dsn}); err != nil {
			panic(err)
		}
	}
}

var evHandler = player.NewExampleEventHandler()

func main() {
	logger := slog.Default()
	if len(os.Args) < 3 {
		logger.Info("Usage: ./oomph-portal <local_port> <remote_addr>")
		return
	}

	if os.Getenv("PPROF_ENABLED") != "" {
		viewer.SetConfiguration(viewer.WithTheme(viewer.ThemeWesteros), viewer.WithAddr("192.168.1.172:8080"))
		mgr := statsview.New()
		go mgr.Start()
	}

	debug.SetGCPercent(-1)
	debug.SetMemoryLimit(4 * 1024 * 1024 * 1024) // 4GB
	fmt.Println("process ID:", os.Getpid())

	// Use a static MOTD status provider instead of ForeignStatusProvider.
	// ForeignStatusProvider tries to ping the backend for status, but if the
	// address isn't directly reachable (e.g. 0.0.0.0) it hangs and prevents
	// clients from connecting. A MOTDStatusProvider always responds immediately.
	statusProvider := portal.NewMOTDStatusProvider("Oomph Portal")

	oconfig.Global = oconfig.DefaultConfig
	oconfig.Global.Movement.AcceptClientPosition = false
	oconfig.Global.Movement.PositionAcceptanceThreshold = 0.003
	oconfig.Global.Movement.AcceptClientVelocity = false
	oconfig.Global.Movement.VelocityAcceptanceThreshold = 0.077

	oconfig.Global.Movement.PersuasionThreshold = 0.001
	oconfig.Global.Movement.CorrectionThreshold = 0.003

	oconfig.Global.Combat.MaximumAttackAngle = 90
	oconfig.Global.Combat.EnableClientEntityTracking = true

	oconfig.Global.Network.GlobalMovementCutoffThreshold = -1
	oconfig.Global.Network.MaxEntityRewind = 6
	oconfig.Global.Network.MaxGhostBlockChain = 7
	oconfig.Global.Network.MaxKnockbackDelay = -1
	oconfig.Global.Network.MaxBlockUpdateDelay = -1

	// Register custom blocks here
	world.FinalizeBlockRegistry()

	autoLogin := false
	portalLogger := logrus.New()
	portalLogger.SetLevel(logrus.DebugLevel)
	proxy := portal.New(portal.Options{
		Logger:  portalLogger,
		Address: ":" + os.Args[1],
		ListenConfig: minecraft.ListenConfig{
			StatusProvider:      statusProvider,
			FlushRate:           -1, // Oomph handles flushing manually.
			TexturePacksRequired: false,
			AllowInvalidPackets:  false,
			AllowUnknownPackets:  false,
			AcceptedProtocols:   []minecraft.Protocol{proto924.Protocol{}},
		},
		AutoLogin: &autoLogin,
	})

	// Register the backend server so the load balancer can route players to it.
	proxy.ServerRegistry().AddServer(server.New("default", os.Args[2]))

	if err := proxy.Listen(); err != nil {
		panic(err)
	}

	go func() {
		var interrupt = make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)
		<-interrupt
		for _, s := range proxy.SessionStore().All() {
			_ = s.Conn().WritePacket(&packet.Disconnect{})
			s.Disconnect("Proxy restarting...")
		}
		time.Sleep(time.Second)
		os.Exit(0)
	}()

	oomphutils.InitializeBlockNameMapping()

	for {
		s, err := proxy.Accept()
		if err != nil {
			logger.Error("failed to accept session", "err", err)
			continue
		}

		go handleSession(s, proxy, logger)
	}
}

func handleSession(s *session.Session, proxy *portal.Portal, logger *slog.Logger) {
	f, err := os.OpenFile(fmt.Sprintf("./logs/%s.log", s.EarlyConn().IdentityData().DisplayName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0744)
	if err != nil {
		s.Disconnect("failed to create log file")
		return
	}

	playerLogHandler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	playerLog := slog.New(playerLogHandler)

	proc := NewProcessor(s, proxy.Listener(), playerLog)
	proc.Player().SetCloser(func() {
		_ = f.Close()
	})
	proc.Player().SetRecoverFunc(func(p *player.Player, err any) {
		fmt.Println("ERROR:", err)
		debug.PrintStack()
		fmt.Println("Please remember this is an example, and you should set this recovery function to something that can log errors, like Sentry.")
		os.Exit(1)
	})
	proc.Player().AddPerm(player.PermissionDebug)
	proc.Player().AddPerm(player.PermissionAlerts)
	proc.Player().AddPerm(player.PermissionLogs)
	proc.Player().HandleEvents(evHandler)

	// Set the handler before login so HandleStartGame can modify GameData.
	s.Handle(proc)

	logger.Info("starting login...", "player", s.EarlyConn().IdentityData().DisplayName)
	if err := s.Login(); err != nil {
		logger.Error("login failed", "player", s.EarlyConn().IdentityData().DisplayName, "err", err)
		s.Disconnect(err.Error())
		_ = f.Close()
		if !errors.Is(err, context.Canceled) {
			logger.Error("failed to login session", "err", err)
		}
		return
	}
	logger.Info("login completed", "player", s.EarlyConn().IdentityData().DisplayName)

	proc.Player().SetServerConn(s.ServerConn())
}
