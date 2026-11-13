package httpapi

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var clasificacionesBancarias = []string{
	"cobro_cliente", "pago_proveedor", "gasto_empresarial", "transferencia_propietario",
	"posible_distribucion", "sueldo_empresarial", "prestamo", "devolucion", "gasto_personal",
	"transferencia_cuentas_propias", "impuesto", "comision_bancaria", "pendiente_revision",
}

type cuentaBancariaEntrada struct {
	Banco             string `json:"banco"`
	TipoCuenta        string `json:"tipoCuenta"`
	NumeroEnmascarado string `json:"numeroEnmascarado"`
	Moneda            string `json:"moneda"`
}

func (s *Servidor) cuentasBancarias(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	if r.Method == http.MethodGet {
		rows, err := s.db.QueryContext(r.Context(), `SELECT cuenta_bancaria_id,banco,tipo_cuenta,numero_enmascarado,moneda,activa FROM spa.cuentas_bancarias WHERE empresa_id=$1 AND eliminado_en IS NULL ORDER BY activa DESC,banco,numero_enmascarado`, u.EmpresaID)
		if err != nil {
			s.errorInterno(w, r, err)
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, banco, tipo, numero, moneda string
			var activa bool
			if err := rows.Scan(&id, &banco, &tipo, &numero, &moneda, &activa); err != nil {
				s.errorInterno(w, r, err)
				return
			}
			items = append(items, map[string]any{"id": id, "banco": banco, "tipoCuenta": tipo, "numeroEnmascarado": numero, "moneda": moneda, "activa": activa})
		}
		responderJSON(w, http.StatusOK, items)
		return
	}
	var entrada cuentaBancariaEntrada
	if err := decodificarJSON(w, r, &entrada); err != nil {
		responderProblema(w, r, 422, "cuenta_invalida", "Cuenta bancaria inválida", err.Error())
		return
	}
	entrada.Banco = strings.TrimSpace(entrada.Banco)
	entrada.TipoCuenta = strings.ToLower(strings.TrimSpace(entrada.TipoCuenta))
	entrada.NumeroEnmascarado = strings.TrimSpace(entrada.NumeroEnmascarado)
	entrada.Moneda = strings.ToUpper(strings.TrimSpace(entrada.Moneda))
	if entrada.Moneda == "" {
		entrada.Moneda = "CLP"
	}
	if entrada.Banco == "" || entrada.NumeroEnmascarado == "" || !enLista(entrada.TipoCuenta, "corriente", "vista", "ahorro", "credito", "otra") || len(entrada.Moneda) != 3 {
		responderProblema(w, r, 422, "cuenta_invalida", "Cuenta bancaria inválida", "Completa banco, tipo, número enmascarado y moneda.")
		return
	}
	id := nuevoUUID()
	_, err := s.db.ExecContext(r.Context(), `INSERT INTO spa.cuentas_bancarias(cuenta_bancaria_id,empresa_id,banco,tipo_cuenta,numero_enmascarado,moneda) VALUES($1,$2,$3,$4,$5,$6)`, id, u.EmpresaID, entrada.Banco, entrada.TipoCuenta, entrada.NumeroEnmascarado, entrada.Moneda)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	responderJSON(w, http.StatusCreated, map[string]any{"id": id, "banco": entrada.Banco, "tipoCuenta": entrada.TipoCuenta, "numeroEnmascarado": entrada.NumeroEnmascarado, "moneda": entrada.Moneda, "activa": true})
}

