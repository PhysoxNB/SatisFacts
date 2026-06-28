package parser

import (
	"fmt"
)

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
		if sErr == nil && special != nil {
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

func (d *DataBlobParser) parseSpecialProperties(r *BinaryReader, typePath string, remainingLen int) (interface{}, error) {
	defer func() {
		if e := recover(); e != nil {
			_ = e
		}
	}()

	posStart := r.Position()

	switch {
	case contains(typePath, "Build_ConveyorBeltMk") || contains(typePath, "Build_ConveyorLiftMk"):
		r.ReadInt32()
		return map[string]interface{}{"type": "ConveyorSpecialProperties"}, nil
	case contains(typePath, "FGConveyorChainActor"):
		return d.parseConveyorChainActorSpecial(r, remainingLen, posStart), nil
	case contains(typePath, "Build_PowerLine") || contains(typePath, "Build_XmassLightsLine"):
		return d.parsePowerLineSpecial(r, remainingLen, posStart), nil
	case contains(typePath, "BP_CircuitSubsystem"):
		return d.parseCircuitSpecial(r), nil
	case contains(typePath, "BP_GameState") || contains(typePath, "BP_GameMode"):
		return d.parseObjectsListSpecial(r), nil
	case contains(typePath, "FGLightweightBuildableSubsystem"):
		return d.parseBuildableSubsystemSpecial(r)
	default:
		bytesRead := r.Position() - posStart
		remaining := remainingLen - bytesRead
		if remaining > 0 {
			r.Skip(remaining)
		}
		return nil, nil
	}
}

func (d *DataBlobParser) parsePowerLineSpecial(r *BinaryReader, remainingLen, posStart int) interface{} {
	srcLn := r.ReadString()
	srcPn := r.ReadString()
	tgtLn := r.ReadString()
	tgtPn := r.ReadString()
	result := map[string]interface{}{
		"type":   "PowerLineSpecialProperties",
		"source": map[string]string{"levelName": srcLn, "pathName": srcPn},
		"target": map[string]string{"levelName": tgtLn, "pathName": tgtPn},
	}
	bytesRead := r.Position() - posStart
	if remainingLen-bytesRead >= 24 {
		result["sourceTranslation"] = map[string]float32{"x": r.ReadFloat32(), "y": r.ReadFloat32(), "z": r.ReadFloat32()}
		result["targetTranslation"] = map[string]float32{"x": r.ReadFloat32(), "y": r.ReadFloat32(), "z": r.ReadFloat32()}
	}
	return result
}

func (d *DataBlobParser) parseCircuitSpecial(r *BinaryReader) interface{} {
	count := r.ReadInt32()
	var circuits []interface{}
	for i := int32(0); i < count; i++ {
		id := r.ReadInt32()
		ln := r.ReadString()
		pn := r.ReadString()
		circuits = append(circuits, map[string]interface{}{
			"id":              id,
			"objectReference": map[string]string{"levelName": ln, "pathName": pn},
		})
	}
	return map[string]interface{}{"type": "CircuitSpecialProperties", "circuits": circuits}
}

func (d *DataBlobParser) parseObjectsListSpecial(r *BinaryReader) interface{} {
	count := r.ReadInt32()
	var objects []interface{}
	for i := int32(0); i < count; i++ {
		ln := r.ReadString()
		pn := r.ReadString()
		objects = append(objects, map[string]string{"levelName": ln, "pathName": pn})
	}
	return map[string]interface{}{"type": "ObjectsListSpecialProperties", "objects": objects}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// parseBuildableSubsystemSpecial walks the FGLightweightBuildableSubsystem,
// which packs huge arrays of lightweight buildables. Outside MAP mode we only
// keep a type->count tally to avoid blowing up memory.
func (d *DataBlobParser) parseBuildableSubsystemSpecial(r *BinaryReader) (interface{}, error) {
	// currentLightweightVersion, present from objectVersion 48 on.
	var lwVersion int32
	if d.ctx.ObjectVersion >= 48 {
		lwVersion = r.ReadInt32()
	}

	entriesCount := r.ReadInt32()
	if entriesCount < 0 || entriesCount > 10000 {
		return nil, fmt.Errorf("invalid entriesCount: %d", entriesCount)
	}

	typeCounts := make(map[string]int)
	var typeEntries map[string][]map[string]interface{}
	if d.mapMode {
		typeEntries = make(map[string][]map[string]interface{})
	}

	for i := int32(0); i < entriesCount; i++ {
		r.ReadString() // typeRef levelName
		typeRefPn := r.ReadString()
		instanceCount := r.ReadInt32()
		if instanceCount < 0 || instanceCount > 200000 {
			return nil, fmt.Errorf("invalid instanceCount: %d for %s", instanceCount, typeRefPn)
		}

		validCount := int32(0)

		// Walk the instances, dropping deleted actors as we go.
		for j := int32(0); j < instanceCount; j++ {
			// Transform is 10 doubles (80 bytes): rotation quat x,y,z,w, then
			// translation x,y,z, then scale x,y,z.
			var entry map[string]interface{}
			if d.mapMode {
				rx := r.ReadFloat64()
				ry := r.ReadFloat64()
				rz := r.ReadFloat64()
				rw := r.ReadFloat64()
				tx := r.ReadFloat64()
				ty := r.ReadFloat64()
				tz := r.ReadFloat64()
				r.ReadFloat64() // scale x
				r.ReadFloat64() // scale y
				r.ReadFloat64() // scale z
				entry = map[string]interface{}{
					"typePath": typeRefPn,
					"position": []float64{tx, ty, tz},
					"rotation": map[string]float64{"x": rx, "y": ry, "z": rz, "w": rw},
				}
			} else {
				r.Skip(80)
			}

			// ObjectReferences: swatch, material, pattern, skin (4 refs = 8 strings)
			r.ReadString(); r.ReadString() // swatch
			r.ReadString(); r.ReadString() // material
			r.ReadString(); r.ReadString() // pattern
			r.ReadString(); r.ReadString() // skin

			// Colors: 8 float32 = 32 bytes
			r.Skip(32)

			// Paint finish + pattern rotation + recipe + blueprint proxy
			r.ReadString(); r.ReadString() // paintFinish
			r.ReadUInt8()                   // patternRotation
			r.ReadString(); r.ReadString() // recipe
			r.ReadString(); r.ReadString() // blueprintProxy

			// FGDynamicStruct (if lightweightVersion >= 2)
			if lwVersion >= 2 {
				hasValidStruct := r.ReadInt32() >= 1
				if hasValidStruct {
					r.ReadString() // structReference levelName
					r.ReadString() // structReference pathName
					savedPayloadSize := r.ReadInt32()
					if savedPayloadSize > 0 {
						r.Skip(int(savedPayloadSize))
					}
				}
			}

			// PlayerInfoHandle (if lightweightVersion >= 3)
			if lwVersion >= 3 {
				if d.ctx.ObjectVersion >= 57 {
					r.ReadUInt8()  // serviceProvider
					r.ReadInt32()  // playerInfoTableIndex
				} else {
					r.ReadUInt8()  // serviceProvider
					r.ReadUInt8()  // playerInfoTableIndex
				}
			}

			validCount++
			if d.mapMode && entry != nil {
				typeEntries[typeRefPn] = append(typeEntries[typeRefPn], entry)
			}
		}
		typeCounts[typeRefPn] = int(validCount)
	}

	result := map[string]interface{}{
		"type":       "BuildableSubsystemSpecialProperties",
		"typeCounts": typeCounts,
	}
	if d.mapMode {
		result["typeEntries"] = typeEntries
	}
	return result, nil
}

// parseConveyorChainActorSpecial reads a belt chain: total length, the
// individual belt segments, and the items riding on it.
func (d *DataBlobParser) parseConveyorChainActorSpecial(r *BinaryReader, remainingLen, posStart int) interface{} {
	useDouble := d.ctx.ObjectVersion >= 41

	readVec3 := func() [3]float64 {
		if useDouble {
			return [3]float64{r.ReadFloat64(), r.ReadFloat64(), r.ReadFloat64()}
		}
		return [3]float64{float64(r.ReadFloat32()), float64(r.ReadFloat32()), float64(r.ReadFloat32())}
	}

	// Heads up: it's lastBelt first, then firstBelt. Backwards from what you'd guess.
	lastBeltLn := r.ReadString()
	lastBeltPn := r.ReadString()
	firstBeltLn := r.ReadString()
	firstBeltPn := r.ReadString()

	countBeltsInChain := r.ReadInt32()
	belts := make([]interface{}, 0, countBeltsInChain)
	for i := int32(0); i < countBeltsInChain; i++ {
		chainActorRefLn := r.ReadString()
		chainActorRefPn := r.ReadString()
		beltRefLn := r.ReadString()
		beltRefPn := r.ReadString()
		splinePointsCount := r.ReadInt32()
		for j := int32(0); j < splinePointsCount; j++ {
			readVec3() // location
			readVec3() // arriveTangent
			readVec3() // leaveTangent
		}
		belt := map[string]interface{}{
			"chainActorRef":    map[string]string{"levelName": chainActorRefLn, "pathName": chainActorRefPn},
			"beltRef":          map[string]string{"levelName": beltRefLn, "pathName": beltRefPn},
			"offsetAtStart":    r.ReadFloat32(),
			"startsAtLength":   r.ReadFloat32(),
			"endsAtLength":     r.ReadFloat32(),
			"firstItemIndex":   r.ReadInt32(),
			"lastItemIndex":    r.ReadInt32(),
			"beltIndexInChain": r.ReadInt32(),
		}
		belts = append(belts, belt)
	}

	totalLength := float64(r.ReadFloat32())
	totalNumberItemsMaybe := r.ReadInt32()
	firstChainItemIndex := r.ReadInt32()
	lastChainItemIndex := r.ReadInt32()
	countItemsInChain := r.ReadInt32()

	items := make([]interface{}, 0, countItemsInChain)
	for n := int32(0); n < countItemsInChain; n++ {
		itemClassLn := r.ReadString()
		itemClassPn := r.ReadString()
		// ItemState
		if d.ctx.ObjectVersion >= 43 {
			bHasValidStruct := r.ReadInt32() >= 1
			if bHasValidStruct {
				r.ReadString() // structRef levelName
				r.ReadString() // structRef pathName
				savedPayloadSize := r.ReadInt32()
				if savedPayloadSize > 0 {
					r.Skip(int(savedPayloadSize))
				}
			}
		} else {
			r.ReadString() // legacyItemStateActor levelName
			r.ReadString() // legacyItemStateActor pathName
		}
		position := r.ReadFloat32()
		items = append(items, map[string]interface{}{
			"item":     map[string]string{"levelName": itemClassLn, "pathName": itemClassPn},
			"position": position,
		})
	}

	return map[string]interface{}{
		"type":                  "ConveyorChainActorSpecialProperties",
		"firstBelt":             map[string]string{"levelName": firstBeltLn, "pathName": firstBeltPn},
		"lastBelt":              map[string]string{"levelName": lastBeltLn, "pathName": lastBeltPn},
		"beltsInChain":          belts,
		"totalLength":           totalLength,
		"totalNumberItemsMaybe": totalNumberItemsMaybe,
		"firstChainItemIndex":   firstChainItemIndex,
		"lastChainItemIndex":    lastChainItemIndex,
		"items":                 items,
	}
}

// parseVehicleSpecial parses vehicle physics data and train linking.
func (d *DataBlobParser) parseVehicleSpecial(r *BinaryReader, remainingLen, posStart int, typePath string) interface{} {
	useDouble := d.ctx.ObjectVersion >= 41

	readVec3 := func() [3]float64 {
		if useDouble {
			return [3]float64{r.ReadFloat64(), r.ReadFloat64(), r.ReadFloat64()}
		}
		return [3]float64{float64(r.ReadFloat32()), float64(r.ReadFloat32()), float64(r.ReadFloat32())}
	}

	// Vehicle transform
	readVec3() // position
	readVec3() // velocity
	readVec3() // angularVelocity

	// Object references (wheels, etc)
	objCount := r.ReadInt32()
	objects := make([]interface{}, 0, objCount)
	for i := int32(0); i < objCount; i++ {
		ln := r.ReadString()
		pn := r.ReadString()
		objects = append(objects, map[string]string{"levelName": ln, "pathName": pn})
	}

	result := map[string]interface{}{
		"type":    "VehicleSpecialProperties",
		"objects": objects,
	}

	// Trains also link to the cars in front and behind.
	bytesRead := r.Position() - posStart
	remaining := remainingLen - bytesRead
	if remaining > 0 && contains(typePath, "BP_Locomotive") {
		frontLn := r.ReadString()
		frontPn := r.ReadString()
		behindLn := r.ReadString()
		behindPn := r.ReadString()
		result["vehicleInFront"] = map[string]string{"levelName": frontLn, "pathName": frontPn}
		result["vehicleBehind"] = map[string]string{"levelName": behindLn, "pathName": behindPn}
		bytesRead = r.Position() - posStart
		remaining = remainingLen - bytesRead
	}

	// Eat whatever's left.
	if remaining > 0 {
		r.Skip(remaining)
	}

	return result
}

// parseDroneSpecial reads a drone's active and queued actions.
func (d *DataBlobParser) parseDroneSpecial(r *BinaryReader) interface{} {
	r.ReadInt32() // unknown int32 (always 0)
	activeCount := r.ReadInt32()
	activeActions := make([]interface{}, 0, activeCount)
	for i := int32(0); i < activeCount; i++ {
		ln := r.ReadString()
		pn := r.ReadString()
		activeActions = append(activeActions, map[string]string{"levelName": ln, "pathName": pn})
	}

	queuedCount := r.ReadInt32()
	queuedActions := make([]interface{}, 0, queuedCount)
	for i := int32(0); i < queuedCount; i++ {
		ln := r.ReadString()
		pn := r.ReadString()
		queuedActions = append(queuedActions, map[string]string{"levelName": ln, "pathName": pn})
	}

	return map[string]interface{}{
		"type":          "DroneSpecialProperties",
		"activeActions": activeActions,
		"queuedActions": queuedActions,
	}
}

// parsePlayerSpecial reads the EOS player data.
func (d *DataBlobParser) parsePlayerSpecial(r *BinaryReader) interface{} {
	flag := r.ReadUInt8()
	result := map[string]interface{}{
		"type": "PlayerSpecialProperties",
		"flag": flag,
	}
	if flag == 248 {
		r.ReadString() // 'EOS' identifier
		r.ReadString() // EOS data
	}
	return result
}
