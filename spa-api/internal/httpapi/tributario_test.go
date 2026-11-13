package httpapi

import (
	"strings"
	"testing"
)

func TestReglaProPymeVigente(t *testing.T) {
	casos := map[int]string{2025: "0.125", 2026: "0.125", 2027: "0.125", 2028: "0.15"}
	for anio, tasa := range casos {
		r, ok := reglaProPymeVigente(anio)
		if !ok {
			t.Fatalf("falta regla para %d", anio)
		}
		if r.TasaIDPC.String() != tasa {
			t.Fatalf("%d: tasa %s, esperada %s", anio, r.TasaIDPC.String(), tasa)
		}
		if r.Fuente == "" || r.Version == "" {
			t.Fatalf("%d: regla sin trazabilidad", anio)
		}
	}
	if _, ok := reglaProPymeVigente(2029); ok {
		t.Fatal("se aceptó un año sin regla oficial versionada")
	}
}

func TestRespuestaLocalNoInventaSinCalculo(t *testing.T) {
	r := respuestaLocalAsistente("¿cuánto pagaré?", contextoAsistente{Anio: 2026})
	if !strings.Contains(r, "no existe una proyección") || !strings.Contains(r, "contador") {
		t.Fatalf("respuesta insegura: %s", r)
	}
}

func TestRespuestaLocalCitaVersion(t *testing.T) {
	c := contextoAsistente{Anio: 2026, CalculoDisponible: true, IDPCEstimado: "125000.0000", VersionReglas: "sii-circular-53-2025-ac2026-v1"}
	r := respuestaLocalAsistente("¿cuánto pagaré de impuesto?", c)
	if !strings.Contains(r, c.IDPCEstimado) || !strings.Contains(r, c.VersionReglas) {
		t.Fatalf("respuesta sin trazabilidad: %s", r)
	}
}
