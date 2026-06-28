package extraction

import (
	"strings"

	"satisfacts/parser"
)

// GameProgressionData is what we dig out about how far a save has progressed.
type GameProgressionData struct {
	CurrentPhase          string   `json:"currentPhase"`
	TargetPhase           string   `json:"targetPhase"`
	PurchasedSchematics   []string `json:"purchasedSchematics"`
	LastActiveSchematic   string   `json:"lastActiveSchematic"`
	UnlockedResearchTrees []string `json:"unlockedResearchTrees"`
	ResearchActivated     bool     `json:"researchActivated"`
	LastUsedHardDriveID   int      `json:"lastUsedHardDriveID"`
	SinkCoupons           int      `json:"sinkCoupons"`
	SinkTotalPoints       int      `json:"sinkTotalPoints"`

	// Custom map settings (1.2+, stored in BP_GameState)
	NodePuritySetting        string  `json:"nodePuritySetting,omitempty"`
	NodeRandomization        string  `json:"nodeRandomization,omitempty"`
	NodeRandomizationSeed    int32   `json:"nodeRandomizationSeed,omitempty"`
	PartsCostMultiplier      float32 `json:"partsCostMultiplier,omitempty"`
	SpacePartsCostMultiplier float32 `json:"spacePartsCostMultiplier,omitempty"`
	PowerConsumptionMultiplier float32 `json:"powerConsumptionMultiplier,omitempty"`
}

// ExtractGameProgression finds the game state object and reads progression off
// it and the managers it points to.
func ExtractGameProgression(retained []*parser.SaveObject, objectMap map[string]*parser.SaveObject) GameProgressionData {
	var gp GameProgressionData

	for _, obj := range retained {
		if obj.Header == nil {
			continue
		}
		typeLower := strings.ToLower(obj.Header.ClassName)
		if !strings.Contains(typeLower, "gamestate") {
			continue
		}
		if obj.Properties == nil {
			continue
		}

		// mGamePhaseManager
		if phaseMgrRef := getObjRefProp(obj, "mGamePhaseManager"); phaseMgrRef != "" {
			if phaseMgrObj, ok := objectMap[phaseMgrRef]; ok && phaseMgrObj.Properties != nil {
				gp.CurrentPhase = extractPhaseName(phaseMgrObj, "mCurrentGamePhase")
				gp.TargetPhase = extractPhaseName(phaseMgrObj, "mTargetGamePhase")
			}
		}

		// mSchematicManager
		if schematicMgrRef := getObjRefProp(obj, "mSchematicManager"); schematicMgrRef != "" {
			if schematicMgrObj, ok := objectMap[schematicMgrRef]; ok && schematicMgrObj.Properties != nil {
				gp.PurchasedSchematics = extractSchematicList(schematicMgrObj)
				gp.LastActiveSchematic = extractLastActiveSchematic(schematicMgrObj)
			}
		}

		// mResearchManager
		if researchMgrRef := getObjRefProp(obj, "mResearchManager"); researchMgrRef != "" {
			if researchMgrObj, ok := objectMap[researchMgrRef]; ok && researchMgrObj.Properties != nil {
				gp.UnlockedResearchTrees = extractResearchTrees(researchMgrObj)
				if v, ok := GetPropBool(researchMgrObj, "mIsActivated"); ok {
					gp.ResearchActivated = v
				}
				if v, ok := GetPropInt32(researchMgrObj, "mLastUsedHardDriveID"); ok {
					gp.LastUsedHardDriveID = int(v)
				}
			}
		}

		// mResourceSinkSubsystem
		if sinkRef := getObjRefProp(obj, "mResourceSinkSubsystem"); sinkRef != "" {
			if sinkObj, ok := objectMap[sinkRef]; ok && sinkObj.Properties != nil {
				if v, ok := GetPropInt32(sinkObj, "mNumResourceSinkCoupons"); ok {
					gp.SinkCoupons = int(v)
				}
				// mCurrentPointLevels is an int array; sum it for the running total.
				if pointLevels, ok := GetPropArray(sinkObj, "mCurrentPointLevels"); ok && pointLevels != nil {
					total := 0
					for _, item := range pointLevels {
						switch v := item.(type) {
						case int32:
							total += int(v)
						case float64:
							total += int(v)
						case int:
							total += v
						case int64:
							total += int(v)
						}
					}
					gp.SinkTotalPoints = total
				}
			}
		}

		// Custom map settings (directly on GameState)
		if v, ok := GetPropString(obj, "mNodePuritySettings"); ok {
			gp.NodePuritySetting = cleanEnumValue(v)
		}
		if v, ok := GetPropString(obj, "mNodeRandomization"); ok {
			gp.NodeRandomization = cleanEnumValue(v)
		}
		if v, ok := GetPropInt32(obj, "mNodeRandomizationSeed"); ok {
			gp.NodeRandomizationSeed = v
		}
		if v, ok := GetPropFloat32(obj, "mPartsCostMultiplier"); ok {
			gp.PartsCostMultiplier = v
		}
		if v, ok := GetPropFloat32(obj, "mSpacePartsCostMultiplier"); ok {
			gp.SpacePartsCostMultiplier = v
		}
		if v, ok := GetPropFloat32(obj, "mPowerConsumptionMultiplier"); ok {
			gp.PowerConsumptionMultiplier = v
		}
	}

	return gp
}

