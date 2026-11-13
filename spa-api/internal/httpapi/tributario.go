package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

const fuenteCircular53 = "https://www.sii.cl/normativa_legislacion/circulares/2025/circu53.pdf"

type reglaProPyme struct {
	AnioComercial                      int
	TasaIDPC, TasaPPMBaja, TasaPPMAlta decimal.Decimal
	Fuente, Version                    string
}

func reglaProPymeVigente(anio int) (reglaProPyme, bool) {
	tasaIDPC := map[int]string{2025: "0.125", 2026: "0.125", 2027: "0.125", 2028: "0.15"}
	v, ok := tasaIDPC[anio]
	if !ok {
		return reglaProPyme{}, false
	}
	baja, alta := "0.0025", "0.005"
	if anio <= 2027 {
		baja, alta = "0.00125", "0.0025"
	}
	return reglaProPyme{anio, decimal.RequireFromString(v), decimal.RequireFromString(baja), decimal.RequireFromString(alta), fuenteCircular53, fmt.Sprintf("sii-circular-53-2025-ac%d-v1", anio)}, true
}

type calculoTributarioEntrada struct {
	AnioComercial               int  `json:"anioComercial"`
	IngresosPreviosSobre50000UF bool `json:"ingresosPreviosSobre50000UF"`
}

func (s *Servidor) reglasTributarias(w http.ResponseWriter, r *http.Request) {
	anio := enteroQuery(r.URL.Query().Get("year"), time.Now().Year(), 2000, 2200)
	regla, ok := reglaProPymeVigente(anio)
	if !ok {
		responderProblema(w, r, 404, "regla_no_disponible", "Regla tributaria no disponible", "No existe una regla oficial revisada para el año solicitado.")
		return
	}
	responderJSON(w, 200, map[string]any{"anioComercial": anio, "anioTributario": anio + 1, "regimen": "pro_pyme_general_14d3", "tasaIDPC": regla.TasaIDPC.String(), "tasaPPMHasta50000UF": regla.TasaPPMBaja.String(), "tasaPPMSobre50000UF": regla.TasaPPMAlta.String(), "version": regla.Version, "fuenteOficial": regla.Fuente, "fechaConsulta": "2026-11-12", "estado": "vigente_revisar_profesional"})
}

