package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Servidor) listarDocumentos(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);q:=r.URL.Query();pagina:=enteroQuery(q.Get("page"),1,1,100000);tamano:=enteroQuery(q.Get("pageSize"),25,1,100);args:=[]any{u.EmpresaID};where:=[]string{"empresa_id=$1","eliminado_en IS NULL"}
	agregar:=func(valor,clausula string){if strings.TrimSpace(valor)!=""{args=append(args,valor);where=append(where,fmt.Sprintf(clausula,len(args)))}}
	agregar(q.Get("direction"),"direccion=$%d");agregar(q.Get("documentType"),"tipo_documento=$%d");agregar(q.Get("status"),"estado_pago=$%d");agregar(q.Get("dateFrom"),"fecha_emision >= $%d");agregar(q.Get("dateTo"),"fecha_emision <= $%d");if rut:=normalizarRUT(q.Get("counterpartyRut"));rut!=""{args=append(args,rut);where=append(where,fmt.Sprintf("(rut_emisor_normalizado=$%d OR rut_receptor_normalizado=$%d)",len(args),len(args)))}
	var total int;if err:=s.db.QueryRowContext(r.Context(),`SELECT count(*) FROM spa.documentos_tributarios WHERE `+strings.Join(where," AND "),args...).Scan(&total);err!=nil{s.errorInterno(w,r,err);return}
	orden:="fecha_emision";if enLista(q.Get("sort"),"folio","monto_total","fecha_emision","fecha_vencimiento"){orden=q.Get("sort")};direccionOrden:="DESC";if strings.EqualFold(q.Get("order"),"asc"){direccionOrden="ASC"};args=append(args,tamano,(pagina-1)*tamano)
	consulta:=`SELECT documento_tributario_id,direccion,tipo_documento,folio,rut_emisor,razon_social_emisor,rut_receptor,razon_social_receptor,fecha_emision,fecha_vencimiento,monto_neto,monto_exento,monto_iva,monto_total,estado_pago,estado_revision FROM spa.documentos_tributarios WHERE `+strings.Join(where," AND ")+` ORDER BY `+orden+` `+direccionOrden+`,documento_tributario_id LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args))
	rows,err:=s.db.QueryContext(r.Context(),consulta,args...);if err!=nil{s.errorInterno(w,r,err);return};defer rows.Close();items:=[]map[string]any{}
	for rows.Next(){var id,direccion,tipo,folio,rutE,rutR,mN,mEx,mIva,mTotal,estadoPago,estadoRevision string;var razonE,razonR sql.NullString;var fecha time.Time;var venc sql.NullTime;if err:=rows.Scan(&id,&direccion,&tipo,&folio,&rutE,&razonE,&rutR,&razonR,&fecha,&venc,&mN,&mEx,&mIva,&mTotal,&estadoPago,&estadoRevision);err!=nil{s.errorInterno(w,r,err);return};items=append(items,map[string]any{"id":id,"direccion":direccion,"tipoDocumento":tipo,"folio":folio,"rutEmisor":rutE,"razonSocialEmisor":nuloString(razonE),"rutReceptor":rutR,"razonSocialReceptor":nuloString(razonR),"fechaEmision":fecha.Format("2006-01-02"),"fechaVencimiento":nuloFecha(venc),"montoNeto":mN,"montoExento":mEx,"montoIva":mIva,"montoTotal":mTotal,"estadoPago":estadoPago,"estadoRevision":estadoRevision})}
	responderJSON(w,200,map[string]any{"items":items,"page":pagina,"pageSize":tamano,"total":total,"totalPages":intCeil(total,tamano)})
}

func (s *Servidor) detalleDocumento(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);var id,direccion,tipo,folio,rutE,rutR,mN,mEx,mIva,mOtros,mTotal,mPagado,estadoPago,estadoRevision,fuente string;var razonE,razonR sql.NullString;var fecha time.Time;var venc sql.NullTime
	err:=s.db.QueryRowContext(r.Context(),`SELECT documento_tributario_id,direccion,tipo_documento,folio,rut_emisor,razon_social_emisor,rut_receptor,razon_social_receptor,fecha_emision,fecha_vencimiento,monto_neto,monto_exento,monto_iva,monto_otros_impuestos,monto_total,monto_pagado,estado_pago,estado_revision,fuente FROM spa.documentos_tributarios WHERE empresa_id=$1 AND documento_tributario_id=$2 AND eliminado_en IS NULL`,u.EmpresaID,r.PathValue("id")).Scan(&id,&direccion,&tipo,&folio,&rutE,&razonE,&rutR,&razonR,&fecha,&venc,&mN,&mEx,&mIva,&mOtros,&mTotal,&mPagado,&estadoPago,&estadoRevision,&fuente)
	if err==sql.ErrNoRows{responderProblema(w,r,404,"documento_no_encontrado","Documento no encontrado","");return};if err!=nil{s.errorInterno(w,r,err);return};responderJSON(w,200,map[string]any{"id":id,"direccion":direccion,"tipoDocumento":tipo,"folio":folio,"rutEmisor":rutE,"razonSocialEmisor":nuloString(razonE),"rutReceptor":rutR,"razonSocialReceptor":nuloString(razonR),"fechaEmision":fecha.Format("2006-01-02"),"fechaVencimiento":nuloFecha(venc),"montoNeto":mN,"montoExento":mEx,"montoIva":mIva,"montoOtrosImpuestos":mOtros,"montoTotal":mTotal,"montoPagado":mPagado,"estadoPago":estadoPago,"estadoRevision":estadoRevision,"fuente":fuente})
}

func enteroQuery(v string,def,min,max int)int{n,err:=strconv.Atoi(v);if err!=nil{return def};if n<min{return min};if n>max{return max};return n}
func intCeil(a,b int)int{if a==0{return 0};return (a+b-1)/b}
func nuloFecha(v sql.NullTime)any{if v.Valid{return v.Time.Format("2006-01-02")};return nil}

