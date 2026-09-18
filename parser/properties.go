package parser

import (
	"fmt"
	"log"
)

// ParseObjectProperties is the exported wrapper for parseObjectProperties.
func (d *DataBlobParser) ParseObjectProperties(r *BinaryReader, binarySize int, header *ObjectHeader) (map[string]Property, error) {
	return d.parseObjectProperties(r, binarySize, header)
}

// parseObjectProperties reads an object's property list, staying within binarySize.
// The recover() turns BinaryReader panics into errors instead of crashing the run.
func (d *DataBlobParser) parseObjectProperties(r *BinaryReader, binarySize int, header *ObjectHeader) (props map[string]Property, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("property parse panic: %v", e)
		}
	}()

	posStart := r.Position()

	// Only Actors carry a parentObject and a components list up front.
	if header != nil && header.Type == "Actor" {
		r.ReadString()
		r.ReadString()
		componentCount := r.ReadInt32()
		for i := int32(0); i < componentCount; i++ {
			r.ReadString()
			r.ReadString()
		}
	}

	properties := make(map[string]Property)
	useCompleteTagType := d.isCompletePropertyTagType()

	if useCompleteTagType {
		r.ReadUInt8()
	}

	remainingSize := binarySize - (r.Position() - posStart)
	posPropsStart := r.Position()

	for r.Position()-posPropsStart < remainingSize-4 {
		name := r.ReadString()
		if name == "None" {
			break
		}

		var propType, structSubtype, arraySubtype, byteEnumName, mapKeyType, mapValueType string
		var propBinarySize int32
		var propFlags uint8
		var boolValueFromHeader bool

		if useCompleteTagType {
			tn := d.readFPropertyTagNode(r)
			propType = tn.name
			propBinarySize = r.ReadInt32()
			propFlags = r.ReadUInt8()
			if propFlags&0x1 != 0 {
				r.ReadInt32()
			}
			if propFlags&0x2 != 0 {
				r.Skip(16)
			}
			if len(tn.children) > 0 {
				switch propType {
				case "StructProperty":
					structSubtype = tn.children[0].name
				case "ArrayProperty":
					arraySubtype = tn.children[0].name
					if arraySubtype == "StructProperty" && len(tn.children[0].children) > 0 {
						structSubtype = tn.children[0].children[0].name
					}
				case "ByteProperty", "EnumProperty":
					byteEnumName = tn.children[0].name
				case "MapProperty":
					if len(tn.children) >= 2 {
						mapKeyType = tn.children[0].name
						mapValueType = tn.children[1].name
					}
				case "SetProperty":
					arraySubtype = tn.children[0].name
				}
			}
		} else {
			propType = r.ReadString()
			propBinarySize = r.ReadInt32()
			r.ReadInt32()
			switch propType {
			case "ArrayProperty":
				arraySubtype = r.ReadString()
			case "StructProperty":
				structSubtype = r.ReadString()
				r.Skip(16)
			case "SetProperty":
				arraySubtype = r.ReadString()
			case "BoolProperty":
				boolValueFromHeader = r.ReadInt8() > 0
			case "ByteProperty":
				byteEnumName = r.ReadString()
			case "EnumProperty":
				byteEnumName = r.ReadString()
			case "MapProperty":
				mapKeyType = r.ReadString()
				mapValueType = r.ReadString()
			}
			hasGuid := r.ReadUInt8()
			if hasGuid != 0 {
				r.Skip(16)
			}
		}

		if propType == "BoolProperty" && !useCompleteTagType {
			properties[name] = Property{Type: propType, Value: boolValueFromHeader}
			continue
		}

		if d.skipProps[name] {
			r.Skip(int(propBinarySize))
			continue
		}

		posAfterHeader := r.Position()
		value, pErr := d.parsePropertyValue(r, propType, int(propBinarySize), useCompleteTagType,
			structSubtype, arraySubtype, propFlags, byteEnumName, mapKeyType, mapValueType, name)
		if pErr != nil {
			r.SetPosition(posAfterHeader + int(propBinarySize))
		} else {
			properties[name] = Property{Type: propType, Value: value}
		}
	}

	if r.Position()-posStart < binarySize-4 {
		hasGuid := r.ReadInt32()
		if hasGuid > 0 {
			r.Skip(16)
		}
	}

	remainingLen := binarySize - (r.Position() - posStart)
	if remainingLen > 0 && header != nil {
		special, sErr := d.parseSpecialProperties(r, header.ClassName, remainingLen)
		if sErr != nil {
			log.Printf("Warning: special properties parse failed for %s (%s): %v (continuing without them)", header.Reference.PathName, header.ClassName, sErr)
		} else if special != nil {
			properties["__special__"] = Property{Type: "Special", Value: special}
		}
	}

	return properties, nil
}

