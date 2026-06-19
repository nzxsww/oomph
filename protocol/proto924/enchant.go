package proto924

import (
	goprotocol "github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// PlayerEnchantOptions is a protocol 924-compatible version of packet.PlayerEnchantOptions.
// Protocol 975 changed EnchantmentOption.Cost from uint32 (read as Varuint32) to uint8 (read as Uint8).
// This struct uses the old wire format so that protocol 924 clients can correctly read/write the packet.
type PlayerEnchantOptions struct {
	Options []enchantOption
}

// ID returns the packet ID for PlayerEnchantOptions.
func (*PlayerEnchantOptions) ID() uint32 {
	return packet.IDPlayerEnchantOptions
}

// Marshal reads/writes the packet fields using protocol 924 wire format.
func (pk *PlayerEnchantOptions) Marshal(r goprotocol.IO) {
	goprotocol.Slice(r, &pk.Options)
}

// enchantOption is a protocol 924-compatible version of goprotocol.EnchantmentOption
// where Cost is uint32 (read as Varuint32) rather than uint8 (read as Uint8).
type enchantOption struct {
	Cost            uint32
	Enchantments    goprotocol.ItemEnchantments
	Name            string
	RecipeNetworkID uint32
}

// Marshal reads/writes a single enchant option using protocol 924 wire format.
func (e *enchantOption) Marshal(r goprotocol.IO) {
	r.Varuint32(&e.Cost)
	goprotocol.Single(r, &e.Enchantments)
	r.String(&e.Name)
	r.Varuint32(&e.RecipeNetworkID)
}

// compile-time interface checks
var _ packet.Packet = (*PlayerEnchantOptions)(nil)
var _ goprotocol.Marshaler = (*enchantOption)(nil)
