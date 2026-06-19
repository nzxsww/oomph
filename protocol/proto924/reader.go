package proto924

import (
	"github.com/go-gl/mathgl/mgl32"
	goprotocol "github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

// reader implements the goprotocol.IO interface for reading protocol 924 (Minecraft 1.26.2) wire format.
// It embeds *goprotocol.Reader and overrides specific methods where the wire format differs from protocol 975.
type reader struct {
	*goprotocol.Reader
}

// newReader creates a new reader wrapping the given protocol.Reader.
func newReader(r *goprotocol.Reader) *reader {
	return &reader{Reader: r}
}

// BlockPos reads a BlockPos in protocol 924 format: X as Varint32, Y as Varuint32, Z as Varint32.
func (r *reader) BlockPos(x *goprotocol.BlockPos) {
	r.Varint32(&x[0])
	var y uint32
	r.Varuint32(&y)
	x[1] = int32(y)
	r.Varint32(&x[2])
}

// EntityMetadata reads entity metadata using the custom BlockPos for EntityDataTypeBlockPos.
func (r *reader) EntityMetadata(x *goprotocol.EntityMetadata) {
	*x = goprotocol.EntityMetadata{}

	var count uint32
	r.Varuint32(&count)
	for i := uint32(0); i < count; i++ {
		var key, dataType uint32
		r.Varuint32(&key)
		r.Varuint32(&dataType)
		switch dataType {
		case goprotocol.EntityDataTypeByte:
			var v byte
			r.Uint8(&v)
			(*x)[key] = v
		case goprotocol.EntityDataTypeInt16:
			var v int16
			r.Int16(&v)
			(*x)[key] = v
		case goprotocol.EntityDataTypeInt32:
			var v int32
			r.Varint32(&v)
			(*x)[key] = v
		case goprotocol.EntityDataTypeFloat32:
			var v float32
			r.Float32(&v)
			(*x)[key] = v
		case goprotocol.EntityDataTypeString:
			var v string
			r.String(&v)
			(*x)[key] = v
		case goprotocol.EntityDataTypeCompoundTag:
			var v map[string]any
			r.NBT(&v, nbt.NetworkLittleEndian)
			(*x)[key] = v
		case goprotocol.EntityDataTypeBlockPos:
			var v goprotocol.BlockPos
			r.BlockPos(&v)
			(*x)[key] = v
		case goprotocol.EntityDataTypeInt64:
			var v int64
			r.Varint64(&v)
			(*x)[key] = v
		case goprotocol.EntityDataTypeVec3:
			var v mgl32.Vec3
			r.Vec3(&v)
			(*x)[key] = v
		default:
			r.UnknownEnumOption(dataType, "entity metadata")
		}
	}
}

// ItemInstanceNew delegates to ItemInstance for protocol 924 compatibility.
func (r *reader) ItemInstanceNew(i *goprotocol.ItemInstance) {
	r.ItemInstance(i)
}

// Float64 reads a float64 value for interface compliance.
func (r *reader) Float64(x *float64) {
	r.Reader.Float64(x)
}

// compile-time interface check
var _ goprotocol.IO = (*reader)(nil)