func (s *Servidor) calcularPrimeraCategoria(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	var entrada calculoTributarioEntrada
	if err := decodificarJSON(w, r, &entrada); err != nil {
		responderProblema(w, r, 422, "calculo_invalido", "Solicitud inválida", err.Error())
		return
	}
	regla, ok := reglaProPymeVigente(entrada.AnioComercial)
	if !ok {
		responderProblema(w, r, 422, "regla_no_disponible", "Regla tributaria no disponible", "Solo se calculan períodos con reglas oficiales versionadas 2025–2028.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	defer tx.Rollback()
	var regimenID, regimenCodigo string
	if err = tx.QueryRowContext(r.Context(), `SELECT rt.regimen_tributario_id,rt.codigo FROM spa.empresas e JOIN spa.regimenes_tributarios rt ON rt.regimen_tributario_id=e.regimen_tributario_id WHERE e.empresa_id=$1 AND e.eliminado_en IS NULL`, u.EmpresaID).Scan(&regimenID, &regimenCodigo); err != nil {
		s.errorInterno(w, r, err)
		return
	}
	if regimenCodigo != "pro_pyme_general_14d3" {
		responderProblema(w, r, 422, "regimen_no_soportado", "Régimen no soportado", "Esta versión calcula exclusivamente Pro Pyme General 14 D N°3.")
		return
	}
	anioID, err := asegurarReglaTributaria(r.Context(), tx, regimenID, regla)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	inicio := time.Date(entrada.AnioComercial, 1, 1, 0, 0, 0, 0, time.UTC)
	fin := inicio.AddDate(1, 0, 0)
	var ingresos, gastos string
	var pendientes int
	err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(sum(CASE WHEN monto>0 AND clasificacion='cobro_cliente' THEN monto ELSE 0 END),0)::text,COALESCE(sum(CASE WHEN monto<0 AND clasificacion IN ('pago_proveedor','gasto_empresarial','comision_bancaria') THEN abs(monto) ELSE 0 END),0)::text,count(*) FILTER(WHERE clasificacion='pendiente_revision') FROM spa.movimientos_bancarios WHERE empresa_id=$1 AND fecha_movimiento >= $2 AND fecha_movimiento < $3 AND eliminado_en IS NULL`, u.EmpresaID, inicio, fin).Scan(&ingresos, &gastos, &pendientes)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	ingresoD, _ := decimal.NewFromString(ingresos)
	gastoD, _ := decimal.NewFromString(gastos)
	base := ingresoD.Sub(gastoD)
	if base.IsNegative() {
		base = decimal.Zero
	}
	idpc := base.Mul(regla.TasaIDPC).Round(0)
	tasaPPM := regla.TasaPPMBaja
	if entrada.IngresosPreviosSobre50000UF {
		tasaPPM = regla.TasaPPMAlta
	}
	ppm := ingresoD.Mul(tasaPPM).Round(0)
	advertencias := []string{"Estimación de apoyo basada únicamente en movimientos bancarios clasificados.", "No incorpora correcciones, créditos, pérdidas tributarias, activos ni ajustes que requieran antecedentes adicionales.", "RAI y SAC requieren saldos de apertura y revisión profesional antes de confirmarse."}
	faltantes := []string{}
	estado := "completado"
	if pendientes > 0 {
		faltantes = append(faltantes, fmt.Sprintf("%d movimientos bancarios pendientes de clasificación", pendientes))
		estado = "pendiente_verificar"
	}
	if ingresoD.IsZero() {
		faltantes = append(faltantes, "No existen cobros de clientes clasificados en el período")
		estado = "pendiente_verificar"
	}
	calculoID := nuevoUUID()
	hash := sha256.Sum256([]byte(u.EmpresaID + "|" + strconv.Itoa(entrada.AnioComercial) + "|" + ingresoD.String() + "|" + gastoD.String() + "|" + regla.Version))
	_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.calculos_tributarios(calculo_tributario_id,empresa_id,anio_tributario_id,regimen_tributario_id,tipo_calculo,periodo_desde,periodo_hasta,version_motor,version_reglas,estado,resultado_principal,supuestos,advertencias,datos_faltantes,hash_entradas,creado_por,fecha_finalizacion) VALUES($1,$2,$3,$4,'primera_categoria',$5,$6,'mispa-tributario-1.0',$7,$8,$9,$10::jsonb,$11::jsonb,$12::jsonb,$13,$14,CURRENT_TIMESTAMP)`, calculoID, u.EmpresaID, anioID, regimenID, inicio, fin.AddDate(0, 0, -1), regla.Version, estado, idpc.StringFixed(4), string(jsonBytes([]string{fmt.Sprintf("PPM usa tramo sobre 50.000 UF: %t", entrada.IngresosPreviosSobre50000UF)})), string(jsonBytes(advertencias)), string(jsonBytes(faltantes)), hex.EncodeToString(hash[:]), u.UsuarioID)
	if err == nil {
		for i, v := range []struct{ codigo, nombre, valor, formula, estado string }{{"ingresos_percibidos", "Ingresos percibidos", ingresoD.StringFixed(4), "Suma de cobros de clientes clasificados", "estimado"}, {"egresos_pagados", "Egresos pagados", gastoD.StringFixed(4), "Pagos a proveedores, gastos empresariales y comisiones clasificados", "estimado"}, {"base_imponible_estimada", "Base imponible estimada", base.StringFixed(4), "máximo(ingresos percibidos - egresos pagados, 0)", "incompleto"}, {"idpc_estimado", "IDPC estimado", idpc.StringFixed(4), "base imponible × tasa IDPC oficial", "estimado"}, {"ppm_estimado", "PPM estimado", ppm.StringFixed(4), "ingresos percibidos × tasa PPM seleccionada", "estimado"}} {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.resultados_calculo_tributario(empresa_id,calculo_tributario_id,codigo,nombre,valor_decimal,estado,formula_aplicada,orden) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, u.EmpresaID, calculoID, v.codigo, v.nombre, v.valor, v.estado, v.formula, i+1)
			if err != nil {
				break
			}
		}
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.registros_tributarios(empresa_id,anio_comercial,tipo_registro,saldo_inicial,saldo_final,estado,version_reglas) VALUES($1,$2,'RAI',0,0,'incompleto',$3),($1,$2,'SAC',0,0,'incompleto',$3) ON CONFLICT (empresa_id,anio_comercial,tipo_registro) DO UPDATE SET estado='incompleto',version_reglas=EXCLUDED.version_reglas`, u.EmpresaID, entrada.AnioComercial, regla.Version)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	s.responderCalculoTributario(w, r, calculoID)
}

func asegurarReglaTributaria(ctx context.Context, tx *sql.Tx, regimenID string, r reglaProPyme) (string, error) {
	var anioID string
	err := tx.QueryRowContext(ctx, `INSERT INTO spa.anios_tributarios(anio_comercial,anio_tributario,fecha_inicio,fecha_fin,estado) VALUES($1::smallint,$2::smallint,make_date($1::int,1,1),make_date($1::int,12,31),'vigente') ON CONFLICT (anio_comercial,anio_tributario) DO UPDATE SET estado='vigente' RETURNING anio_tributario_id`, r.AnioComercial, r.AnioComercial+1).Scan(&anioID)
	if err != nil {
		return "", err
	}
	reglas := []struct{ codigo, nombre, valor string }{{"idpc_pro_pyme_general", "Tasa IDPC Pro Pyme General", r.TasaIDPC.String()}, {"ppm_pro_pyme_hasta_50000_uf", "Tasa PPM hasta 50.000 UF", r.TasaPPMBaja.String()}, {"ppm_pro_pyme_sobre_50000_uf", "Tasa PPM sobre 50.000 UF", r.TasaPPMAlta.String()}}
	for _, v := range reglas {
		_, err = tx.ExecContext(ctx, `INSERT INTO spa.reglas_tributarias(regimen_tributario_id,anio_tributario_id,codigo,nombre,tipo,valor_decimal,vigente_desde,vigente_hasta,fuente_oficial,fecha_consulta_fuente,version,estado) VALUES($1,$2,$3,$4,'tasa',$5,make_date($6,1,1),make_date($6,12,31),$7,DATE '2026-11-12',1,'vigente') ON CONFLICT (codigo,anio_tributario_id,regimen_tributario_id,version) DO UPDATE SET valor_decimal=EXCLUDED.valor_decimal,fuente_oficial=EXCLUDED.fuente_oficial,fecha_consulta_fuente=EXCLUDED.fecha_consulta_fuente,estado='vigente'`, regimenID, anioID, v.codigo, v.nombre, v.valor, r.AnioComercial, r.Fuente)
		if err != nil {
			return "", err
		}
	}
	return anioID, nil
}

func (s *Servidor) listarCalculosTributarios(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	rows, err := s.db.QueryContext(r.Context(), `SELECT calculo_tributario_id,EXTRACT(YEAR FROM periodo_desde)::int,estado,resultado_principal::text,version_reglas,advertencias,datos_faltantes,fecha_creacion FROM spa.calculos_tributarios WHERE empresa_id=$1 AND tipo_calculo='primera_categoria' ORDER BY fecha_creacion DESC LIMIT 20`, u.EmpresaID)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, estado, resultado, version string
		var anio int
		var advertencias, faltantes []byte
		var fecha time.Time
		if rows.Scan(&id, &anio, &estado, &resultado, &version, &advertencias, &faltantes, &fecha) == nil {
			items = append(items, map[string]any{"id": id, "anioComercial": anio, "estado": estado, "resultadoPrincipal": resultado, "versionReglas": version, "advertencias": jsonCrudo(advertencias), "datosFaltantes": jsonCrudo(faltantes), "fechaCreacion": fecha})
		}
	}
	responderJSON(w, 200, items)
}

func (s *Servidor) responderCalculoTributario(w http.ResponseWriter, r *http.Request, id string) {
	u := sesionActual(r)
	var anio int
	var estado, resultado, version string
	var advertencias, faltantes []byte
	var fecha time.Time
	err := s.db.QueryRowContext(r.Context(), `SELECT EXTRACT(YEAR FROM periodo_desde)::int,estado,resultado_principal::text,version_reglas,advertencias,datos_faltantes,fecha_creacion FROM spa.calculos_tributarios WHERE empresa_id=$1 AND calculo_tributario_id=$2`, u.EmpresaID, id).Scan(&anio, &estado, &resultado, &version, &advertencias, &faltantes, &fecha)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			responderProblema(w, r, 404, "calculo_no_encontrado", "Cálculo no encontrado", "")
			return
		}
		s.errorInterno(w, r, err)
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT codigo,nombre,valor_decimal::text,estado,formula_aplicada FROM spa.resultados_calculo_tributario WHERE empresa_id=$1 AND calculo_tributario_id=$2 ORDER BY orden`, u.EmpresaID, id)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	defer rows.Close()
	valores := []map[string]any{}
	for rows.Next() {
		var codigo, nombre, valor, estadoR, formula string
		if rows.Scan(&codigo, &nombre, &valor, &estadoR, &formula) == nil {
			valores = append(valores, map[string]any{"codigo": codigo, "nombre": nombre, "valor": valor, "estado": estadoR, "formula": formula})
		}
	}
	responderJSON(w, 201, map[string]any{"id": id, "anioComercial": anio, "estado": estado, "resultadoPrincipal": resultado, "versionReglas": version, "advertencias": jsonCrudo(advertencias), "datosFaltantes": jsonCrudo(faltantes), "fechaCreacion": fecha, "resultados": valores, "fuenteOficial": fuenteCircular53})
}

func jsonCrudo(v []byte) any {
	if len(v) == 0 {
		return []any{}
	}
	var x any
	if json.Unmarshal(v, &x) != nil {
		return []any{}
	}
	return x
}