type tagNode struct {
	name     string
	children []tagNode
}

func (d *DataBlobParser) readFPropertyTagNode(r *BinaryReader) tagNode {
	name := r.ReadString()
	count := r.ReadInt32()
	node := tagNode{name: name}
	for i := int32(0); i < count; i++ {
		node.children = append(node.children, d.readFPropertyTagNode(r))
	}
	return node
}

type nestedPropHeader struct {
	propType            string
	propBinarySize      int32
	propFlags           uint8
	structSubtype       string
	arraySubtype        string
	byteEnumName        string
	boolValueFromHeader bool
}

func (d *DataBlobParser) parseNestedPropertyHeader(r *BinaryReader, useCompleteTagType bool) nestedPropHeader {
	h := nestedPropHeader{}
	if useCompleteTagType {
		tn := d.readFPropertyTagNode(r)
		h.propType = tn.name
		h.propBinarySize = r.ReadInt32()
		h.propFlags = r.ReadUInt8()
		if h.propFlags&0x1 != 0 {
			r.ReadInt32()
		}
		if h.propFlags&0x2 != 0 {
			r.Skip(16)
		}
		if len(tn.children) > 0 {
			switch h.propType {
			case "StructProperty":
				h.structSubtype = tn.children[0].name
			case "ArrayProperty":
				h.arraySubtype = tn.children[0].name
				if h.arraySubtype == "StructProperty" && len(tn.children[0].children) > 0 {
					h.structSubtype = tn.children[0].children[0].name
				}
			case "ByteProperty":
				h.byteEnumName = tn.children[0].name
			}
		}
	} else {
		h.propType = r.ReadString()
		h.propBinarySize = r.ReadInt32()
		r.ReadInt32()
		switch h.propType {
		case "ArrayProperty":
			h.arraySubtype = r.ReadString()
		case "StructProperty":
			h.structSubtype = r.ReadString()
			r.Skip(16)
		case "BoolProperty":
			h.boolValueFromHeader = r.ReadInt8() > 0
		case "ByteProperty":
			h.byteEnumName = r.ReadString()
		case "EnumProperty":
			h.byteEnumName = r.ReadString()
		case "MapProperty":
			r.ReadString()
			r.ReadString()
		}
		hasGuid := r.ReadUInt8()
		if hasGuid != 0 {
			r.Skip(16)
		}
	}
	return h
}

