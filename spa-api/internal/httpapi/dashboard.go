package httpapi

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

type indicador struct{Codigo string `json:"codigo"`;Etiqueta string `json:"etiqueta"`;Valor string `json:"valor"`;Estado string `json:"estado"`;Detalle string `json:"detalle,omitempty"`}

func periodo(r *http.Request)(int,int){ahora:=time.Now();anio:=enteroQuery(r.URL.Query().Get("year"),ahora.Year(),2000,2200);mes:=enteroQuery(r.URL.Query().Get("month"),int(ahora.Month()),0,12);return anio,mes}

func (s *Servidor) resumenDashboard(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);anio,mes:=periodo(r);inicio:=time.Date(anio,time.January,1,0,0,0,0,time.UTC);fin:=inicio.AddDate(1,0,0);if mes>0{inicio=time.Date(anio,time.Month(mes),1,0,0,0,0,time.UTC);fin=inicio.AddDate(0,1,0)}
	var ventas,compras,ivaDebito,ivaCredito,pendientes string;err:=s.db.QueryRowContext(r.Context(),`SELECT COALESCE(sum(CASE WHEN direccion='venta' THEN (CASE WHEN tipo_documento='nota_credito' THEN -monto_neto ELSE monto_neto END) ELSE 0 END),0)::text,COALESCE(sum(CASE WHEN direccion='compra' THEN (CASE WHEN tipo_documento='nota_credito' THEN -monto_neto ELSE monto_neto END) ELSE 0 END),0)::text,COALESCE(sum(CASE WHEN direccion='venta' THEN (CASE WHEN tipo_documento='nota_credito' THEN -monto_iva ELSE monto_iva END) ELSE 0 END),0)::text,COALESCE(sum(CASE WHEN direccion='compra' THEN (CASE WHEN tipo_documento='nota_credito' THEN -monto_iva ELSE monto_iva END) ELSE 0 END),0)::text,count(*) FILTER (WHERE estado_revision IN ('pendiente_revision','incompleto'))::text FROM spa.documentos_tributarios WHERE empresa_id=$1 AND fecha_emision >= $2 AND fecha_emision < $3 AND eliminado_en IS NULL AND estado_documento<>'anulado'`,u.EmpresaID,inicio,fin).Scan(&ventas,&compras,&ivaDebito,&ivaCredito,&pendientes)
	if err!=nil{s.errorInterno(w,r,err);return};ivaPagar:=restarNoNegativo(ivaDebito,ivaCredito);utilidad:=restar(ventas,compras)
	responderJSON(w,200,map[string]any{"year":anio,"month":mes,"updatedAt":time.Now().UTC(),"indicators":[]indicador{{"ventas_netas","Ventas netas",ventas,"estimado","Documentos de venta del periodo"},{"compras_netas","Compras netas",compras,"estimado","Documentos de compra del periodo"},{"iva_debito","IVA debito",ivaDebito,"estimado","IVA de ventas"},{"iva_credito","IVA credito",ivaCredito,"estimado","Sujeto a validacion de procedencia"},{"iva_pagar","IVA estimado por pagar",ivaPagar,"estimado","Debito menos credito; no constituye declaracion"},{"resultado_provisional","Resultado provisional",utilidad,"incompleto","Ventas menos compras; aun no incluye todos los gastos ni ajustes"},{"pendientes_revision","Pendientes de revision",pendientes,"pendiente_revision","Documentos con informacion incompleta o sin confirmar"}}})
}

func (s *Servidor) estimacionIVA(w http.ResponseWriter,r *http.Request){u:=sesionActual(r);anio,mes:=periodo(r);if mes==0{mes=int(time.Now().Month())};inicio:=time.Date(anio,time.Month(mes),1,0,0,0,0,time.UTC);fin:=inicio.AddDate(0,1,0);var debito,credito string;var documentos int;err:=s.db.QueryRowContext(r.Context(),`SELECT COALESCE(sum(CASE WHEN direccion='venta' THEN CASE WHEN tipo_documento='nota_credito' THEN -monto_iva ELSE monto_iva END ELSE 0 END),0)::text,COALESCE(sum(CASE WHEN direccion='compra' THEN CASE WHEN tipo_documento='nota_credito' THEN -monto_iva ELSE monto_iva END ELSE 0 END),0)::text,count(*) FROM spa.documentos_tributarios WHERE empresa_id=$1 AND fecha_emision >= $2 AND fecha_emision < $3 AND eliminado_en IS NULL AND estado_documento<>'anulado'`,u.EmpresaID,inicio,fin).Scan(&debito,&credito,&documentos);if err!=nil{s.errorInterno(w,r,err);return};responderJSON(w,200,map[string]any{"year":anio,"month":mes,"ivaDebito":debito,"ivaCredito":credito,"ivaPorPagar":restarNoNegativo(debito,credito),"remanenteCredito":restarNoNegativo(credito,debito),"documentosConsiderados":documentos,"estado":"estimado","advertencias":[]string{"Estimacion de apoyo; requiere validar procedencia del credito fiscal y antecedentes del periodo."}})}

func restar(a,b string)string{x,_:=decimal.NewFromString(a);y,_:=decimal.NewFromString(b);return x.Sub(y).StringFixed(4)}
func restarNoNegativo(a,b string)string{x,_:=decimal.NewFromString(restar(a,b));if x.IsNegative(){return "0.0000"};return x.StringFixed(4)}
