package editor

import (
	"fmt"
	"strings"

	"satisfacts/parser"
)

// DepotItem represents a single item in the Dimensional Depot's mStoredItems array.
type DepotItem struct {
	ItemClass string // pathName from ItemClass ObjectProperty
	Amount    int32
	StartByte int // byte offset within the object's property data
	EndByte   int // byte offset within the object's property data
}

// FindCentralStorageSubsystem searches TOC headers for FGCentralStorageSubsystem.
// Returns the object index and its header.
func FindCentralStorageSubsystem(tocData []byte, ctx *parser.VersionContext, objectCount int) (int, *parser.ObjectHeader, error) {
	if objectCount == 0 {
		return -1, nil, fmt.Errorf("no objects in level")
	}
	headers, err := parser.ParseObjectHeaders(parser.NewBinaryReader(tocData[4:]), ctx, objectCount)
	if err != nil {
		return -1, nil, fmt.Errorf("parsing TOC headers: %w", err)
	}
	for i, h := range headers {
		if strings.Contains(strings.ToLower(h.ClassName), "centralstoragesubsystem") {
			return i, &headers[i], nil
		}
	}
	return -1, nil, fmt.Errorf("FGCentralStorageSubsystem not found in TOC")
}

// ReorderDepotItems rewrites the Dimensional Depot's mStoredItems array so the
// items come out in the order given by newOrder, which holds original item
// indices (newOrder[0] is the item that should end up first).
//
// A permutation moves the same struct elements around, so the body length never
// changes and none of the enclosing size fields need touching.
func ReorderDepotItems(sd *SaveData, newOrder []int) error {
	_, levels, _, _, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		return fmt.Errorf("parsing body: %w", err)
	}

	for _, l := range levels {
		if !l.IsPersistent || l.ObjectCount == 0 {
			continue
		}
		tocData := sd.Decompressed[l.TocDataStart:l.TocDataEnd]
		idx, header, ferr := FindCentralStorageSubsystem(tocData, sd.VersionCtx, int(l.ObjectCount))
		if ferr != nil {
			continue
		}

		objStart := l.DataObjOffsets[idx]
		objEnd := objStart + l.DataObjSizes[idx]
		objData := sd.Decompressed[l.DataBlobStart+objStart : l.DataBlobStart+objEnd]

		items, err := FindStoredItemsArray(objData, header, sd.VersionCtx, l.SaveCustomVersions[idx])
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("depot has no items to reorder")
		}
		if err := validatePermutation(newOrder, len(items)); err != nil {
			return err
		}

		var rebuilt []byte
		for _, orig := range newOrder {
			rebuilt = append(rebuilt, objData[items[orig].StartByte:items[orig].EndByte]...)
		}

		structStart := l.DataBlobStart + objStart + items[0].StartByte
		structEnd := l.DataBlobStart + objStart + items[len(items)-1].EndByte

		// A validated permutation shuffles equal-size elements, so the total
		// byte count never changes. Copy directly into the existing slice to
		// avoid allocating a second body-sized buffer on large saves.
		if len(rebuilt) == structEnd-structStart {
			copy(sd.Decompressed[structStart:structEnd], rebuilt)
		} else {
			newDecompressed := make([]byte, 0, len(sd.Decompressed)-(structEnd-structStart)+len(rebuilt))
			newDecompressed = append(newDecompressed, sd.Decompressed[:structStart]...)
			newDecompressed = append(newDecompressed, rebuilt...)
			newDecompressed = append(newDecompressed, sd.Decompressed[structEnd:]...)
			sd.Decompressed = newDecompressed
		}
		return nil
	}

	return fmt.Errorf("FGCentralStorageSubsystem not found in save")
}

// validatePermutation rejects orders that would drop, duplicate or invent items.
// Writing one of those out produces a save the game may load but with a mangled
// depot, so it's worth failing loudly here.
func validatePermutation(order []int, n int) error {
	if len(order) != n {
		return fmt.Errorf("new order has %d entries but the depot has %d items", len(order), n)
	}
	seen := make([]bool, n)
	for _, o := range order {
		if o < 0 || o >= n {
			return fmt.Errorf("new order contains out-of-range index %d (want 0..%d)", o, n-1)
		}
		if seen[o] {
			return fmt.Errorf("new order contains duplicate index %d", o)
		}
		seen[o] = true
	}
	return nil
}