func (d *DataBlobParser) parsePropertyValue(
	r *BinaryReader,
	propType string,
	binarySize int,
	useCompleteTagType bool,
	structSubtype, arraySubtype string,
	propFlags uint8,
	byteEnumName, mapKeyType, mapValueType, propName string,
) (value interface{}, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("property value parse panic: %v", e)
		}
	}()

	posStart := r.Position()

	if useCompleteTagType && propType == "BoolProperty" {
		return (propFlags & 0x10) != 0, nil
	}

	switch propType {
	case "ObjectProperty", "InterfaceProperty":
		value = d.parseObjectProperty(r)
	case "StructProperty":
		value, err = d.parseStructProperty(r, binarySize, useCompleteTagType, structSubtype)
		if err != nil {
			return nil, err
		}
	case "ArrayProperty":
		value, err = d.parseArrayProperty(r, binarySize, useCompleteTagType, arraySubtype, structSubtype)
		if err != nil {
			return nil, err
		}
	case "IntProperty", "Int32Property":
		value = r.ReadInt32()
	case "UInt32Property":
		value = r.ReadUInt32()
	case "Int64Property":
		value = r.ReadInt64()
	case "UInt64Property":
		value = r.ReadUInt64()
	case "Int8Property":
		value = r.ReadInt8()
	case "UInt8Property":
		value = r.ReadUInt8()
	case "FloatProperty":
		value = r.ReadFloat32()
	case "DoubleProperty":
		value = r.ReadFloat64()
	case "BoolProperty":
		value = false
	case "StrProperty", "NameProperty":
		value = r.ReadString()
	case "ByteProperty":
		if byteEnumName != "" && byteEnumName != "None" {
			value = r.ReadString()
		} else {
			value = r.ReadUInt8()
		}
	case "EnumProperty":
		value = r.ReadString()
	case "TextProperty":
		value = d.parseTextPropertyValue(r)
	case "SetProperty":
		value = d.parseSetProperty(r, arraySubtype)
	case "MapProperty":
		value = d.parseMapProperty(r, mapKeyType, mapValueType, propName)
	default:
		r.Skip(binarySize)
		return nil, nil
	}

	bytesRead := r.Position() - posStart
	if bytesRead < binarySize {
		r.Skip(binarySize - bytesRead)
	} else if bytesRead > binarySize {
		r.SetPosition(posStart + binarySize)
	}

	return value, nil
}

func (d *DataBlobParser) parseObjectProperty(r *BinaryReader) map[string]string {
	levelName := r.ReadString()
	pathName := r.ReadString()
	return map[string]string{"levelName": levelName, "pathName": pathName}
}

func (d *DataBlobParser) parsePropertyList(r *BinaryReader) (map[string]interface{}, error) {
	useCompleteTagType := d.isCompletePropertyTagType()
	if useCompleteTagType {
		r.ReadUInt8()
	}
	props := make(map[string]interface{})
	for {
		name := r.ReadString()
		if name == "None" {
			break
		}
		nested := d.parseNestedPropertyHeader(r, useCompleteTagType)
		if nested.propType == "BoolProperty" && !useCompleteTagType {
			props[name] = nested.boolValueFromHeader
			continue
		}
		if d.skipProps[name] {
			r.Skip(int(nested.propBinarySize))
			continue
		}
		posAH := r.Position()
		val, err := d.parsePropertyValue(r, nested.propType, int(nested.propBinarySize),
			useCompleteTagType, nested.structSubtype, nested.arraySubtype, nested.propFlags,
			nested.byteEnumName, "", "", name)
		if err != nil {
			r.SetPosition(posAH + int(nested.propBinarySize))
		} else {
			props[name] = val
		}
	}
	return props, nil
}

func (d *DataBlobParser) parseTextPropertyValue(r *BinaryReader) interface{} {
	flags := r.ReadUInt32()
	historyType := r.ReadInt8()
	result := map[string]interface{}{"flags": flags, "historyType": historyType}
	switch historyType {
	case 0:
		result["namespace"] = r.ReadString()
		result["key"] = r.ReadString()
		result["value"] = r.ReadString()
	case 1, 2:
		result["sourceFmt"] = d.parseTextPropertyValue(r)
		argsCount := r.ReadInt32()
		var args []interface{}
		for i := int32(0); i < argsCount; i++ {
			argName := r.ReadString()
			vt := r.ReadUInt8()
			arg := map[string]interface{}{"name": argName, "valueType": vt}
			if vt == 4 {
				arg["argumentValue"] = d.parseTextPropertyValue(r)
			}
			args = append(args, arg)
		}
		result["arguments"] = args
	case 3:
		result["sourceText"] = d.parseTextPropertyValue(r)
		result["transformType"] = r.ReadUInt8()
	case 4:
		result["tableId"] = r.ReadString()
		result["textKey"] = r.ReadString()
	case -1:
		hasCulture := r.ReadInt32()
		result["hasCultureInvariantString"] = hasCulture == 1
		if hasCulture == 1 {
			result["value"] = r.ReadString()
		}
	}
	return result
}