func (s *Servidor) importarCartola(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.cfg.MaxArchivoBytes))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		responderProblema(w, r, 413, "archivo_demasiado_grande", "Archivo no aceptado", "El archivo supera el límite permitido.")
		return
	}
	cuentaID := strings.TrimSpace(r.FormValue("cuentaId"))
	var existe bool
	if err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM spa.cuentas_bancarias WHERE empresa_id=$1 AND cuenta_bancaria_id=$2 AND activa AND eliminado_en IS NULL)`, u.EmpresaID, cuentaID).Scan(&existe); err != nil || !existe {
		responderProblema(w, r, 422, "cuenta_invalida", "Cuenta bancaria inválida", "Selecciona una cuenta activa de la empresa.")
		return
	}
	f, cab, err := r.FormFile("archivo")
	if err != nil {
		responderProblema(w, r, 422, "archivo_requerido", "Archivo requerido", "Adjunta una cartola CSV.")
		return
	}
	defer f.Close()
	if !strings.HasSuffix(strings.ToLower(cab.Filename), ".csv") {
		responderProblema(w, r, 422, "formato_no_admitido", "Formato no admitido", "La cartola debe ser CSV.")
		return
	}
	ruta, nombre, hash, tamano, err := s.guardarArchivo(f, cab)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	archivoID, trabajoID := nuevoUUID(), nuevoUUID()
	clave := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if clave == "" {
		clave = "cartola-" + cuentaID + "-" + hash
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		_ = os.Remove(ruta)
		s.errorInterno(w, r, err)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.archivos_importados(archivo_importado_id,empresa_id,nombre_original,nombre_almacenado,ruta_almacenamiento,tipo_mime,extension,tamano_bytes,hash_sha256,origen,subido_por) VALUES($1,$2,$3,$4,$5,'text/csv','.csv',$6,$7,'banco',$8)`, archivoID, u.EmpresaID, cab.Filename, nombre, ruta, tamano, hash, u.UsuarioID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.trabajos_importacion(trabajo_importacion_id,empresa_id,archivo_importado_id,tipo,estado,clave_idempotencia,configuracion,iniciado_por,fecha_inicio) VALUES($1,$2,$3,'cartola_bancaria','procesando',$4,jsonb_build_object('cuentaId',$5::text),$6,CURRENT_TIMESTAMP)`, trabajoID, u.EmpresaID, archivoID, clave, cuentaID, u.UsuarioID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		_ = os.Remove(ruta)
		if esDuplicado(err) {
			responderProblema(w, r, 409, "importacion_duplicada", "Cartola ya importada", "El archivo o la clave de idempotencia ya fue procesado.")
			return
		}
		s.errorInterno(w, r, err)
		return
	}
	res := s.procesarCartolaCSV(r.Context(), u, ruta, trabajoID, cuentaID)
	estado := "completado"
	if res.Rechazadas > 0 {
		estado = "completado_con_errores"
	}
	if res.Total > 0 && res.Total == res.Rechazadas {
		estado = "fallido"
	}
	_, _ = s.db.ExecContext(r.Context(), `UPDATE spa.trabajos_importacion SET estado=$1,total_filas=$2,filas_creadas=$3,filas_duplicadas=$4,filas_rechazadas=$5,fecha_fin=CURRENT_TIMESTAMP WHERE empresa_id=$6 AND trabajo_importacion_id=$7`, estado, res.Total, res.Creadas, res.Duplicadas, res.Rechazadas, u.EmpresaID, trabajoID)
	responderJSON(w, 201, map[string]any{"id": trabajoID, "estado": estado, "totalFilas": res.Total, "filasCreadas": res.Creadas, "filasDuplicadas": res.Duplicadas, "filasRechazadas": res.Rechazadas})
}

func (s *Servidor) procesarCartolaCSV(ctx context.Context, u sesionJWT, ruta, trabajoID, cuentaID string) resumenImportacion {
	var res resumenImportacion
	f, err := os.Open(ruta)
	if err != nil {
		return res
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	primera, _ := buf.ReadString('\n')
	_, _ = f.Seek(0, 0)
	lector := csv.NewReader(f)
	lector.TrimLeadingSpace = true
	lector.FieldsPerRecord = -1
	if strings.Count(primera, ";") > strings.Count(primera, ",") {
		lector.Comma = ';'
	}
	head, err := lector.Read()
	if err != nil {
		s.registrarErrorImportacion(ctx, u.EmpresaID, trabajoID, nil, "archivo", "cabecera_invalida", err.Error(), nil)
		return res
	}
	mapa := map[string]int{}
	for i, v := range head {
		mapa[normalizarCabecera(v)] = i
	}
	for _, requerida := range []string{"fecha", "descripcion", "monto"} {
		if _, ok := mapa[requerida]; !ok {
			s.registrarErrorImportacion(ctx, u.EmpresaID, trabajoID, nil, requerida, "columna_faltante", "Falta la columna "+requerida, nil)
			return res
		}
	}
	for numero := 2; ; numero++ {
		fila, err := lector.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		res.Total++
		if err != nil {
			res.Rechazadas++
			n := numero
			s.registrarErrorImportacion(ctx, u.EmpresaID, trabajoID, &n, "fila", "csv_invalido", err.Error(), nil)
			continue
		}
		valor := func(nombre string) string {
			if i, ok := mapa[nombre]; ok && i < len(fila) {
				return strings.TrimSpace(fila[i])
			}
			return ""
		}
		datos := map[string]string{"fecha": valor("fecha"), "descripcion": valor("descripcion"), "referencia": valor("referencia"), "monto": valor("monto"), "saldo": valor("saldo")}
		duplicado, e := s.insertarMovimientoBancario(ctx, u, trabajoID, cuentaID, datos)
		if e != nil {
			res.Rechazadas++
			n := numero
			s.registrarErrorImportacion(ctx, u.EmpresaID, trabajoID, &n, "fila", "fila_invalida", e.Error(), datos)
		} else if duplicado {
			res.Duplicadas++
		} else {
			res.Creadas++
		}
	}
	return res
}

func normalizarCabecera(v string) string {
	v = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(v, "\ufeff")))
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", " ", "_", "-", "_")
	v = r.Replace(v)
	alias := map[string]string{"date": "fecha", "fecha_movimiento": "fecha", "description": "descripcion", "detalle": "descripcion", "glosa": "descripcion", "reference": "referencia", "numero_documento": "referencia", "amount": "monto", "importe": "monto", "balance": "saldo", "saldo_informado": "saldo"}
	if a, ok := alias[v]; ok {
		return a
	}
	return v
}

func parsearFechaBancaria(v string) (time.Time, error) {
	for _, formato := range []string{"2006-01-02", "02-01-2006", "02/01/2006"} {
		if f, err := time.Parse(formato, strings.TrimSpace(v)); err == nil {
			return f, nil
		}
	}
	return time.Time{}, errors.New("fecha debe usar YYYY-MM-DD, DD-MM-YYYY o DD/MM/YYYY")
}

func decimalBancario(v string) (decimal.Decimal, error) {
	v = strings.TrimSpace(strings.NewReplacer("$", "", " ", "").Replace(v))
	if v == "" {
		return decimal.Zero, errors.New("monto obligatorio")
	}
	ultimaComa, ultimoPunto := strings.LastIndex(v, ","), strings.LastIndex(v, ".")
	if ultimaComa >= 0 && ultimoPunto >= 0 {
		if ultimaComa > ultimoPunto {
			v = strings.ReplaceAll(v, ".", "")
			v = strings.ReplaceAll(v, ",", ".")
		} else {
			v = strings.ReplaceAll(v, ",", "")
		}
	} else if ultimaComa >= 0 {
		v = strings.ReplaceAll(v, ".", "")
		v = strings.ReplaceAll(v, ",", ".")
	} else if ultimoPunto >= 0 {
		decimales := len(v) - ultimoPunto - 1
		if strings.Count(v, ".") > 1 || decimales == 3 {
			v = strings.ReplaceAll(v, ".", "")
		}
	}
	d, err := decimal.NewFromString(v)
	if err != nil || d.IsZero() {
		return decimal.Zero, errors.New("monto inválido")
	}
	return d, nil
}

func (s *Servidor) insertarMovimientoBancario(ctx context.Context, u sesionJWT, trabajoID, cuentaID string, d map[string]string) (bool, error) {
	fecha, err := parsearFechaBancaria(d["fecha"])
	if err != nil {
		return false, err
	}
	descripcion := strings.TrimSpace(d["descripcion"])
	if descripcion == "" {
		return false, errors.New("descripcion obligatoria")
	}
	monto, err := decimalBancario(d["monto"])
	if err != nil {
		return false, err
	}
	var saldo any
	if strings.TrimSpace(d["saldo"]) != "" {
		sd, e := decimalBancario(d["saldo"])
		if e != nil {
			return false, fmt.Errorf("saldo: %w", e)
		}
		saldo = sd.StringFixed(4)
	}
	h := sha256.Sum256([]byte(cuentaID + "|" + fecha.Format("2006-01-02") + "|" + strings.ToLower(descripcion) + "|" + d["referencia"] + "|" + monto.StringFixed(4)))
	_, err = s.db.ExecContext(ctx, `INSERT INTO spa.movimientos_bancarios(empresa_id,cuenta_bancaria_id,trabajo_importacion_id,fecha_movimiento,descripcion,referencia,monto,saldo_informado,hash_movimiento) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9)`, u.EmpresaID, cuentaID, trabajoID, fecha, descripcion, d["referencia"], monto.StringFixed(4), saldo, hex.EncodeToString(h[:]))
	if esDuplicado(err) {
		return true, nil
	}
	return false, err
}

func (s *Servidor) listarMovimientosBancarios(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	q := r.URL.Query()
	pagina := enteroQuery(q.Get("page"), 1, 1, 100000)
	tamano := enteroQuery(q.Get("pageSize"), 30, 1, 100)
	args := []any{u.EmpresaID}
	where := []string{"m.empresa_id=$1", "m.eliminado_en IS NULL"}
	if cuenta := strings.TrimSpace(q.Get("accountId")); cuenta != "" {
		args = append(args, cuenta)
		where = append(where, fmt.Sprintf("m.cuenta_bancaria_id=$%d", len(args)))
	}
	if clase := strings.TrimSpace(q.Get("classification")); clase != "" {
		args = append(args, clase)
		where = append(where, fmt.Sprintf("m.clasificacion=$%d", len(args)))
	}
	var total int
	if err := s.db.QueryRowContext(r.Context(), `SELECT count(*) FROM spa.movimientos_bancarios m WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		s.errorInterno(w, r, err)
		return
	}
	args = append(args, tamano, (pagina-1)*tamano)
	rows, err := s.db.QueryContext(r.Context(), `SELECT m.movimiento_bancario_id,m.cuenta_bancaria_id,c.banco,c.numero_enmascarado,m.fecha_movimiento,m.descripcion,COALESCE(m.referencia,''),m.monto,m.saldo_informado,m.clasificacion,m.origen_clasificacion,COALESCE(m.explicacion_sugerencia,''),EXISTS(SELECT 1 FROM spa.conciliaciones x WHERE x.empresa_id=m.empresa_id AND x.movimiento_bancario_id=m.movimiento_bancario_id AND x.estado='confirmada' AND x.eliminado_en IS NULL) FROM spa.movimientos_bancarios m JOIN spa.cuentas_bancarias c ON c.empresa_id=m.empresa_id AND c.cuenta_bancaria_id=m.cuenta_bancaria_id WHERE `+strings.Join(where, " AND ")+` ORDER BY m.fecha_movimiento DESC,m.movimiento_bancario_id DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, cuenta, banco, numero, descripcion, referencia, monto, clase, origen, justificacion string
		var fecha time.Time
		var saldo sql.NullString
		var conciliado bool
		if err := rows.Scan(&id, &cuenta, &banco, &numero, &fecha, &descripcion, &referencia, &monto, &saldo, &clase, &origen, &justificacion, &conciliado); err != nil {
			s.errorInterno(w, r, err)
			return
		}
		items = append(items, map[string]any{"id": id, "cuentaId": cuenta, "banco": banco, "cuenta": numero, "fecha": fecha.Format("2006-01-02"), "descripcion": descripcion, "referencia": referencia, "monto": monto, "saldo": nuloString(saldo), "clasificacion": clase, "origenClasificacion": origen, "justificacion": justificacion, "conciliado": conciliado})
	}
	responderJSON(w, 200, map[string]any{"items": items, "page": pagina, "pageSize": tamano, "total": total, "totalPages": intCeil(total, tamano)})
}

type clasificacionEntrada struct {
	Clasificacion string `json:"clasificacion"`
	Justificacion string `json:"justificacion"`
}

func (s *Servidor) clasificarMovimiento(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	var e clasificacionEntrada
	if err := decodificarJSON(w, r, &e); err != nil {
		responderProblema(w, r, 422, "clasificacion_invalida", "Clasificación inválida", err.Error())
		return
	}
	e.Clasificacion = strings.ToLower(strings.TrimSpace(e.Clasificacion))
	e.Justificacion = strings.TrimSpace(e.Justificacion)
	if !enLista(e.Clasificacion, clasificacionesBancarias...) {
		responderProblema(w, r, 422, "clasificacion_invalida", "Clasificación inválida", "Selecciona una clasificación admitida.")
		return
	}
	if enLista(e.Clasificacion, "gasto_empresarial", "gasto_personal", "transferencia_propietario", "posible_distribucion") && e.Justificacion == "" {
		responderProblema(w, r, 422, "justificacion_requerida", "Justificación requerida", "Explica el propósito y conserva el respaldo correspondiente.")
		return
	}
	resultado, err := s.db.ExecContext(r.Context(), `UPDATE spa.movimientos_bancarios SET clasificacion=$1,origen_clasificacion='manual',explicacion_sugerencia=NULLIF($2,''),confirmado_por=$3,fecha_confirmacion=CURRENT_TIMESTAMP WHERE empresa_id=$4 AND movimiento_bancario_id=$5 AND eliminado_en IS NULL`, e.Clasificacion, e.Justificacion, u.UsuarioID, u.EmpresaID, r.PathValue("id"))
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	n, _ := resultado.RowsAffected()
	if n == 0 {
		responderProblema(w, r, 404, "movimiento_no_encontrado", "Movimiento no encontrado", "")
		return
	}
	responderJSON(w, 200, map[string]any{"id": r.PathValue("id"), "clasificacion": e.Clasificacion, "justificacion": e.Justificacion})
}

func (s *Servidor) candidatosConciliacion(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	var monto decimal.Decimal
	var fecha time.Time
	if err := s.db.QueryRowContext(r.Context(), `SELECT monto,fecha_movimiento FROM spa.movimientos_bancarios WHERE empresa_id=$1 AND movimiento_bancario_id=$2 AND eliminado_en IS NULL`, u.EmpresaID, r.PathValue("id")).Scan(&monto, &fecha); err != nil {
		responderProblema(w, r, 404, "movimiento_no_encontrado", "Movimiento no encontrado", "")
		return
	}
	direccion := "compra"
	if monto.IsPositive() {
		direccion = "venta"
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT documento_tributario_id,tipo_documento,folio,fecha_emision,monto_total,monto_pagado,CASE WHEN direccion='venta' THEN COALESCE(razon_social_receptor,rut_receptor) ELSE COALESCE(razon_social_emisor,rut_emisor) END FROM spa.documentos_tributarios WHERE empresa_id=$1 AND direccion=$2 AND estado_pago IN ('pendiente','parcial') AND eliminado_en IS NULL AND fecha_emision BETWEEN $3::date-INTERVAL '90 days' AND $3::date+INTERVAL '30 days' ORDER BY abs(monto_total-$4),abs(fecha_emision-$3::date) LIMIT 20`, u.EmpresaID, direccion, fecha, monto.Abs().StringFixed(4))
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, tipo, folio, total, pagado, contraparte string
		var f time.Time
		if rows.Scan(&id, &tipo, &folio, &f, &total, &pagado, &contraparte) == nil {
			items = append(items, map[string]any{"id": id, "tipoDocumento": tipo, "folio": folio, "fechaEmision": f.Format("2006-01-02"), "montoTotal": total, "montoPagado": pagado, "contraparte": contraparte})
		}
	}
	responderJSON(w, 200, items)
}

