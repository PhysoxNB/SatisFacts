package parser

import (
	"fmt"
	"strings"
	"sync"
)

func (d *DataBlobParser) parseSpecialProperties(r *BinaryReader, typePath string, remainingLen int) (interface{}, error) {
	defer func() {
		if e := recover(); e != nil {
			_ = e
		}
	}()

	posStart := r.Position()

	switch specialKindFor(typePath) {
	case specialConveyor:
		r.ReadInt32()
		return map[string]interface{}{"type": "ConveyorSpecialProperties"}, nil
	case specialConveyorChain:
		return d.parseConveyorChainActorSpecial(r, remainingLen, posStart), nil
	case specialPowerLine:
		return d.parsePowerLineSpecial(r, remainingLen, posStart), nil
	case specialCircuit:
		return d.parseCircuitSpecial(r), nil
	case specialObjectsList:
		return d.parseObjectsListSpecial(r), nil
	case specialBuildableSubsystem:
		return d.parseBuildableSubsystemSpecial(r)
	case specialRailroadVehicle:
		return d.parseRailroadVehicleSpecial(r, remainingLen, posStart)
	default:
		bytesRead := r.Position() - posStart
		remaining := remainingLen - bytesRead
		if remaining > 0 {
			r.Skip(remaining)
		}
		return nil, nil
	}
}

// specialKind identifies which trailing custom-binary layout a class uses, if any.
type specialKind uint8

const (
	specialNone specialKind = iota
	specialConveyor
	specialConveyorChain
	specialPowerLine
	specialCircuit
	specialObjectsList
	specialBuildableSubsystem
	specialRailroadVehicle
)

// specialKindCache memoizes the dispatch below per class name. It used to run up
// to nine substring scans for every object in the save, which is millions of
// times over only a few thousand distinct class names.
var (
	specialKindMu    sync.RWMutex
	specialKindCache = make(map[string]specialKind)
)

func specialKindFor(typePath string) specialKind {
	specialKindMu.RLock()
	k, ok := specialKindCache[typePath]
	specialKindMu.RUnlock()
	if ok {
		return k
	}

	k = computeSpecialKind(typePath)

	specialKindMu.Lock()
	specialKindCache[typePath] = k
	specialKindMu.Unlock()
	return k
}

