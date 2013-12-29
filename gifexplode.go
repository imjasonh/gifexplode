package gifexplode

import (
	"appengine"
	
	"image/gif"
	"net/http"
)

func init() {
	http.HandleFunc("/upload", upload)
}

func upload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	f, _, err := r.FormFile("file")
	if err != nil {
		c.Errorf("formfile: %v", err)
		http.Error(w, "", http.StatusBadRequest)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		c.Errorf("gif decode: %v", err)
		http.Error(w, "", http.StatusBadRequest)
	}
	fs := []string{}
	for _, i := range g.Image {
		f := "data:image/gif;base64,"
		fs = append(fs, f)
	}
}