type conciliacionEntrada struct {
	DocumentoID string `json:"documentoId"`
	Monto       string `json:"monto"`
	Observacion string `json:"observacion"`
}

func (s *Servidor) conciliarMovimiento(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	var e conciliacionEntrada
	if err := decodificarJSON(w, r, &e); err != nil {
		responderProblema(w, r, 422, "conciliacion_invalida", "Conciliación inválida", err.Error())
		return
	}
	monto, err := decimalCLP(e.Monto)
	if err != nil || monto.IsZero() {
		responderProblema(w, r, 422, "monto_invalido", "Monto inválido", "El monto conciliado debe ser mayor que cero.")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	defer tx.Rollback()
	var mov decimal.Decimal
	var direccion string
	if err = tx.QueryRowContext(r.Context(), `SELECT abs(monto),CASE WHEN monto>0 THEN 'venta' ELSE 'compra' END FROM spa.movimientos_bancarios WHERE empresa_id=$1 AND movimiento_bancario_id=$2 AND eliminado_en IS NULL FOR UPDATE`, u.EmpresaID, r.PathValue("id")).Scan(&mov, &direccion); err != nil {
		responderProblema(w, r, 404, "movimiento_no_encontrado", "Movimiento no encontrado", "")
		return
	}
	var total, pagado decimal.Decimal
	if err = tx.QueryRowContext(r.Context(), `SELECT monto_total,monto_pagado FROM spa.documentos_tributarios WHERE empresa_id=$1 AND documento_tributario_id=$2 AND direccion=$3 AND eliminado_en IS NULL FOR UPDATE`, u.EmpresaID, e.DocumentoID, direccion).Scan(&total, &pagado); err != nil {
		responderProblema(w, r, 422, "documento_invalido", "Documento no conciliable", "El documento no corresponde al flujo del movimiento.")
		return
	}
	var aplicado decimal.Decimal
	_ = tx.QueryRowContext(r.Context(), `SELECT COALESCE(sum(monto_conciliado),0) FROM spa.conciliaciones WHERE empresa_id=$1 AND movimiento_bancario_id=$2 AND estado='confirmada' AND eliminado_en IS NULL`, u.EmpresaID, r.PathValue("id")).Scan(&aplicado)
	if monto.GreaterThan(mov.Sub(aplicado)) || monto.GreaterThan(total.Sub(pagado)) {
		responderProblema(w, r, 422, "monto_excede_saldo", "Monto excede el saldo", "El monto supera el saldo pendiente del movimiento o documento.")
		return
	}
	id := nuevoUUID()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.conciliaciones(conciliacion_id,empresa_id,movimiento_bancario_id,estado,monto_conciliado,observacion,creado_por) VALUES($1,$2,$3,'confirmada',$4,NULLIF($5,''),$6)`, id, u.EmpresaID, r.PathValue("id"), monto.StringFixed(4), strings.TrimSpace(e.Observacion), u.UsuarioID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.conciliaciones_documentos(empresa_id,conciliacion_id,documento_tributario_id,monto_aplicado) VALUES($1,$2,$3,$4)`, u.EmpresaID, id, e.DocumentoID, monto.StringFixed(4))
	}
	if err == nil {
		nuevo := pagado.Add(monto)
		estado := "parcial"
		if nuevo.GreaterThanOrEqual(total) {
			estado = "pagado"
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE spa.documentos_tributarios SET monto_pagado=$1,estado_pago=$2 WHERE empresa_id=$3 AND documento_tributario_id=$4`, nuevo.StringFixed(4), estado, u.EmpresaID, e.DocumentoID)
	}
	if err == nil {
		clase := "pago_proveedor"
		if direccion == "venta" {
			clase = "cobro_cliente"
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE spa.movimientos_bancarios SET clasificacion=$1,origen_clasificacion='manual',confirmado_por=$2,fecha_confirmacion=CURRENT_TIMESTAMP WHERE empresa_id=$3 AND movimiento_bancario_id=$4`, clase, u.UsuarioID, u.EmpresaID, r.PathValue("id"))
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	responderJSON(w, 201, map[string]any{"id": id, "estado": "confirmada", "monto": monto.StringFixed(4)})
}

func (s *Servidor) resumenBancario(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	var total, pendientes, conciliados, gastos, propietario int
	err := s.db.QueryRowContext(r.Context(), `SELECT count(*),count(*) FILTER(WHERE clasificacion='pendiente_revision'),count(*) FILTER(WHERE EXISTS(SELECT 1 FROM spa.conciliaciones c WHERE c.empresa_id=m.empresa_id AND c.movimiento_bancario_id=m.movimiento_bancario_id AND c.estado='confirmada' AND c.eliminado_en IS NULL)),count(*) FILTER(WHERE clasificacion IN ('gasto_empresarial','gasto_personal','comision_bancaria')),count(*) FILTER(WHERE clasificacion IN ('transferencia_propietario','posible_distribucion','sueldo_empresarial','gasto_personal')) FROM spa.movimientos_bancarios m WHERE empresa_id=$1 AND eliminado_en IS NULL`, u.EmpresaID).Scan(&total, &pendientes, &conciliados, &gastos, &propietario)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	responderJSON(w, 200, map[string]int{"total": total, "pendientes": pendientes, "conciliados": conciliados, "gastos": gastos, "movimientosPropietario": propietario, "alertas": pendientes})
}
