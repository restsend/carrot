package carrot

import (
	"net/http"
)

type PongoRender struct {
	as       *StaticAssets
	fileName string
	ctx      map[string]interface{}
}

// Render implements render.Render
func (r *PongoRender) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	t, err := r.as.sets.FromFile(r.fileName)
	if err != nil {
		return err
	}
	result, err := t.ExecuteBytes(r.ctx)
	if err != nil {
		return err
	}
	_, err = w.Write(result)
	return err
}

// WriteContentType implements render.Render
func (r *PongoRender) WriteContentType(w http.ResponseWriter) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = []string{"text/html; charset=utf-8"}
	}
}
