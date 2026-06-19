package proto924

import (
	"reflect"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
	goprotocol "github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

// writer implements the goprotocol.IO interface for writing protocol 924 wire format.
type writer struct {
	*goprotocol.Writer
}

// newWriter creates a new writer wrapping the given protocol.Writer.
func newWriter(w *goprotocol.Writer) *writer {
	return &writer{Writer: w}
}

// BlockPos writes a BlockPos in protocol 924 format: X as Varint32, Y as Varuint32, Z as Varint32.
func (w *writer) BlockPos(x *goprotocol.BlockPos) {
	w.Varint32(&x[0])
	y := uint32(x[1])
	w.Varuint32(&y)
	w.Varint32(&x[2])
}

// EntityMetadata writes entity metadata using the custom BlockPos for EntityDataTypeBlockPos.
func (w *writer) EntityMetadata(x *goprotocol.EntityMetadata) {
	l := uint32(len(*x))
	w.Varuint32(&l)

	keys := make([]int, 0, l)
	for k := range *x {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		key := uint32(k)
		value := (*x)[uint32(k)]
		w.Varuint32(&key)
		switch v := value.(type) {
		case byte:
			entityDataTypeByte := goprotocol.EntityDataTypeByte
			w.Varuint32(&entityDataTypeByte)
			w.Uint8(&v)
		case int16:
			entityDataTypeInt16 := goprotocol.EntityDataTypeInt16
			w.Varuint32(&entityDataTypeInt16)
			w.Int16(&v)
		case int32:
			entityDataTypeInt32 := goprotocol.EntityDataTypeInt32
			w.Varuint32(&entityDataTypeInt32)
			w.Varint32(&v)
		case float32:
			entityDataTypeFloat32 := goprotocol.EntityDataTypeFloat32
			w.Varuint32(&entityDataTypeFloat32)
			w.Float32(&v)
		case string:
			entityDataTypeString := goprotocol.EntityDataTypeString
			w.Varuint32(&entityDataTypeString)
			w.String(&v)
		case map[string]any:
			entityDataTypeCompoundTag := goprotocol.EntityDataTypeCompoundTag
			w.Varuint32(&entityDataTypeCompoundTag)
			w.NBT(&v, nbt.NetworkLittleEndian)
		case goprotocol.BlockPos:
			entityDataTypeBlockPos := goprotocol.EntityDataTypeBlockPos
			w.Varuint32(&entityDataTypeBlockPos)
			w.BlockPos(&v)
		case int64:
			entityDataTypeInt64 := goprotocol.EntityDataTypeInt64
			w.Varuint32(&entityDataTypeInt64)
			w.Varint64(&v)
		case mgl32.Vec3:
			entityDataTypeVec3 := goprotocol.EntityDataTypeVec3
			w.Varuint32(&entityDataTypeVec3)
			w.Vec3(&v)
		default:
			w.UnknownEnumOption(reflect.TypeOf(value), "entity metadata")
		}
	}
}

// ItemInstanceNew delegates to ItemInstance for protocol 924 compatibility.
func (w *writer) ItemInstanceNew(i *goprotocol.ItemInstance) {
	w.ItemInstance(i)
}

// Float64 writes a float64 value for interface compliance.
func (w *writer) Float64(x *float64) {
	w.Writer.Float64(x)
}

// compile-time interface check
var _ goprotocol.IO = (*writer)(nil)