func (d *DataBlobParser) parseSetProperty(r *BinaryReader, setSubtype string) interface{} {
	numElementsToRemove := r.ReadInt32()
	count := r.ReadInt32()
	var values []interface{}
	for i := int32(0); i < count; i++ {
		switch setSubtype {
		case "UInt32Property":
			values = append(values, r.ReadUInt32())
		case "IntProperty":
			values = append(values, r.ReadInt32())
		case "ObjectProperty":
			ln := r.ReadString()
			pn := r.ReadString()
			values = append(values, map[string]string{"levelName": ln, "pathName": pn})
		case "NameProperty", "StrProperty":
			values = append(values, r.ReadString())
		case "StructProperty":
			// Struct elements in a set are basically always 16-byte GUIDs.
			guid := r.ReadBytes(16)
			values = append(values, guid)
		default:
			return map[string]interface{}{"type": "SetProperty", "subType": setSubtype, "skipped": true}
		}
	}
	return map[string]interface{}{
		"type": "SetProperty", "subType": setSubtype,
		"numElementsToRemove": numElementsToRemove, "values": values,
	}
}

func (d *DataBlobParser) parseMapProperty(r *BinaryReader, keyType, valueType, propName string) interface{} {
	numElementsToRemove := r.ReadInt32()
	count := r.ReadInt32()
	var entries []interface{}
	for i := int32(0); i < count; i++ {
		var key, val interface{}
		switch keyType {
		case "IntProperty":
			key = r.ReadInt32()
		case "Int64Property":
			key = r.ReadInt64()
		case "StrProperty", "NameProperty":
			key = r.ReadString()
		case "ObjectProperty":
			ln := r.ReadString()
			pn := r.ReadString()
			key = map[string]string{"levelName": ln, "pathName": pn}
		case "EnumProperty":
			key = r.ReadString()
		case "ByteProperty":
			key = r.ReadUInt8()
		case "StructProperty":
			if propName == "mSaveData" || propName == "mUnresolvedSaveData" {
				x := r.ReadInt32()
				y := r.ReadInt32()
				z := r.ReadInt32()
				key = map[string]int32{"x": x, "y": y, "z": z}
			} else {
				key, _ = d.parsePropertyList(r)
			}
		default:
			return map[string]interface{}{"type": "MapProperty", "keyType": keyType, "valueType": valueType, "skipped": true}
		}
		switch valueType {
		case "IntProperty":
			val = r.ReadInt32()
		case "Int64Property":
			val = r.ReadInt64()
		case "StrProperty", "NameProperty":
			val = r.ReadString()
		case "ObjectProperty":
			ln := r.ReadString()
			pn := r.ReadString()
			val = map[string]string{"levelName": ln, "pathName": pn}
		case "EnumProperty":
			val = r.ReadString()
		case "ByteProperty":
			val = r.ReadUInt8()
		case "StructProperty":
			val, _ = d.parsePropertyList(r)
		default:
			return map[string]interface{}{"type": "MapProperty", "keyType": keyType, "valueType": valueType, "skipped": true}
		}
		entries = append(entries, map[string]interface{}{"key": key, "value": val})
	}
	return map[string]interface{}{
		"type": "MapProperty", "keyType": keyType, "valueType": valueType,
		"numElementsToRemove": numElementsToRemove, "entries": entries,
	}
}

