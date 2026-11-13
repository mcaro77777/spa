package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type problema struct {
	Type string `json:"type,omitempty"`; Title string `json:"title"`; Status int `json:"status"`
	Detail string `json:"detail,omitempty"`; Instance string `json:"instance,omitempty"`; Code string `json:"code,omitempty"`
	Errors map[string][]string `json:"errors,omitempty"`
}

func responderJSON(w http.ResponseWriter, estado int, dato any) {
	if w.Header().Get("Content-Type") == "" { w.Header().Set("Content-Type", "application/json; charset=utf-8") }
	w.WriteHeader(estado)
	if dato != nil { _ = json.NewEncoder(w).Encode(dato) }
}

func responderProblema(w http.ResponseWriter, r *http.Request, estado int, codigo, titulo, detalle string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	responderJSON(w, estado, problema{Type: "https://mispa.cl/problemas/"+codigo, Title: titulo, Status: estado, Detail: detalle, Instance: r.URL.Path, Code: codigo})
}

func decodificarJSON(w http.ResponseWriter, r *http.Request, destino any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body); d.DisallowUnknownFields()
	if err := d.Decode(destino); err != nil { return err }
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) { return errors.New("el cuerpo debe contener un unico objeto JSON") }
	return nil
}