func computeSpecialKind(typePath string) specialKind {
	switch {
	case strings.Contains(typePath, "Build_ConveyorBeltMk") || strings.Contains(typePath, "Build_ConveyorLiftMk"):
		return specialConveyor
	case strings.Contains(typePath, "FGConveyorChainActor"):
		return specialConveyorChain
	case strings.Contains(typePath, "Build_PowerLine") || strings.Contains(typePath, "Build_XmassLightsLine"):
		return specialPowerLine
	case strings.Contains(typePath, "BP_CircuitSubsystem"):
		return specialCircuit
	case strings.Contains(typePath, "BP_GameState") || strings.Contains(typePath, "BP_GameMode"):
		return specialObjectsList
	case strings.Contains(typePath, "FGLightweightBuildableSubsystem"):
		return specialBuildableSubsystem
	case strings.Contains(typePath, "BP_Locomotive") || strings.Contains(typePath, "BP_FreightWagon"):
		return specialRailroadVehicle
	}
	return specialNone
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

// parseRailroadVehicleSpecial reads the binary data that follows the regular
// properties on AFGRailroadVehicle instances (locomotives and wagons).
// The layout is:
//
//	AFGVehicle: TArray<FVehiclePhysicsData> mStoredPhysicsData
//	AFGRailroadVehicle: FObjectReferenceDisc mCoupledVehicleFront
//	                    FObjectReferenceDisc mCoupledVehicleBack
//
// FVehiclePhysicsData = FName + FVector(3×double) + FQuat(4×double) + FVector + FVector + uint8
func (d *DataBlobParser) parseRailroadVehicleSpecial(r *BinaryReader, remainingLen, posStart int) (interface{}, error) {
	// Skip mStoredPhysicsData: TArray = int32 count, then count × FVehiclePhysicsData
	physicsCount := r.ReadInt32()
	if physicsCount < 0 || physicsCount > 10000 {
		return nil, fmt.Errorf("invalid physicsCount: %d", physicsCount)
	}
	for i := int32(0); i < physicsCount; i++ {
		r.ReadString() // FName BoneName
		r.Skip(24)     // FVector Position (3×double)
		r.Skip(32)     // FQuat Quaternion (4×double)
		r.Skip(24)     // FVector AngVel (3×double)
		r.Skip(24)     // FVector LinVel (3×double)
		r.ReadUInt8()  // uint8 Flags
	}

	// These two refs are the whole reason this function exists - everything
	// else above is just skipped past to get here.
	result := map[string]interface{}{
		"type": "RailroadVehicleSpecialProperties",
	}

	// mCoupledVehicleFront - the vehicle coupled to the front of this one
	frontLn := r.ReadString()
	frontPn := r.ReadString()
	result["mCoupledVehicleFront"] = map[string]string{"levelName": frontLn, "pathName": frontPn}

	// mCoupledVehicleBack - the vehicle coupled to the back of this one
	backLn := r.ReadString()
	backPn := r.ReadString()
	result["mCoupledVehicleBack"] = map[string]string{"levelName": backLn, "pathName": backPn}

	// Skip any remaining bytes
	bytesRead := r.Position() - posStart
	remaining := remainingLen - bytesRead
	if remaining > 0 {
		r.Skip(remaining)
	}

	return result, nil
}

// parseBuildableSubsystemSpecial walks the FGLightweightBuildableSubsystem,
// which packs huge arrays of lightweight buildables. We extract type->count
// tallies and individual transforms (for map rendering).
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
	// transforms per type: flat array of [qx,qy,qz,qw, tx,ty,tz] as float64
	typeTransforms := make(map[string][]float64)
	// beam lengths per type: float64 cm, one per instance (0 if not a beam)
	typeBeamLengths := make(map[string][]float64)
	// dynamicStruct payloads per type: raw bytes of FGDynamicStruct savedPayload
	typeDynamicPayloads := make(map[string][][]byte)
	// dynamicStruct reference pathName per type (first instance)
	typeDynamicRef := make(map[string]string)
	// blueprintProxy refs per type (one per instance, or empty string)
	typeBlueprintProxies := make(map[string][]string)

	for i := int32(0); i < entriesCount; i++ {
		r.ReadString() // typeRef levelName
		typeRefPn := r.ReadString()
		instanceCount := r.ReadInt32()
		if instanceCount < 0 || instanceCount > 10000000 {
			return nil, fmt.Errorf("invalid instanceCount: %d for %s", instanceCount, typeRefPn)
		}

		validCount := int32(0)
		transforms := make([]float64, 0, instanceCount*7)

		// Walk the instances, dropping deleted actors as we go.
		for j := int32(0); j < instanceCount; j++ {
			// Transform is 10 doubles (80 bytes): rotation quat x,y,z,w, then
			// translation x,y,z, then scale x,y,z.
			qx := r.ReadFloat64()
			qy := r.ReadFloat64()
			qz := r.ReadFloat64()
			qw := r.ReadFloat64()
			tx := r.ReadFloat64()
			ty := r.ReadFloat64()
			tz := r.ReadFloat64()
			r.Skip(24) // scale x,y,z (3 doubles = 24 bytes)

			transforms = append(transforms, qx, qy, qz, qw, tx, ty, tz)

			// ObjectReferences: swatch, material, pattern, skin (4 refs = 8 strings)
			r.ReadString()
			r.ReadString() // swatch
			r.ReadString()
			r.ReadString() // material
			r.ReadString()
			r.ReadString() // pattern
			r.ReadString()
			r.ReadString() // skin

			// Colors: 8 float32 = 32 bytes
			r.Skip(32)

			// Paint finish + pattern rotation + recipe + blueprint proxy
			r.ReadString()
			r.ReadString() // paintFinish
			r.ReadUInt8()  // patternRotation
			r.ReadString()
			r.ReadString() // recipe
			bpProxyLevel := r.ReadString()
			bpProxyPath := r.ReadString() // blueprintProxy

			// FGDynamicStruct (if lightweightVersion >= 2)
			beamLength := 0.0
			if lwVersion >= 2 {
				hasValidStruct := r.ReadInt32() >= 1
				if hasValidStruct {
					r.ReadString()                // structReference levelName
					structRefPn := r.ReadString() // structReference pathName
					savedPayloadSize := r.ReadInt32()
					if savedPayloadSize > 0 && savedPayloadSize < 1000000 {
						payload := r.ReadBytesCopy(int(savedPayloadSize))
						typeDynamicPayloads[typeRefPn] = append(typeDynamicPayloads[typeRefPn], payload)
						if _, ok := typeDynamicRef[typeRefPn]; !ok {
							typeDynamicRef[typeRefPn] = structRefPn
						}
						beamLength = extractBeamLength(payload)
					} else if savedPayloadSize > 0 {
						r.Skip(int(savedPayloadSize))
					}
				}
			}

			// PlayerInfoHandle (if lightweightVersion >= 3)
			if lwVersion >= 3 {
				if d.ctx.ObjectVersion >= 57 {
					r.ReadUInt8() // serviceProvider
					r.ReadInt32() // playerInfoTableIndex
				} else {
					r.ReadUInt8() // serviceProvider
					r.ReadUInt8() // playerInfoTableIndex
				}
			}

			typeBeamLengths[typeRefPn] = append(typeBeamLengths[typeRefPn], beamLength)
			bpProxyRef := bpProxyLevel + ":" + bpProxyPath
			if bpProxyRef != ":" {
				typeBlueprintProxies[typeRefPn] = append(typeBlueprintProxies[typeRefPn], bpProxyRef)
			}

			validCount++
		}
		// Accumulate rather than overwrite: typeRefPn can legitimately repeat
		// across entries (e.g. after a mod re-registers a class), and
		// typeTransforms/typeBeamLengths/typeBlueprintProxies must all stay
		// index-aligned with each other and with the running typeCounts total.
		typeCounts[typeRefPn] += int(validCount)
		if len(transforms) > 0 {
			typeTransforms[typeRefPn] = append(typeTransforms[typeRefPn], transforms...)
		}
	}

	result := map[string]interface{}{
		"type":                 "BuildableSubsystemSpecialProperties",
		"typeCounts":           typeCounts,
		"typeTransforms":       typeTransforms,
		"typeBeamLengths":      typeBeamLengths,
		"typeDynamicPayloads":  typeDynamicPayloads,
		"typeDynamicRef":       typeDynamicRef,
		"typeBlueprintProxies": typeBlueprintProxies,
	}
	return result, nil
}