func (d *DataBlobParser) parseStructProperty(r *BinaryReader, binarySize int, useCompleteTagType bool, structSubtype string) (interface{}, error) {
	defer func() {
		if e := recover(); e != nil {
			_ = e
		}
	}()

	posStart := r.Position()
	useDouble := d.ctx.ObjectVersion >= 41
	readCoord := func() float64 {
		if useDouble {
			return r.ReadFloat64()
		}
		return float64(r.ReadFloat32())
	}

	var value interface{}

	switch structSubtype {
	case "Vector", "Rotator":
		value = map[string]float64{"x": readCoord(), "y": readCoord(), "z": readCoord()}
	case "Box":
		value = map[string]interface{}{
			"min":     map[string]float64{"x": readCoord(), "y": readCoord(), "z": readCoord()},
			"max":     map[string]float64{"x": readCoord(), "y": readCoord(), "z": readCoord()},
			"isValid": r.ReadUInt8(),
		}
	case "PlayerInfoHandle":
		sp := r.ReadUInt8()
		if d.ctx.ObjectVersion >= 57 {
			value = map[string]interface{}{"serviceProvider": sp, "playerInfoTableIndex": r.ReadInt32()}
		} else {
			value = map[string]interface{}{"serviceProvider": sp, "playerInfoTableIndex": r.ReadUInt8()}
		}
	case "Transform":
		value = map[string]interface{}{
			"rotation":    map[string]float64{"x": readCoord(), "y": readCoord(), "z": readCoord(), "w": readCoord()},
			"translation": map[string]float64{"x": readCoord(), "y": readCoord(), "z": readCoord()},
			"scale":       map[string]float64{"x": readCoord(), "y": readCoord(), "z": readCoord()},
		}
	case "FluidBox":
		value = map[string]float32{"value": r.ReadFloat32()}
	case "LinearColor":
		value = map[string]float64{"r": float64(r.ReadFloat32()), "g": float64(r.ReadFloat32()), "b": float64(r.ReadFloat32()), "a": float64(r.ReadFloat32())}
	case "Color":
		value = map[string]uint8{"r": r.ReadUInt8(), "g": r.ReadUInt8(), "b": r.ReadUInt8(), "a": r.ReadUInt8()}
	case "InventoryItem":
		ln := r.ReadString()
		pn := r.ReadString()
		item := map[string]interface{}{"itemClass": map[string]string{"levelName": ln, "pathName": pn}}
		if d.ctx.ObjectVersion >= 43 {
			bHasValidStruct := r.ReadInt32()
			if bHasValidStruct >= 1 {
				sln := r.ReadString()
				spn := r.ReadString()
				item["itemStateStructRef"] = map[string]string{"levelName": sln, "pathName": spn}
				payloadSize := r.ReadInt32()
				if payloadSize > 0 {
					r.Skip(int(payloadSize))
				}
			}
		} else {
			ln2 := r.ReadString()
			pn2 := r.ReadString()
			item["legacyItemStateActor"] = map[string]string{"levelName": ln2, "pathName": pn2}
		}
		value = item
	case "InventoryStack":
		stack := make(map[string]interface{})
		for {
			pn := r.ReadString()
			if pn == "None" {
				break
			}
			nested := d.parseNestedPropertyHeader(r, useCompleteTagType)
			if nested.propType == "BoolProperty" && !useCompleteTagType {
				stack[pn] = nested.boolValueFromHeader
				continue
			}
			posAH := r.Position()
			val, err := d.parsePropertyValue(r, nested.propType, int(nested.propBinarySize),
				useCompleteTagType, nested.structSubtype, nested.arraySubtype, nested.propFlags,
				nested.byteEnumName, "", "", pn)
			if err != nil {
				r.SetPosition(posAH + int(nested.propBinarySize))
			} else {
				stack[pn] = val
			}
		}
		value = stack
	default:
		dynStruct := make(map[string]interface{})
		posEnd := posStart + binarySize
		for r.Position() < posEnd {
			pn := r.ReadString()
			if pn == "None" {
				break
			}
			nested := d.parseNestedPropertyHeader(r, useCompleteTagType)
			if nested.propType == "BoolProperty" && !useCompleteTagType {
				dynStruct[pn] = nested.boolValueFromHeader
				continue
			}
			posAH := r.Position()
			val, err := d.parsePropertyValue(r, nested.propType, int(nested.propBinarySize),
				useCompleteTagType, nested.structSubtype, nested.arraySubtype, nested.propFlags,
				nested.byteEnumName, "", "", pn)
			if err != nil {
				r.SetPosition(posAH + int(nested.propBinarySize))
			} else {
				dynStruct[pn] = val
			}
		}
		structBytesRead := r.Position() - posStart
		if structBytesRead < binarySize {
			r.Skip(binarySize - structBytesRead)
		} else if structBytesRead > binarySize {
			r.SetPosition(posEnd)
		}
		value = dynStruct
	}

	return map[string]interface{}{"type": structSubtype, "value": value}, nil
}

