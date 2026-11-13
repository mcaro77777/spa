package httpapi

import "testing"

func TestNormalizarCabeceraBancaria(t *testing.T) {
	casos := map[string]string{"Fecha Movimiento": "fecha", "DESCRIPCIÓN": "descripcion", "Glosa": "descripcion", "Amount": "monto", "Saldo Informado": "saldo"}
	for entrada, esperado := range casos {
		if actual := normalizarCabecera(entrada); actual != esperado {
			t.Fatalf("%q: esperado %q, recibido %q", entrada, esperado, actual)
		}
	}
}

func TestDecimalBancario(t *testing.T) {
	casos := map[string]string{"$ 1.234,50": "1234.5", "-45.000": "-45000", "1.234.567": "1234567", "1234.50": "1234.5", "-10,25": "-10.25"}
	for entrada, esperado := range casos {
		actual, err := decimalBancario(entrada)
		if err != nil {
			t.Fatalf("%q: %v", entrada, err)
		}
		if actual.String() != esperado {
			t.Fatalf("%q: esperado %s, recibido %s", entrada, esperado, actual.String())
		}
	}
}

func TestParsearFechaBancaria(t *testing.T) {
	for _, v := range []string{"2026-11-12", "12-11-2026", "12/11/2026"} {
		if _, err := parsearFechaBancaria(v); err != nil {
			t.Fatalf("%q: %v", v, err)
		}
	}
}
