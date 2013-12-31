package gifexplode

import (
	"appengine"
	"appengine/blobstore"
	"appengine/channel"
	"appengine/datastore"
	"appengine/delay"

	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"text/template"
)

const (
	// Maximum single frame size
	maxFrameSize = 1 << 18 // 256 KB
)

var later = delay.Func("explode", func(c appengine.Context, cid string, bk appengine.BlobKey) error {
	if _, err := blobstore.Stat(c, bk); err == datastore.ErrNoSuchEntity {
		return nil
	}
	frames, err := frames(c, blobstore.NewReader(c, bk))
	if err != nil {
		return err
	}
	l := len(frames)
	type data struct {
		I, L int
		F    string
	}
	for i := 0; i < l; i++ {
		if err := channel.SendJSON(c, cid, data{i, l, frames[i]}); err != nil {
			return err
		}
	}
	return blobstore.Delete(c, bk)
})

func init() {
	http.HandleFunc("/", in)
	http.HandleFunc("/upload", upload)
}

var inTmpl = template.Must(template.New("in").Parse(`
<html><body>
<form action="{{.}}" method="POST" id="form" enctype="multipart/form-data">
<input type="file" name="file" id="file" accept="image/gif"></input>
<script type="text/javascript">
document.getElementById('file').onchange = function() {
  document.getElementById('form').submit();
};
</script></form></body></html>
`))

var outTmpl = template.Must(template.New("out").Parse(`
<html><body>
<script type="text/javascript" src="/_ah/channel/jsapi"></script>
<script type="text/javascript">
document.innerHTML = 'loading...';
var tok = '{{.Tok}}';
var s = new goog.appengine.Channel(tok).open();
s.onmessage = function(m) {
  var d = JSON.parse(m.data);
  for (var i = document.getElementsByTagName('img').length; i < d.L; i++) {
    document.body.appendChild(document.createElement('img'));
  }
  document.getElementsByTagName('img')[d.I].src = d.F;
};
</script></body></html>
`))

func in(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	url, err := blobstore.UploadURL(c, "/upload", nil)
	if err != nil {
		c.Errorf("uploadurl: %v", err)
		http.Error(w, "Error getting upload URL", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	inTmpl.Execute(w, url)
}

func upload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	blobs, _, err := blobstore.ParseUpload(r)
	if err != nil {
		c.Errorf("parseupload: %v", err)
		http.Error(w, "Error parsing upload", http.StatusInternalServerError)
		return
	}
	f := blobs["file"]
	if len(f) == 0 {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}

	cid := appengine.RequestID(c)
	tok, err := channel.Create(c, cid)
	if err != nil {
		c.Errorf("create channel: %v", err)
		http.Error(w, "Error creating channel", http.StatusInternalServerError)
		return
	}
	later.Call(c, cid, f[0].BlobKey)
	outTmpl.Execute(w, struct {
		Tok string
	}{tok})
}

func frames(c appengine.Context, r io.Reader) ([]string, error) {
	g, err := gif.DecodeAll(r)
	if err != nil {
		c.Errorf("gif decode: %v", err)
		return nil, errors.New("Error decoding GIF")
	}
	fs := make([]string, 0, len(g.Image))
	for _, i := range g.Image {
		buf := bytes.NewBuffer(make([]byte, 0, maxFrameSize))
		// TODO: Upgrade to go1.2 and gif.Encode
		err = png.Encode(buf, layered{g.Image[0], i})
		if err != nil {
			c.Errorf("png encode: %v", err)
			return nil, errors.New("Error encoding frame")
		}
		fs = append(fs, fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes())))
	}
	return fs, nil
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