// extractBeamLength parses a FFGDynamicStruct payload (UE5+ tagged property list)
// and returns the BeamLength float value in cm, or 0 if not found.
func extractBeamLength(payload []byte) float64 {
	if len(payload) < 4 {
		return 0
	}
	r := NewBinaryReader(payload)
	for r.CanRead(4) {
		propName := r.ReadString()
		if propName == "None" || propName == "" {
			break
		}
		// FPropertyTypeName: list of nodes (FName + int32 innerCount), tree
		readPropTypeName(r)
		size := r.ReadInt32()
		flags := r.ReadUInt8()
		if flags&0x01 != 0 {
			r.ReadInt32() // ArrayIndex
		}
		if flags&0x02 != 0 {
			r.Skip(16) // PropertyGuid
		}
		// Read value based on property name
		if propName == "BeamLength" && size == 4 {
			return float64(r.ReadFloat32())
		}
		r.Skip(int(size))
	}
	return 0
}

// readPropTypeName reads a FPropertyTypeName tree from the reader.
func readPropTypeName(r *BinaryReader) {
	name := r.ReadString()
	if name == "None" || name == "" {
		return
	}
	innerCount := int(r.ReadInt32())
	for i := 0; i < innerCount; i++ {
		readPropTypeName(r)
	}
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
