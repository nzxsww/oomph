package proto975

import (
	"github.com/sandertv/gophertunnel/minecraft"
	goprotocol "github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Protocol implements minecraft.Protocol for Minecraft 1.26.20 (protocol 975).
// This is an identity implementation: packets are already in the latest format, so
// no conversion is needed.
type Protocol struct{}

// ID returns the protocol version number.
func (Protocol) ID() int32 { return 975 }

// Ver returns the human-readable version string.
func (Protocol) Ver() string { return "1.26.20" }

// Packets returns the full standard packet pool for protocol 975.
func (Protocol) Packets(listener bool) packet.Pool {
	if listener {
		return packet.NewClientPool()
	}
	return packet.NewServerPool()
}

// NewReader creates a standard protocol reader.
func (Protocol) NewReader(r minecraft.ByteReader, shieldID int32, enableLimits bool) goprotocol.IO {
	return goprotocol.NewReader(r, shieldID, enableLimits)
}

// NewWriter creates a standard protocol writer.
func (Protocol) NewWriter(w minecraft.ByteWriter, shieldID int32) goprotocol.IO {
	return goprotocol.NewWriter(w, shieldID)
}

// ConvertToLatest returns the packet unchanged.
func (Protocol) ConvertToLatest(pk packet.Packet, _ *minecraft.Conn) []packet.Packet {
	return []packet.Packet{pk}
}

// ConvertFromLatest returns the packet unchanged.
func (Protocol) ConvertFromLatest(pk packet.Packet, _ *minecraft.Conn) []packet.Packet {
	return []packet.Packet{pk}
}
