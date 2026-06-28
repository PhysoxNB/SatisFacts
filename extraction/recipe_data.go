package extraction

import (
	"encoding/json"
	"strings"
)

// RecipeProduct represents a product output of a recipe.
type RecipeProduct struct {
	Item   string  `json:"item"`
	Amount float64 `json:"amount"`
}

// Recipe represents a crafting recipe.
type Recipe struct {
	Name        string          `json:"name"`
	ClassName   string          `json:"className"`
	Ingredients []RecipeProduct `json:"ingredients"`
	Products    []RecipeProduct `json:"products"`
	Duration    float64         `json:"duration"`
	ProducedIn  []string        `json:"producedIn"`
}

var recipeMap map[string]*Recipe

// LoadRecipes loads recipes from the embedded JSON file.
func LoadRecipes(path string) error {
	data, err := dataFilesFS.ReadFile("data/recipes.json")
	if err != nil {
		return err
	}
	recipeMap = make(map[string]*Recipe)
	if err := json.Unmarshal(data, &recipeMap); err != nil {
		return err
	}
	return nil
}

// GetPerCycleOutputs returns a recipe's per-cycle outputs, or nil if we don't
// know the recipe or it has no products.
func GetPerCycleOutputs(recipePath string) []RecipeProduct {
	if recipePath == "" || recipeMap == nil {
		return nil
	}
	recipe, ok := recipeMap[recipePath]
	if !ok || recipe.Products == nil {
		return nil
	}
	return recipe.Products
}

// Stack size -> the items that stack to that size.
var itemStackSizeMap = map[int][]string{
	1: {
		"Blade Runners", "Boom Box", "Chainsaw", "Gas Mask", "Hazmat Suit",
		"Hover Pack", "HUB Parts", "Jetpack", "Nobelisk Detonator", "Object Scanner",
		"Parachute", "Rebar Gun", "Rifle", "Xeno-Basher", "Xeno-Zapper", "Zipline",
	},
	50: {
		"Adaptive Control Unit", "Alien Carapace", "Alien DNA Capsule", "Alien Organs",
		"Alien Remains", "Assembly Director System", "Automated Wiring", "Bacon Agaric",
		"Computer", "Fused Modular Frame", "Heavy Modular Frame", "Magnetic Field Generator",
		"Medicinal Inhaler", "Mercer Sphere", "Miner", "Modular Engine", "Modular Frame",
		"Motor", "Nuclear Pasta", "Paleberry", "Plutonium Fuel Rod", "Power Slug",
		"Pressure Conversion Cube", "Quantum Computer", "Radio Control Unit", "Smart Plating",
		"Somersloop", "Supercomputer", "Thermal Propulsion Rocket", "Turbo Motor",
		"Uranium Fuel Rod", "Versatile Framework",
	},
	100: {
		"Alien Protein", "Aluminum Ingot", "Bauxite", "Beacon", "Beryl Nut",
		"Caterium Ingot", "Caterium Ore", "Coal", "Compacted Coal", "Cooling System",
		"Copper Ingot", "Copper Ore", "Crystal Oscillator", "Electromagnetic Control Rod",
		"Empty Canister", "Empty Fluid Tank", "Encased Industrial Beam", "Fabric",
		"Hard Drive", "Heat Sink", "High-Speed Connector", "Iron Ingot", "Iron Ore",
		"Limestone", "Packaged Alumina Solution", "Packaged Fuel", "Packaged Heavy Oil Residue",
		"Packaged Liquid Biofuel", "Packaged Nitric Acid", "Packaged Nitrogen Gas",
		"Packaged Oil", "Packaged Sulfuric Acid", "Packaged Turbofuel", "Packaged Water",
		"Plutonium Pellet", "Power Shard", "Raw Quartz", "Reinforced Iron Plate",
		"Rotor", "SAM", "Smokeless Powder", "Stator", "Steel Ingot", "Sulfur",
		"Superposition Oscillator", "Uranium",
	},
	200: {
		"Alclad Aluminum Sheet", "Aluminum Casing", "Battery", "Biomass", "Black Powder",
		"Cable", "Circuit Board", "Color Cartridge", "Copper Sheet", "Encased Plutonium Cell",
		"Encased Uranium Cell", "Iron Plate", "Iron Rod", "Mycelia", "Petroleum Coke",
		"Plastic", "Polymer Resin", "Quartz Crystal", "Rubber", "Silica", "Solid Biofuel",
		"Steel Beam", "Steel Pipe", "Time Crystal", "Uranium Pellet", "Vines", "Wood",
	},
	500: {
		"Aluminum Scrap", "Concrete", "Copper Powder", "FICSIT Coupon", "Flower Petals",
		"Leaves", "Non-fissile Uranium", "Plutonium Waste", "Quickwire", "Screw",
		"Uranium Waste", "Wire",
	},
}

// A few internal names that don't match their display name.
var itemAliases = map[string]string{
	"Cement": "Concrete",
}

// fluidNames is the list of fluid/gas item name substrings for stack size detection.
var fluidNames = []string{
	"Water", "Crude Oil", "Heavy Oil Residue", "Fuel", "Turbofuel",
	"Liquid Biofuel", "Nitrogen Gas", "Sulfuric Acid", "Nitric Acid",
	"Alumina Solution", "Oil", "Liquid", "Gas", "Nitric",
	"Crude", "Biofuel", "Residue",
}

// GetStackSize returns the stack size for an item by display name.
func GetStackSize(itemName string) int {
	displayName := itemName
	if alias, ok := itemAliases[itemName]; ok {
		displayName = alias
	}
	spacedName := camelToSpaced(displayName)
	for stackSize, items := range itemStackSizeMap {
		for _, item := range items {
			if item == displayName || item == spacedName {
				return stackSize
			}
		}
	}
	return 100
}

// GetBuildingOutputStackSize is the max a building output slot holds. Fluids and
// gases cap at 50m³ per slot; solids just use the normal stack size.
func GetBuildingOutputStackSize(itemName string) int {
	itemLower := strings.ToLower(itemName)
	for _, fluid := range fluidNames {
		if strings.Contains(itemLower, strings.ToLower(fluid)) {
			return 50
		}
	}
	return GetStackSize(itemName)
}

// camelToSpaced converts CamelCase to space-separated words.
func camelToSpaced(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune(' ')
		}
		result.WriteRune(r)
	}
	return result.String()
}
