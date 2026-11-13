package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type chatEntrada struct {
	ConversacionID        string `json:"conversacionId"`
	Mensaje               string `json:"mensaje"`
	ConsentimientoExterno bool   `json:"consentimientoExterno"`
}

type contextoAsistente struct {
	Anio                  int    `json:"anio"`
	CalculoDisponible     bool   `json:"calculoDisponible"`
	EstadoCalculo         string `json:"estadoCalculo,omitempty"`
	IDPCEstimado          string `json:"idpcEstimado,omitempty"`
	IngresosPercibidos    string `json:"ingresosPercibidos,omitempty"`
	EgresosPagados        string `json:"egresosPagados,omitempty"`
	BaseImponible         string `json:"baseImponible,omitempty"`
	VersionReglas         string `json:"versionReglas,omitempty"`
	MovimientosPendientes int    `json:"movimientosPendientes"`
	DocumentosPendientes  int    `json:"documentosPendientes"`
}

func (s *Servidor) asistenteChat(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	var e chatEntrada
	if err := decodificarJSON(w, r, &e); err != nil {
		responderProblema(w, r, 422, "mensaje_invalido", "Mensaje inválido", err.Error())
		return
	}
	e.Mensaje = strings.TrimSpace(e.Mensaje)
	if len(e.Mensaje) < 2 || len(e.Mensaje) > 2000 {
		responderProblema(w, r, 422, "mensaje_invalido", "Mensaje inválido", "Escribe entre 2 y 2.000 caracteres.")
		return
	}
	ctx, err := s.contextoSeguroAsistente(r, u)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	herramientas := []string{"resumen_tributario", "pendientes_revision"}
	respuesta := respuestaLocalAsistente(e.Mensaje, ctx)
	proveedor := "local"
	modelo := "mispa-determinista-1"
	if s.cfg.ProveedorIA == "openai" && e.ConsentimientoExterno {
		externa, exErr := s.respuestaOpenAI(r, e.Mensaje, ctx)
		if exErr == nil && strings.TrimSpace(externa) != "" {
			respuesta = externa
			proveedor = "openai"
			modelo = s.cfg.OpenAIModelo
		} else {
			s.logger.Warn("proveedor IA no disponible; se usa respuesta local", "error", exErr)
		}
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	defer tx.Rollback()
	conversacionID := strings.TrimSpace(e.ConversacionID)
	if conversacionID == "" {
		conversacionID = nuevoUUID()
		titulo := e.Mensaje
		if len([]rune(titulo)) > 80 {
			titulo = string([]rune(titulo)[:80])
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.conversaciones_ia(conversacion_ia_id,empresa_id,usuario_id,titulo,consentimiento_datos,proveedor,modelo) VALUES($1,$2,$3,$4,$5,$6,$7)`, conversacionID, u.EmpresaID, u.UsuarioID, titulo, e.ConsentimientoExterno, proveedor, modelo)
	} else {
		var valido bool
		err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM spa.conversaciones_ia WHERE empresa_id=$1 AND usuario_id=$2 AND conversacion_ia_id=$3 AND estado='activa' AND eliminado_en IS NULL)`, u.EmpresaID, u.UsuarioID, conversacionID).Scan(&valido)
		if err == nil && !valido {
			err = sql.ErrNoRows
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		responderProblema(w, r, 404, "conversacion_no_encontrada", "Conversación no encontrada", "")
		return
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.mensajes_ia(empresa_id,conversacion_ia_id,rol,contenido) VALUES($1,$2,'usuario',$3)`, u.EmpresaID, conversacionID, e.Mensaje)
	}
	referencias := []string{}
	if ctx.CalculoDisponible {
		referencias = append(referencias, "calculo_tributario:"+ctx.VersionReglas)
	}
	advertencias := []string{"Respuesta informativa; no sustituye al SII ni a un profesional tributario."}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO spa.mensajes_ia(empresa_id,conversacion_ia_id,rol,contenido,herramientas_usadas,referencias_internas,advertencias) VALUES($1,$2,'asistente',$3,$4::jsonb,$5::jsonb,$6::jsonb)`, u.EmpresaID, conversacionID, respuesta, string(jsonBytes(herramientas)), string(jsonBytes(referencias)), string(jsonBytes(advertencias)))
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE spa.conversaciones_ia SET consentimiento_datos=$1,proveedor=$2,modelo=$3,fecha_modificacion=CURRENT_TIMESTAMP WHERE empresa_id=$4 AND conversacion_ia_id=$5`, e.ConsentimientoExterno, proveedor, modelo, u.EmpresaID, conversacionID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	responderJSON(w, 200, map[string]any{"conversacionId": conversacionID, "respuesta": respuesta, "proveedor": proveedor, "modelo": modelo, "herramientasUsadas": herramientas, "referencias": referencias, "advertencias": advertencias, "contexto": ctx})
}

func (s *Servidor) contextoSeguroAsistente(r *http.Request, u sesionJWT) (contextoAsistente, error) {
	c := contextoAsistente{Anio: time.Now().Year()}
	var calculoID string
	err := s.db.QueryRowContext(r.Context(), `SELECT calculo_tributario_id,EXTRACT(YEAR FROM periodo_desde)::int,estado,resultado_principal::text,version_reglas FROM spa.calculos_tributarios WHERE empresa_id=$1 AND tipo_calculo='primera_categoria' ORDER BY fecha_creacion DESC LIMIT 1`, u.EmpresaID).Scan(&calculoID, &c.Anio, &c.EstadoCalculo, &c.IDPCEstimado, &c.VersionReglas)
	if err == nil {
		c.CalculoDisponible = true
		rows, qErr := s.db.QueryContext(r.Context(), `SELECT codigo,valor_decimal::text FROM spa.resultados_calculo_tributario WHERE empresa_id=$1 AND calculo_tributario_id=$2`, u.EmpresaID, calculoID)
		if qErr != nil {
			return c, qErr
		}
		defer rows.Close()
		for rows.Next() {
			var codigo, valor string
			if rows.Scan(&codigo, &valor) == nil {
				switch codigo {
				case "ingresos_percibidos":
					c.IngresosPercibidos = valor
				case "egresos_pagados":
					c.EgresosPagados = valor
				case "base_imponible_estimada":
					c.BaseImponible = valor
				}
			}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return c, err
	}
	err = s.db.QueryRowContext(r.Context(), `SELECT count(*) FILTER(WHERE clasificacion='pendiente_revision') FROM spa.movimientos_bancarios WHERE empresa_id=$1 AND eliminado_en IS NULL`, u.EmpresaID).Scan(&c.MovimientosPendientes)
	if err != nil {
		return c, err
	}
	err = s.db.QueryRowContext(r.Context(), `SELECT count(*) FROM spa.documentos_tributarios WHERE empresa_id=$1 AND estado_revision IN ('pendiente_revision','incompleto') AND eliminado_en IS NULL`, u.EmpresaID).Scan(&c.DocumentosPendientes)
	return c, err
}

func respuestaLocalAsistente(pregunta string, c contextoAsistente) string {
	p := strings.ToLower(pregunta)
	advertencia := " Esta explicación es preliminar y debe revisarse con tu contador."
	if !c.CalculoDisponible {
		return "Todavía no existe una proyección tributaria trazable. Abre el módulo Tributario, revisa los movimientos pendientes y ejecuta el cálculo del año comercial correspondiente." + advertencia
	}
	if strings.Contains(p, "pendiente") || strings.Contains(p, "falta") {
		return fmt.Sprintf("Tienes %d movimientos bancarios y %d documentos pendientes de revisión. Clasifícalos y conserva sus respaldos antes de volver a calcular.", c.MovimientosPendientes, c.DocumentosPendientes) + advertencia
	}
	if strings.Contains(p, "gan") || strings.Contains(p, "utilidad") || strings.Contains(p, "resultado") {
		return fmt.Sprintf("Para %d, el cálculo usa ingresos percibidos por $%s y egresos pagados por $%s. La base imponible preliminar es $%s; no equivale necesariamente a caja disponible ni utilidad distribuible.", c.Anio, c.IngresosPercibidos, c.EgresosPagados, c.BaseImponible) + advertencia
	}
	if strings.Contains(p, "impuesto") || strings.Contains(p, "pagar") || strings.Contains(p, "renta") {
		return fmt.Sprintf("La proyección de IDPC para %d es $%s, calculada con la versión de reglas %s. No descuenta créditos, PPM pagados ni ajustes que no estén registrados.", c.Anio, c.IDPCEstimado, c.VersionReglas) + advertencia
	}
	return fmt.Sprintf("La última proyección corresponde a %d y estima un IDPC de $%s. Puedo explicar el resultado, los datos pendientes o la diferencia entre ingresos, egresos y base imponible.", c.Anio, c.IDPCEstimado) + advertencia
}

func (s *Servidor) respuestaOpenAI(r *http.Request, pregunta string, c contextoAsistente) (string, error) {
	contexto, _ := json.Marshal(c)
	instruccion := "Eres el asistente de MiSpA IA para microempresas chilenas. Explica únicamente el contexto agregado entregado. No inventes tasas, saldos, normas ni datos. No des asesoría legal ni afirmes que una declaración está lista. Distingue caja, resultado y base imponible. Responde en español claro, cita el año y la versión de reglas, enumera datos faltantes y termina recomendando revisión profesional."
	payload := map[string]any{"model": s.cfg.OpenAIModelo, "store": false, "max_output_tokens": 800, "instructions": instruccion, "input": "Pregunta: " + pregunta + "\nContexto interno anonimizado JSON: " + string(contexto)}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.OpenAIAPIKey)
	req.Header.Set("Content-Type", "application/json")
	cliente := &http.Client{Timeout: time.Duration(s.cfg.TimeoutIASegundos) * time.Second}
	resp, err := cliente.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	limitado := io.LimitReader(resp.Body, 2<<20)
	datos, err := io.ReadAll(limitado)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OpenAI Responses devolvió estado %d", resp.StatusCode)
	}
	var salida struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(datos, &salida) != nil {
		return "", errors.New("respuesta externa inválida")
	}
	for _, item := range salida.Output {
		for _, contenido := range item.Content {
			if contenido.Type == "output_text" && strings.TrimSpace(contenido.Text) != "" {
				return contenido.Text, nil
			}
		}
	}
	return "", errors.New("respuesta externa sin texto")
}

func (s *Servidor) listarConversacionesIA(w http.ResponseWriter, r *http.Request) {
	u := sesionActual(r)
	rows, err := s.db.QueryContext(r.Context(), `SELECT conversacion_ia_id,COALESCE(titulo,''),proveedor,modelo,fecha_modificacion FROM spa.conversaciones_ia WHERE empresa_id=$1 AND usuario_id=$2 AND eliminado_en IS NULL ORDER BY fecha_modificacion DESC LIMIT 30`, u.EmpresaID, u.UsuarioID)
	if err != nil {
		s.errorInterno(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, titulo string
		var proveedor, modelo sql.NullString
		var fecha time.Time
		if rows.Scan(&id, &titulo, &proveedor, &modelo, &fecha) == nil {
			items = append(items, map[string]any{"id": id, "titulo": titulo, "proveedor": nuloString(proveedor), "modelo": nuloString(modelo), "fechaModificacion": fecha})
		}
	}
	responderJSON(w, 200, items)
}
