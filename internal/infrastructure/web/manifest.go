package web

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"slices"
	"time"
)

// manifestFile is the PWA manifest inside the asset tree, manifestPath the URL the
// shell links it by.
const (
	manifestFile = "site.webmanifest"
	manifestPath = assetsPrefix + manifestFile
)

// assetTreeHandler serves the shell's asset tree. Everything in it is a plain file except
// the manifest, which is rendered: see handleManifest. The path is cleaned before the
// comparison, or a doubled slash before the filename (reachable as %2F, which no mux
// cleans) misses the renderer and the file server hands out the raw JSON.
func assetTreeHandler() http.Handler {
	files := http.StripPrefix(assetsPrefix, http.FileServerFS(FS()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Clean(r.URL.Path) == manifestPath {
			handleManifest(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// handleManifest answers with site.webmanifest, every icon src stamped. It is the one
// asset whose *contents* are asset URLs, and static JSON cannot carry the build, so
// served as a file an installed app keeps whatever icon bytes the browser cached under
// the bare path. ServeContent keeps the conditional-request handling the file server
// would have done (modtime is zero on the embedded copy, real under WEB_ASSETS_DIR).
func handleManifest(w http.ResponseWriter, r *http.Request) {
	raw, err := fs.ReadFile(FS(), manifestFile)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	stamped, err := stampManifestIcons(raw)
	if err != nil {
		http.Error(w, "manifest", http.StatusInternalServerError)
		return
	}
	var mod time.Time
	if info, err := fs.Stat(FS(), manifestFile); err == nil {
		mod = info.ModTime()
	}
	w.Header().Set("Content-Type", "application/manifest+json") // Go sniffs text/plain
	http.ServeContent(w, r, manifestFile, mod, bytes.NewReader(stamped))
}

// stampManifestIcons rewrites each icons[].src through AssetURL and splices the array
// back in, leaving every other byte where it was: re-encoding the whole object would
// sort the top-level keys.
func stampManifestIcons(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil { // the opening brace
		return nil, err
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		afterKey := int(dec.InputOffset())
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		end := int(dec.InputOffset())
		if key != "icons" {
			continue
		}
		stamped, err := stampIcons(value)
		if err != nil {
			return nil, err
		}
		open := bytes.IndexByte(raw[afterKey:end], '[')
		if open < 0 { // not an array: leave the document alone
			return raw, nil
		}
		return slices.Concat(raw[:afterKey+open], stamped, raw[end:]), nil
	}
	return raw, nil // no icons to stamp
}

// stampIcons rewrites the src of every entry in the manifest's icons array. Icons stay
// untyped so adding a field to the JSON needs no change here.
func stampIcons(raw json.RawMessage) ([]byte, error) {
	var icons []map[string]any
	if err := json.Unmarshal(raw, &icons); err != nil {
		return nil, err
	}
	for _, icon := range icons {
		if src, ok := icon["src"].(string); ok {
			icon["src"] = AssetURL(src)
		}
	}
	return json.Marshal(icons)
}
