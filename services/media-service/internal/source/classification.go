package source

import "strings"

var genreAliases = map[string]string{
	"acoustic": "acoustic", "ambient": "ambient", "classical": "classical",
	"disco": "disco", "downtempo": "downtempo", "dub": "dub",
	"electro": "electronic", "electronic": "electronic", "electronica": "electronic",
	"experimental": "experimental", "folk": "folk", "funk": "funk",
	"hip hop": "hip-hop", "hip-hop": "hip-hop", "indie": "indie",
	"instrumental hip-hop": "hip-hop", "electro swing": "electronic",
	"industrial": "industrial", "jazz": "jazz", "metal": "metal",
	"noise": "noise", "piano": "piano", "pop": "pop", "punk": "punk",
	"rock": "rock", "synthwave": "synthwave", "trip hop": "trip-hop",
	"trip-hop": "trip-hop",
}

var moodAliases = map[string]string{
	"calm": "calm", "chill": "relaxed", "chilled": "relaxed",
	"dark": "dark", "dance": "energetic", "energetic": "energetic",
	"focus": "focused", "focused": "focused", "happy": "uplifting",
	"hopeful": "uplifting", "meditation": "calm", "nocturnal": "nocturnal",
	"relaxed": "relaxed", "sleep": "calm", "uplifting": "uplifting",
}

// ClassifyTags maps untrusted provider tags onto Infinitune's bounded taxonomy.
func ClassifyTags(tags []string) (genres, moods []string) {
	genreSet, moodSet := map[string]struct{}{}, map[string]struct{}{}
	for _, raw := range tags {
		for _, tag := range strings.FieldsFunc(raw, func(value rune) bool {
			return value == ';' || value == ',' || value == '|'
		}) {
			tag = strings.ToLower(strings.TrimSpace(tag))
			tag = strings.NewReplacer("_", " ", "/", " ").Replace(tag)
			if genre, ok := genreAliases[tag]; ok {
				genreSet[genre] = struct{}{}
			}
			if mood, ok := moodAliases[tag]; ok {
				moodSet[mood] = struct{}{}
			}
			for needle, mood := range moodAliases {
				if strings.Contains(tag, needle) {
					moodSet[mood] = struct{}{}
				}
			}
		}
	}
	for _, value := range sortedKeys(genreSet) {
		genres = append(genres, value)
	}
	for _, value := range sortedKeys(moodSet) {
		moods = append(moods, value)
	}
	return genres, moods
}

func sortedKeys(values map[string]struct{}) []string {
	order := []string{"acoustic", "ambient", "classical", "disco", "downtempo", "dub",
		"electronic", "experimental", "folk", "funk", "hip-hop", "indie", "industrial",
		"jazz", "metal", "noise", "piano", "pop", "punk", "rock", "synthwave", "trip-hop",
		"calm", "dark", "energetic", "focused", "nocturnal", "relaxed", "uplifting"}
	result := make([]string, 0, len(values))
	for _, key := range order {
		if _, ok := values[key]; ok {
			result = append(result, key)
		}
	}
	return result
}