// FindStoredItemsArray locates the mStoredItems ArrayProperty within an object's
// binary data and returns the byte positions of each struct element.
func FindStoredItemsArray(objData []byte, header *parser.ObjectHeader, ctx *parser.VersionContext, saveCustomVersion int32) (elements []DepotItem, err error) {
	r := parser.NewBinaryReader(objData)
	useCompleteTagType := IsCompletePropertyTagType(ctx, saveCustomVersion)

	// Skip per-object header: saveCustomVersion + shouldMigrate (if levelVersion >= 38) + binarySize
	if ctx.LevelVersion >= 38 {
		r.ReadInt32() // saveCustomVersion
		r.ReadInt32() // shouldMigrate
	}
	r.ReadInt32() // binarySize - the actual object data follows

	// Actor header: parentObject, parentObject, componentCount, components
	if header.Type == "Actor" {
		r.ReadString()
		r.ReadString()
		componentCount := r.ReadInt32()
		for i := int32(0); i < componentCount; i++ {
			r.ReadString()
			r.ReadString()
		}
	}

	if useCompleteTagType {
		r.ReadUInt8() // complete tag type byte
	}

	// Walk properties
	for {
		name := r.ReadString()
		if name == "None" {
			break
		}

		var propType, arraySubtype string
		var propBinarySize int32
		var propFlags uint8

		if useCompleteTagType {
			tn := ReadFPropertyTagNode(r)
			propType = tn.Name
			propBinarySize = r.ReadInt32()
			propFlags = r.ReadUInt8()
			if propFlags&0x1 != 0 {
				r.ReadInt32()
			}
			if propFlags&0x2 != 0 {
				r.Skip(16)
			}
			if len(tn.Children) > 0 && propType == "ArrayProperty" {
				arraySubtype = tn.Children[0].Name
			}
		} else {
			propType = r.ReadString()
			propBinarySize = r.ReadInt32()
			r.ReadInt32()
			if propType == "ArrayProperty" {
				arraySubtype = r.ReadString()
			} else if propType == "StructProperty" {
				r.ReadString() // structSubtype
				r.Skip(16)
			}
			hasGuid := r.ReadUInt8()
			if hasGuid != 0 {
				r.Skip(16)
			}
		}

		propValueStart := r.Position()

		if name != "mStoredItems" {
			r.SetPosition(propValueStart + int(propBinarySize))
			continue
		}

		if propType != "ArrayProperty" || arraySubtype != "StructProperty" {
			return nil, fmt.Errorf("mStoredItems is not ArrayProperty<StructProperty>, got %s<%s>", propType, arraySubtype)
		}

		// Array value: int32 count, then optional header (old format), then struct elements
		structCount := r.ReadInt32()

		if !useCompleteTagType {
			r.ReadString() // hName
			r.ReadString() // hType
			r.ReadInt32()  // hBinSize
			r.ReadInt32()  // hIdx
			r.ReadString() // hSubtype
			r.Skip(16)
			hasGuid := r.ReadUInt8()
			if hasGuid != 0 {
				r.Skip(16)
			}
		}

		// Parse each struct element, tracking byte positions
		for i := int32(0); i < structCount; i++ {
			elemStart := r.Position()
			item := DepotItem{StartByte: elemStart}

			for {
				pn := r.ReadString()
				if pn == "None" {
					break
				}
				nested := ParseNestedPropertyHeader(r, useCompleteTagType)

				if nested.PropType == "BoolProperty" && !useCompleteTagType {
					continue
				}

				valStart := r.Position()

				switch nested.PropType {
				case "ObjectProperty", "InterfaceProperty":
					levelName := r.ReadString()
					pathName := r.ReadString()
					if pn == "ItemClass" {
						item.ItemClass = pathName
					}
					_ = levelName
				case "IntProperty":
					v := r.ReadInt32()
					if pn == "Amount" || pn == "amount" {
						item.Amount = v
					}
				case "Int64Property":
					r.ReadInt64()
				case "FloatProperty":
					r.ReadFloat32()
				case "StrProperty", "NameProperty":
					r.ReadString()
				case "EnumProperty":
					r.ReadString()
				case "ByteProperty":
					r.ReadUInt8()
				case "BoolProperty":
					// value is in flags for new format
				case "StructProperty":
					r.Skip(int(nested.PropBinarySize))
				default:
					r.Skip(int(nested.PropBinarySize))
				}

				expectedPos := valStart + int(nested.PropBinarySize)
				if r.Position() != expectedPos {
					r.SetPosition(expectedPos)
				}
			}

			item.EndByte = r.Position()
			elements = append(elements, item)
		}

		return elements, nil
	}

	return nil, fmt.Errorf("mStoredItems property not found in object")
}
