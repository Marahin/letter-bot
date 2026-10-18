package region

// Region groups the respawns of a guild in the summary. Key is stored in
// web_spot.region and names the Discord channel; Name is a proper noun, shown
// as is in every language.
type Region struct {
	Key  string
	Name string
}
