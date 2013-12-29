package gifexplode

import (
	"appengine"

	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"image/gif"
	"image/png"
	"net/http"
)

const (
	bufSize = 2 << 17
)

var tmpl = template.Must(template.New("tmpl").Parse(`
<html><body>
{{range .Frames}}
<img src="{{.}}" /><br />
{{end}}
</body></html>
`))

func init() {
	http.HandleFunc("/upload", upload)
}

func upload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	f, _, err := r.FormFile("file")
	if err != nil {
		c.Errorf("formfile: %v", err)
		http.Error(w, "No file specified", http.StatusBadRequest)
		return
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		c.Errorf("gif decode: %v", err)
		http.Error(w, "Error decoding GIF", http.StatusBadRequest)
		return
	}
	fs := []string{}
	for _, i := range g.Image {
		buf := bytes.NewBuffer(make([]byte, 0, bufSize))
		// TODO: Upgrade to go1.2 and gif.Encode
		err = png.Encode(buf, i)
		if err != nil {
			c.Errorf("png encode: %v", err)
			http.Error(w, "Error encoding", http.StatusInternalServerError)
			return
		}
		dataURI := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes()))
		fmt.Println(dataURI)
		fs = append(fs, dataURI)
	}
	tmpl.Execute(w, struct {
		Frames []string
	}{fs})
}
