package gifexplode

import (
	"appengine"
	"appengine/urlfetch"

	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"strings"
	"text/template"
)

const (
	// Maximum total image size
	maxImgSize = 1 << 22

	// Maximum single frame size
	maxFrameSize = 1 << 18
)

var tmpl = template.Must(template.New("tmpl").Parse(`
<html><body>
<ol>
  {{range .Frames}}
  <li><img src="{{.}}" /></li>
  {{end}}
</ol>
</body></html>
`))

func init() {
	http.HandleFunc("/upload", upload)
	http.HandleFunc("/fetch", fetch)
}

func upload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	mpf, _, err := r.FormFile("file")
	if err != nil {
		c.Errorf("formfile: %v", err)
		http.Error(w, "No file specified", http.StatusBadRequest)
		return
	}
	defer mpf.Close()
	write(c, w, mpf)
}

func write(c appengine.Context, w http.ResponseWriter, r io.Reader) {
	lim := &io.LimitedReader{R: r, N: maxImgSize}
	g, err := gif.DecodeAll(lim)
	if lim.N <= 0 {
		http.Error(w, "File too large, > 4MB", http.StatusBadRequest)
		return
	}
	if err != nil {
		c.Errorf("gif decode: %v", err)
		http.Error(w, "Error decoding GIF", http.StatusBadRequest)
		return
	}
	fs := make([]string, 0, len(g.Image))
	for _, i := range g.Image {
		buf := bytes.NewBuffer(make([]byte, 0, maxFrameSize))
		// TODO: Upgrade to go1.2 and gif.Encode
		err = png.Encode(buf, layered{g.Image[0], i})
		if err != nil {
			c.Errorf("png encode: %v", err)
			http.Error(w, "Error encoding", http.StatusInternalServerError)
			return
		}
		fs = append(fs, fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes())))
	}
	tmpl.Execute(w, struct {
		Frames []string
	}{fs})
}

func fetch(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "Must provide URL to fetch", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	c := appengine.NewContext(r)
	resp, err := urlfetch.Client(c).Get(url)
	if err != nil {
		c.Errorf("urlfetch: %v", err)
		http.Error(w, "Error fetching", http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()
	write(c, w, resp.Body)
}

type layered struct {
	back, front image.Image
}

func (l layered) At(x, y int) color.Color {
	f := l.front.At(x, y)
	if _, _, _, a := f.RGBA(); a == 0 {
		return l.back.At(x, y)
	}
	return f
}

func (l layered) ColorModel() color.Model {
	return l.back.ColorModel()
}

func (l layered) Bounds() image.Rectangle {
	return l.back.Bounds()
}
