// Package regions is the fixed catalog of the regions that group the respawns
// of the summary. A respawn without a region, or with a key the catalog does not
// know, belongs to Other.
package regions

import "spot-assistant/internal/core/dto/region"

// CategoryName is the Discord category that holds the region channels.
const CategoryName = "Letter respawns"

// ChannelPrefix keeps the region channels apart from the guild's own channels.
const ChannelPrefix = "letter-"

// Other holds the respawns without a known region. It is always last.
var Other = region.Region{Key: "other", Name: "Other"}

// Catalog is in channel order. Never change a key: it is stored in web_spot.region
// and in summary_region_channels.
var Catalog = []region.Region{
	{Key: "thais-carlin", Name: "Thais, Carlin & Cults"},
	{Key: "venore", Name: "Venore"},
	{Key: "ab-dendriel", Name: "Ab'Dendriel"},
	{Key: "kazordoon", Name: "Kazordoon & Warzones"},
	{Key: "edron", Name: "Edron"},
	{Key: "grimvale", Name: "Grimvale & Werecreatures"},
	{Key: "darama", Name: "Darama & Krailos"},
	{Key: "port-hope", Name: "Port Hope & Tiquanda"},
	{Key: "liberty-bay", Name: "Liberty Bay & Goroma"},
	{Key: "ice-islands", Name: "Ice Islands"},
	{Key: "yalahar", Name: "Yalahar"},
	{Key: "zao", Name: "Zao & Farmine"},
	{Key: "feyrist", Name: "Feyrist"},
	{Key: "roshamuul", Name: "Roshamuul"},
	{Key: "oramond", Name: "Oramond"},
	{Key: "issavi", Name: "Issavi & Kilmaresh"},
	{Key: "ingol", Name: "Ingol"},
	{Key: "buried-cathedral", Name: "Buried Cathedral"},
	{Key: "hive", Name: "The Hive"},
	{Key: "demon-quests", Name: "POI, Inquisition & Ferumbras"},
	{Key: "endgame-quests", Name: "Endgame Quests"},
}

var byKey = func() map[string]region.Region {
	m := make(map[string]region.Region, len(Catalog)+1)
	for _, r := range Catalog {
		m[r.Key] = r
	}
	m[Other.Key] = Other
	return m
}()

// Lookup finds a catalog region or Other by its key.
func Lookup(key string) (region.Region, bool) {
	r, ok := byKey[key]
	return r, ok
}

// Of returns the region of a respawn: Other for "" or an unknown key.
func Of(key string) region.Region {
	if r, ok := byKey[key]; ok {
		return r
	}
	return Other
}

// Valid tells a key a respawn may store. Other is stored as "", not as its key.
func Valid(key string) bool {
	_, ok := byKey[key]
	return ok && key != Other.Key
}

// Index is the position of the region in the channel order, Other last.
func Index(key string) int {
	for i, r := range Catalog {
		if r.Key == key {
			return i
		}
	}
	return len(Catalog)
}

// ChannelName is the Discord channel of the region, e.g. "letter-port-hope".
func ChannelName(r region.Region) string {
	return ChannelPrefix + r.Key
}