func (d *DataBlobParser) parseArrayProperty(r *BinaryReader, binarySize int, useCompleteTagType bool, arraySubtype string, structSubtype string) (value interface{}, err error) {
	defer func() {
		if e := recover(); e != nil {
			value = nil
			err = nil
		}
	}()

	posStart := r.Position()
	var items []interface{}

	switch arraySubtype {
	case "IntProperty", "Int32Property":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			items = append(items, r.ReadInt32())
		}
	case "Int64Property":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			items = append(items, r.ReadInt64())
		}
	case "FloatProperty":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			items = append(items, r.ReadFloat32())
		}
	case "ByteProperty":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			items = append(items, r.ReadUInt8())
		}
	case "ObjectProperty", "SoftObjectProperty", "InterfaceProperty":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			ln := r.ReadString()
			pn := r.ReadString()
			items = append(items, map[string]string{"levelName": ln, "pathName": pn})
		}
	case "EnumProperty":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			items = append(items, r.ReadString())
		}
	case "StrProperty", "NameProperty":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			items = append(items, r.ReadString())
		}
	case "BoolProperty":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			if useCompleteTagType {
				v := r.ReadUInt8()
				items = append(items, (v&0x10) != 0)
			} else {
				v := r.ReadInt8()
				items = append(items, v > 0)
			}
		}
	case "UInt32Property":
		count := r.ReadInt32()
		for i := int32(0); i < count; i++ {
			items = append(items, r.ReadUInt32())
		}
	case "StructProperty":
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
		// Struct array elements are always parsed as property lists, regardless of
		// format. Known shapes (Vector, Transform, ...) only get special handling
		// as standalone StructProperty, not inside arrays.
		for i := int32(0); i < structCount; i++ {
			item := make(map[string]interface{})
			for {
				pn := r.ReadString()
				if pn == "None" {
					break
				}
				nested := d.parseNestedPropertyHeader(r, useCompleteTagType)
				if nested.propType == "BoolProperty" && !useCompleteTagType {
					item[pn] = nested.boolValueFromHeader
					continue
				}
				posAH := r.Position()
				val, err := d.parsePropertyValue(r, nested.propType, int(nested.propBinarySize),
					useCompleteTagType, nested.structSubtype, nested.arraySubtype, nested.propFlags,
					nested.byteEnumName, "", "", pn)
				if err != nil {
					r.SetPosition(posAH + int(nested.propBinarySize))
				} else {
					item[pn] = val
				}
			}
			items = append(items, item)
		}
	default:
		bytesRead := r.Position() - posStart
		remaining := binarySize - bytesRead
		if remaining > 0 {
			r.Skip(remaining)
		}
		return map[string]interface{}{"type": arraySubtype, "skipped": true}, nil
	}

	return map[string]interface{}{"type": arraySubtype, "count": len(items), "items": items}, nil
}