// cleanEnumValue strips the enum noise, e.g. "ENodePuritySettings::NPS_AllPure"
// becomes "AllPure".
func cleanEnumValue(v string) string {
	if idx := strings.Index(v, "::"); idx >= 0 {
		v = v[idx+2:]
	}
	// ...then the NPS_/NRM_ style prefixes.
	v = strings.TrimPrefix(v, "NPS_")
	v = strings.TrimPrefix(v, "NRM_")
	return v
}

func getObjRefProp(obj *parser.SaveObject, propName string) string {
	prop, ok := obj.Properties[propName]
	if !ok {
		return ""
	}
	// Usually an ObjectProperty (map[string]string of levelName/pathName).
	if refMap, ok := prop.Value.(map[string]string); ok {
		return refMap["pathName"]
	}
	if refMap, ok := prop.Value.(map[string]interface{}); ok {
		if pn, ok := refMap["pathName"].(string); ok {
			return pn
		}
	}
	return ""
}

func extractPhaseName(obj *parser.SaveObject, propName string) string {
	prop, ok := obj.Properties[propName]
	if !ok {
		return ""
	}
	var pathName string
	if refMap, ok := prop.Value.(map[string]string); ok {
		pathName = refMap["pathName"]
	} else if refMap, ok := prop.Value.(map[string]interface{}); ok {
		if pn, ok := refMap["pathName"].(string); ok {
			pathName = pn
		}
	}
	if pathName == "" {
		return ""
	}

	// e.g. .../GP_Project_Assembly_Phase_4.GP_Project_Assembly_Phase_4_C -> Project_Assembly_Phase_4
	phaseName := pathName
	if idx := strings.LastIndex(phaseName, "/"); idx >= 0 {
		phaseName = phaseName[idx+1:]
	}
	phaseName = strings.TrimSuffix(phaseName, "_C")
	if strings.Contains(phaseName, "GP_") {
		phaseName = strings.Replace(phaseName, "GP_", "", 1)
	}
	if strings.Contains(phaseName, ".") {
		phaseName = strings.Split(phaseName, ".")[0]
	}
	return phaseName
}

func extractSchematicList(obj *parser.SaveObject) []string {
	prop, ok := obj.Properties["mPurchasedSchematics"]
	if !ok {
		return nil
	}
	items := getArrayItems(prop)
	if items == nil {
		return nil
	}

	var schematics []string
	for _, item := range items {
		var pathName string
		switch v := item.(type) {
		case map[string]string:
			pathName = v["pathName"]
		case map[string]interface{}:
			if pn, ok := v["pathName"].(string); ok {
				pathName = pn
			}
		}

		if pathName == "" {
			continue
		}

		// e.g. Schematic_1-2.Schematic_1-2_C -> 1-2
		name := pathName
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		name = strings.TrimSuffix(name, "_C")
		if strings.Contains(name, ".") {
			name = strings.Split(name, ".")[0]
		}
		name = strings.TrimPrefix(name, "Schematic_")
		schematics = append(schematics, name)
	}
	return schematics
}

func extractLastActiveSchematic(obj *parser.SaveObject) string {
	prop, ok := obj.Properties["mLastActiveSchematic"]
	if !ok {
		return ""
	}
	var pathName string
	if refMap, ok := prop.Value.(map[string]string); ok {
		pathName = refMap["pathName"]
	} else if refMap, ok := prop.Value.(map[string]interface{}); ok {
		if pn, ok := refMap["pathName"].(string); ok {
			pathName = pn
		}
	}
	if pathName == "" {
		return ""
	}
	name := pathName
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.Replace(name, ".Schematic_", "", 1)
	name = strings.TrimSuffix(name, "_C")
	return name
}

func extractResearchTrees(obj *parser.SaveObject) []string {
	prop, ok := obj.Properties["mUnlockedResearchTrees"]
	if !ok {
		return nil
	}
	items := getArrayItems(prop)
	if items == nil {
		return nil
	}

	var trees []string
	for _, item := range items {
		var pathName string
		switch v := item.(type) {
		case map[string]string:
			pathName = v["pathName"]
		case map[string]interface{}:
			if pn, ok := v["pathName"].(string); ok {
				pathName = pn
			}
		}

		if pathName == "" {
			continue
		}

		// e.g. BPD_ResearchTree_Caterium.BPD_ResearchTree_Caterium_C -> Caterium
		name := pathName
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		name = strings.TrimSuffix(name, "_C")
		if strings.Contains(name, ".") {
			name = strings.Split(name, ".")[0]
		}
		name = strings.TrimPrefix(name, "BPD_ResearchTree_")
		trees = append(trees, name)
	}
	return trees
}

func getArrayItems(prop parser.Property) []interface{} {
	if prop.Type != "ArrayProperty" {
		return nil
	}
	switch v := prop.Value.(type) {
	case map[string]interface{}:
		if items, ok := v["items"].([]interface{}); ok {
			return items
		}
	case []interface{}:
		return v
	}
	return nil
}
