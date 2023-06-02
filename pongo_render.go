package carrot

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
)

func RegisterCarrotFilters() {
	if !pongo2.FilterExists("markdown") {
		pongo2.RegisterFilter("markdown", markdownFilter)
	}
	if !pongo2.FilterExists("stringify") {
		pongo2.RegisterFilter("stringify", stringifyFilter)
	}
}

type PongoRender struct {
	sets     *pongo2.TemplateSet
	fileName string
	ctx      map[string]any
}

// Render implements render.Render
func (r *PongoRender) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	t, err := r.sets.FromFile(r.fileName)
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

func stringifyFilter(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	var data []byte
	var err error

	if param.IsInteger() {
		indent := strings.Repeat(" ", param.Integer())
		data, err = json.MarshalIndent(in.Interface(), "", indent)
	} else {
		data, err = json.Marshal(in.Interface())
	}
	if err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:stringify",
			OrigError: err,
		}
	}
	return pongo2.AsSafeValue(string(data)), nil
}

func markdownFilter(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return in, nil
	//return pongo2.AsValue(Markdown(in.String())), nil
}
