package gifexplode

import (
	"appengine"
	"appengine/blobstore"
	"appengine/channel"
	"appengine/datastore"
	"appengine/delay"
	"appengine/urlfetch"

	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// Maximum single frame size
const maxFrameSize = 1 << 18 // 256 KB

func init() {
	http.HandleFunc("/", in)
	http.HandleFunc("/upload", upload)
	http.HandleFunc("/fetch", fetch)
}

var inTmpl = template.Must(template.New("in").Parse(`
<html><body>
<form action="{{.}}" method="POST" id="form" enctype="multipart/form-data">
  <label for="file">Select an animated GIF</label>
  <input type="file" name="file" id="file" accept="image/gif"></input>
  <script type="text/javascript">
  document.getElementById('file').onchange = function() {
    document.getElementById('form').submit();
  };
  </script>
</form>

<form action="/fetch" method="GET">
  <label for="url">Specify a URL</label>
  <input type="text" name="url"></input>
  <input type="submit"></input>
</form></body></html>
`))

var outTmpl = template.Must(template.New("out").Parse(`
<html><body>
<div id="loading">Loading...</div>
<script type="text/javascript" src="/_ah/channel/jsapi"></script>
<script type="text/javascript">tok = '{{.Tok}}';</script>
<script type="text/javascript" src="/client.js"></script>
</body></html>
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
	blobLater.Call(c, cid, f[0].BlobKey)
	outTmpl.Execute(w, struct {
		Tok string
	}{tok})
}

var blobLater = delay.Func("bloblater", func(c appengine.Context, cid string, bk appengine.BlobKey) error {
	if _, err := blobstore.Stat(c, bk); err == datastore.ErrNoSuchEntity {
		return nil
	}
	frames, err := framify(c, blobstore.NewReader(c, bk))
	if err != nil {
		return err
	}
	if err := send(c, cid, frames); err != nil {
		return err
	}
	return blobstore.Delete(c, bk)
})

type data struct {
	FrameNum    int    `json:"f"`
	TotalFrames int    `json:"tf"`
	FramePart   int    `json:"p"`
	TotalParts  int    `json:"tp"`
	Data        string `json:"d"`
}

func send(c appengine.Context, cid string, frames []string) error {
	totalFrames := len(frames)
	for frameNum, frameData := range frames {
		chunks := chunkify(frameData)
		totalParts := len(chunks)
		for partNum, partData := range chunks {
			b, err := json.Marshal(data{frameNum, totalFrames, partNum, totalParts, partData})
			if err != nil {
				return fmt.Errorf("channel json: %v", err)
			}
			if err := channel.Send(c, cid, string(b)); err != nil {
				return fmt.Errorf("channel send: %v", err)
			}
		}
	}
	return nil
}

// Max size of a data chunk to send to the client. Channel API messages have
// a 32K limit, and we send JSON packets with some overhead (~50B)
const chunkSize = 32*1024 - 50

// Chunkify a big string into smaller strings
func chunkify(s string) []string {
	chunks := make([]string, 0, len(s)/chunkSize+1)
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

func fetch(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "No URL specified", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	cid := appengine.RequestID(c)
	tok, err := channel.Create(c, cid)
	if err != nil {
		c.Errorf("create channel: %v", err)
		http.Error(w, "Error creating channel", http.StatusInternalServerError)
		return
	}
	fetchLater.Call(c, cid, url)
	outTmpl.Execute(w, struct {
		Tok string
	}{tok})
}

var fetchLater = delay.Func("fetchlater", func(c appengine.Context, cid, url string) error {
	resp, err := urlfetch.Client(c).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	frames, err := framify(c, resp.Body)
	if err != nil {
		return err
	}
	return send(c, cid, frames)
})

func framify(c appengine.Context, r io.Reader) ([]string, error) {
	g, err := gif.DecodeAll(r)
	if err != nil {
		c.Errorf("gif decode: %v", err)
		return nil, errors.New("Error decoding GIF")
	}
	frames := make([]string, 0, len(g.Image))
	for _, i := range g.Image {
		buf := bytes.NewBuffer(make([]byte, 0, maxFrameSize))
		// TODO: Upgrade to go1.2 and gif.Encode
		err = png.Encode(buf, layered{g.Image[0], i})
		if err != nil {
			c.Errorf("png encode: %v", err)
			return nil, errors.New("Error encoding frame")
		}
		frames = append(frames, fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes())))
	}
	return frames, nil
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
