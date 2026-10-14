package web

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
)

// assetVersions maps each file under static/ to a short hash of its contents,
// taken once from the embedded copy. A page links to /static/style.css?v=<hash>
// rather than to the bare name, so a cache in front of the app (Cloudflare, or
// the browser itself) can never pair a new page with an old stylesheet: a
// changed file is a changed address — see implementation.md, "Stack".
var assetVersions = func() map[string]string {
	v := map[string]string{}
	fs.WalkDir(staticFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := staticFS.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		v[path[len("static/"):]] = hex.EncodeToString(sum[:])[:12]
		return nil
	})
	return v
}()

// asset is the address a template links a static file by. A name that is not
// embedded panics at render, the same way a misspelt template would, rather
// than linking a file that does not exist.
func asset(name string) string {
	v, ok := assetVersions[name]
	if !ok {
		panic("web: no static file " + name)
	}
	return "/static/" + name + "?v=" + v
}
