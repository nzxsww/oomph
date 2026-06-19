package proto924

import (
	"github.com/oomph-ac/oomph/protocol/proto975"
	"github.com/sandertv/gophertunnel/minecraft"
	goprotocol "github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Protocol implements minecraft.Protocol for Minecraft 1.26.2 (protocol 924).
// It chains to proto975 for base pools and conversion so that each protocol
// only needs to handle its own differences from the immediately newer version.
type Protocol struct{}

func (Protocol) ID() int32 { return 924 }
func (Protocol) Ver() string { return "1.26.2" }

// Packets builds on proto975's pool, removing packets added after 924.
func (Protocol) Packets(listener bool) packet.Pool {
	pool := proto975.Protocol{}.Packets(listener)
	excluded := []uint32{
		packet.IDClientBoundDataStore,
		packet.IDServerBoundDataStore,
		packet.IDResourcePacksReadyForValidation,
		packet.IDLocatorBar,
		packet.IDPartyChanged,
		packet.IDServerBoundDataDrivenScreenClosed,
		packet.IDSyncWorldClocks,
		packet.IDClientBoundAttributeLayerSync,
		packet.IDServerStoreInfo,
		packet.IDServerPresenceInfo,
	}
	for _, id := range excluded {
		delete(pool, id)
	}
	if _, ok := pool[packet.IDPlayerEnchantOptions]; ok {
		pool[packet.IDPlayerEnchantOptions] = func() packet.Packet {
			return &PlayerEnchantOptions{}
		}
	}
	return pool
}

func (Protocol) NewReader(r minecraft.ByteReader, shieldID int32, enableLimits bool) goprotocol.IO {
	return newReader(goprotocol.NewReader(r, shieldID, enableLimits))
}

func (Protocol) NewWriter(w minecraft.ByteWriter, shieldID int32) goprotocol.IO {
	return newWriter(goprotocol.NewWriter(w, shieldID))
}

// ConvertToLatest converts 924-specific packets then delegates to proto975.
func (Protocol) ConvertToLatest(pk packet.Packet, conn *minecraft.Conn) []packet.Packet {
	switch p := pk.(type) {
	case *PlayerEnchantOptions:
		result := &packet.PlayerEnchantOptions{
			Options: make([]goprotocol.EnchantmentOption, len(p.Options)),
		}
		for i, opt := range p.Options {
			result.Options[i].Cost = uint8(opt.Cost)
			result.Options[i].Enchantments = opt.Enchantments
			result.Options[i].Name = opt.Name
			result.Options[i].RecipeNetworkID = opt.RecipeNetworkID
		}
		return proto975.Protocol{}.ConvertToLatest(result, conn)
	default:
		return proto975.Protocol{}.ConvertToLatest(pk, conn)
	}
}

// ConvertFromLatest runs proto975's conversion first, then applies 924-specific changes.
func (Protocol) ConvertFromLatest(pk packet.Packet, conn *minecraft.Conn) []packet.Packet {
	pk = proto975.Protocol{}.ConvertFromLatest(pk, conn)[0]
	switch p := pk.(type) {
	case *packet.StartGame:
		p.ForceExperimentalGameplay = goprotocol.Option(false)
		return []packet.Packet{p}
	case *packet.PlayerEnchantOptions:
		result := &PlayerEnchantOptions{
			Options: make([]enchantOption, len(p.Options)),
		}
		for i, opt := range p.Options {
			result.Options[i].Cost = uint32(opt.Cost)
			result.Options[i].Enchantments = opt.Enchantments
			result.Options[i].Name = opt.Name
			result.Options[i].RecipeNetworkID = opt.RecipeNetworkID
		}
		return []packet.Packet{result}
	default:
		switch pk.ID() {
		case packet.IDClientBoundDataStore,
			packet.IDServerBoundDataStore,
			packet.IDResourcePacksReadyForValidation,
			packet.IDLocatorBar,
			packet.IDPartyChanged,
			packet.IDServerBoundDataDrivenScreenClosed,
			packet.IDSyncWorldClocks,
			packet.IDClientBoundAttributeLayerSync,
			packet.IDServerStoreInfo,
			packet.IDServerPresenceInfo:
			return nil
		}
		return []packet.Packet{pk}
	}
}
