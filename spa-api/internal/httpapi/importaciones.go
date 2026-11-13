package httpapi

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var columnasCSV=[]string{"direction","document_type","folio","issuer_rut","recipient_rut","issue_date","due_date","net_amount_clp","exempt_amount_clp","vat_amount_clp","other_taxes_clp","total_amount_clp","payment_status"}

func (s *Servidor) importarDocumentos(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);r.Body=http.MaxBytesReader(w,r.Body,int64(s.cfg.MaxArchivoBytes));if err:=r.ParseMultipartForm(2<<20);err!=nil{responderProblema(w,r,413,"archivo_demasiado_grande","Archivo no aceptado","El archivo supera el limite o el formulario es invalido.");return}
	f,cab,err:=r.FormFile("archivo");if err!=nil{responderProblema(w,r,422,"archivo_requerido","Archivo requerido","Adjunta el CSV en el campo archivo.");return};defer f.Close()
	if !strings.HasSuffix(strings.ToLower(cab.Filename),".csv"){responderProblema(w,r,422,"formato_no_admitido","Formato no admitido","En el MVP se admite un archivo CSV.");return}
	ruta,nombre,hash,tamano,err:=s.guardarArchivo(f,cab);if err!=nil{s.errorInterno(w,r,err);return}
	defer func(){if err!=nil{_ = os.Remove(ruta)}}()
	var existente string
	if qerr:=s.db.QueryRowContext(r.Context(),`SELECT t.trabajo_importacion_id FROM spa.trabajos_importacion t JOIN spa.archivos_importados a ON a.empresa_id=t.empresa_id AND a.archivo_importado_id=t.archivo_importado_id WHERE t.empresa_id=$1 AND a.hash_sha256=$2 ORDER BY t.fecha_creacion DESC LIMIT 1`,u.EmpresaID,hash).Scan(&existente);qerr==nil{_ = os.Remove(ruta);responderJSON(w,200,map[string]any{"id":existente,"duplicado":true});return}
	archivoID:=nuevoUUID();trabajoID:=nuevoUUID();clave:=strings.TrimSpace(r.Header.Get("Idempotency-Key"));if clave==""{clave=hash}
	tx,err:=s.db.BeginTx(r.Context(),nil);if err!=nil{s.errorInterno(w,r,err);return};defer tx.Rollback()
	_,err=tx.ExecContext(r.Context(),`INSERT INTO spa.archivos_importados(archivo_importado_id,empresa_id,nombre_original,nombre_almacenado,ruta_almacenamiento,tipo_mime,extension,tamano_bytes,hash_sha256,origen,subido_por) VALUES($1,$2,$3,$4,$5,'text/csv','.csv',$6,$7,'sii_csv',$8)`,archivoID,u.EmpresaID,cab.Filename,nombre,ruta,tamano,hash,u.UsuarioID)
	if err==nil{_,err=tx.ExecContext(r.Context(),`INSERT INTO spa.trabajos_importacion(trabajo_importacion_id,empresa_id,archivo_importado_id,tipo,estado,clave_idempotencia,iniciado_por,fecha_inicio) VALUES($1,$2,$3,'documentos_tributarios','procesando',$4,$5,CURRENT_TIMESTAMP)`,trabajoID,u.EmpresaID,archivoID,clave,u.UsuarioID)}
	if err==nil{err=tx.Commit()};if err!=nil{if esDuplicado(err){responderProblema(w,r,409,"importacion_duplicada","Importacion duplicada","La clave de idempotencia ya fue utilizada.");return};s.errorInterno(w,r,err);return}
	resumen:=s.procesarCSV(r.Context(),u,ruta,trabajoID,archivoID)
	estado:="completado";if resumen.Rechazadas>0{estado="completado_con_errores"};if resumen.Total==resumen.Rechazadas&&resumen.Total>0{estado="fallido"}
	_,_=s.db.ExecContext(r.Context(),`UPDATE spa.trabajos_importacion SET estado=$1,total_filas=$2,filas_creadas=$3,filas_duplicadas=$4,filas_rechazadas=$5,fecha_fin=CURRENT_TIMESTAMP WHERE empresa_id=$6 AND trabajo_importacion_id=$7`,estado,resumen.Total,resumen.Creadas,resumen.Duplicadas,resumen.Rechazadas,u.EmpresaID,trabajoID)
	responderJSON(w,201,map[string]any{"id":trabajoID,"estado":estado,"totalFilas":resumen.Total,"filasCreadas":resumen.Creadas,"filasDuplicadas":resumen.Duplicadas,"filasRechazadas":resumen.Rechazadas})
}

type resumenImportacion struct{Total,Creadas,Duplicadas,Rechazadas int}

func (s *Servidor) procesarCSV(ctx context.Context,u sesionJWT,ruta,trabajoID,archivoID string) resumenImportacion{
	var res resumenImportacion;f,err:=os.Open(ruta);if err!=nil{return res};defer f.Close();lector:=csv.NewReader(f);lector.TrimLeadingSpace=true;lector.FieldsPerRecord=-1
	buf:=bufio.NewReader(f);linea,_:=buf.ReadString('\n');if strings.Count(linea,";")>strings.Count(linea,","){lector.Comma=';'};_,_=f.Seek(0,0);lector=csv.NewReader(f);lector.TrimLeadingSpace=true;lector.FieldsPerRecord=-1;if strings.Count(linea,";")>strings.Count(linea,","){lector.Comma=';'}
	head,err:=lector.Read();if err!=nil{s.registrarErrorImportacion(ctx,u.EmpresaID,trabajoID,nil,"archivo","cabecera_invalida",err.Error(),nil);return res};mapa:=map[string]int{};for i,v:=range head{mapa[strings.TrimSpace(strings.TrimPrefix(v,"\ufeff"))]=i};for _,c:=range columnasCSV{if _,ok:=mapa[c];!ok{s.registrarErrorImportacion(ctx,u.EmpresaID,trabajoID,nil,c,"columna_faltante","Falta la columna obligatoria "+c,nil);return res}}
	for numero:=2;;numero++{fila,err:=lector.Read();if errors.Is(err,io.EOF){break};res.Total++;if err!=nil{res.Rechazadas++;n:=numero;s.registrarErrorImportacion(ctx,u.EmpresaID,trabajoID,&n,"fila","csv_invalido",err.Error(),nil);continue};datos:=map[string]string{};for c,i:=range mapa{if i<len(fila){datos[c]=strings.TrimSpace(fila[i])}}
		duplicado,err:=s.insertarDocumentoImportado(ctx,u,trabajoID,archivoID,datos);if err!=nil{res.Rechazadas++;n:=numero;s.registrarErrorImportacion(ctx,u.EmpresaID,trabajoID,&n,"fila","fila_invalida",err.Error(),datos)}else if duplicado{res.Duplicadas++}else{res.Creadas++}
	};return res
}

func (s *Servidor) insertarDocumentoImportado(ctx context.Context,u sesionJWT,trabajoID,archivoID string,d map[string]string)(bool,error){
	direccion:=strings.ToLower(d["direction"]);tipo:=strings.ToLower(d["document_type"]);estadoPago:=strings.ToLower(d["payment_status"]);if direccion!="compra"&&direccion!="venta"{return false,errors.New("direction debe ser compra o venta")};if !enLista(tipo,"factura","factura_exenta","nota_credito","nota_debito","boleta","boleta_exenta","liquidacion_factura","otro"){return false,errors.New("document_type no reconocido")};if !enLista(estadoPago,"pendiente","parcial","pagado","no_aplica"){return false,errors.New("payment_status no reconocido")};if d["folio"]==""{return false,errors.New("folio es obligatorio")}
	fecha,err:=time.Parse("2006-01-02",d["issue_date"]);if err!=nil{return false,errors.New("issue_date debe usar YYYY-MM-DD")};var vencimiento any=nil;if d["due_date"]!=""{v,fechaErr:=time.Parse("2006-01-02",d["due_date"]);if fechaErr!=nil{return false,errors.New("due_date debe usar YYYY-MM-DD")};if v.Before(fecha){return false,errors.New("due_date no puede ser anterior a issue_date")};vencimiento=v};emisor:=normalizarRUT(d["issuer_rut"]);receptor:=normalizarRUT(d["recipient_rut"]);if !rutValido(emisor)||!rutValido(receptor){return false,errors.New("RUT emisor o receptor invalido")}
	var rutEmpresa string;if err=s.db.QueryRowContext(ctx,`SELECT rut_normalizado FROM spa.empresas WHERE empresa_id=$1`,u.EmpresaID).Scan(&rutEmpresa);err!=nil{return false,err};if (direccion=="compra"&&receptor!=rutEmpresa)||(direccion=="venta"&&emisor!=rutEmpresa){return false,errors.New("el RUT de la empresa no coincide con la direccion del documento")}
	neto,err:=decimalCLP(d["net_amount_clp"]);if err!=nil{return false,fmt.Errorf("net_amount_clp: %w",err)};exento,err:=decimalCLP(d["exempt_amount_clp"]);if err!=nil{return false,fmt.Errorf("exempt_amount_clp: %w",err)};iva,err:=decimalCLP(d["vat_amount_clp"]);if err!=nil{return false,fmt.Errorf("vat_amount_clp: %w",err)};otros,err:=decimalCLP(d["other_taxes_clp"]);if err!=nil{return false,fmt.Errorf("other_taxes_clp: %w",err)};total,err:=decimalCLP(d["total_amount_clp"]);if err!=nil{return false,fmt.Errorf("total_amount_clp: %w",err)};if neto.Add(exento).Add(iva).Add(otros).Sub(total).Abs().GreaterThan(decimal.NewFromInt(1)){return false,errors.New("los componentes no coinciden con el total")}
	contraparteRUT:=emisor;if direccion=="venta"{contraparteRUT=receptor};tx,err:=s.db.BeginTx(ctx,nil);if err!=nil{return false,err};defer tx.Rollback();var contraparteID string
	err=tx.QueryRowContext(ctx,`INSERT INTO spa.contrapartes(empresa_id,rut,rut_normalizado,razon_social,es_cliente,es_proveedor) VALUES($1,$2,$3,$3,$4,$5) ON CONFLICT (empresa_id,rut_normalizado) WHERE eliminado_en IS NULL DO UPDATE SET es_cliente=spa.contrapartes.es_cliente OR EXCLUDED.es_cliente,es_proveedor=spa.contrapartes.es_proveedor OR EXCLUDED.es_proveedor RETURNING contraparte_id`,u.EmpresaID,contraparteRUT,contraparteRUT,direccion=="venta",direccion=="compra").Scan(&contraparteID)
	if err!=nil{return false,err};_,err=tx.ExecContext(ctx,`INSERT INTO spa.documentos_tributarios(empresa_id,contraparte_id,trabajo_importacion_id,archivo_importado_id,direccion,tipo_documento,folio,rut_emisor,rut_emisor_normalizado,rut_receptor,rut_receptor_normalizado,fecha_emision,fecha_vencimiento,monto_neto,monto_exento,monto_iva,monto_otros_impuestos,monto_total,estado_pago,estado_revision,fuente,datos_originales,creado_por) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,'estimado','importacion_csv',$20,$21)`,u.EmpresaID,contraparteID,trabajoID,archivoID,direccion,tipo,d["folio"],d["issuer_rut"],emisor,d["recipient_rut"],receptor,fecha,vencimiento,neto.StringFixed(4),exento.StringFixed(4),iva.StringFixed(4),otros.StringFixed(4),total.StringFixed(4),estadoPago,jsonBytes(d),u.UsuarioID)
	if err!=nil{if esDuplicado(err){return true,nil};return false,err};return false,tx.Commit()
}

func (s *Servidor) guardarArchivo(origen multipart.File,cab *multipart.FileHeader)(ruta,nombre,hash string,tamano int64,err error){nombre=nuevoUUID()+".csv";ruta=filepath.Join(s.cfg.DirectorioUploads,nombre);destino,e:=os.OpenFile(ruta,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if e!=nil{err=e;return};defer destino.Close();h:=sha256.New();tamano,err=io.Copy(io.MultiWriter(destino,h),origen);if err!=nil{return};hash=hex.EncodeToString(h.Sum(nil));return}
func decimalCLP(v string)(decimal.Decimal,error){v=strings.TrimSpace(strings.ReplaceAll(v,"$",""));v=strings.ReplaceAll(v," ","");if strings.Contains(v,","){v=strings.ReplaceAll(v,".","");v=strings.ReplaceAll(v,",",".")};d,err:=decimal.NewFromString(v);if err!=nil||d.IsNegative(){return decimal.Zero,errors.New("monto invalido")};return d, nil}
func enLista(v string,permitidos ...string)bool{for _,x:=range permitidos{if v==x{return true}};return false}
func jsonBytes(v any)[]byte{b,_:=json.Marshal(v);return b}
func (s *Servidor) registrarErrorImportacion(ctx context.Context,empresa,trabajo string,fila *int,columna,codigo,mensaje string,datos any){_,_=s.db.ExecContext(ctx,`INSERT INTO spa.errores_importacion(empresa_id,trabajo_importacion_id,numero_fila,columna,codigo_error,mensaje,datos_fila) VALUES($1,$2,$3,$4,$5,$6,$7)`,empresa,trabajo,fila,columna,codigo,mensaje,jsonBytes(datos))}

func (s *Servidor) listarImportaciones(w http.ResponseWriter,r *http.Request){u:=sesionActual(r);rows,err:=s.db.QueryContext(r.Context(),`SELECT trabajo_importacion_id,tipo,estado,total_filas,filas_creadas,filas_duplicadas,filas_rechazadas,fecha_creacion FROM spa.trabajos_importacion WHERE empresa_id=$1 ORDER BY fecha_creacion DESC LIMIT 100`,u.EmpresaID);if err!=nil{s.errorInterno(w,r,err);return};defer rows.Close();salida:=[]map[string]any{};for rows.Next(){var id,tipo,estado string;var total,creadas,duplicadas,rechazadas int;var fecha time.Time;if rows.Scan(&id,&tipo,&estado,&total,&creadas,&duplicadas,&rechazadas,&fecha)==nil{salida=append(salida,map[string]any{"id":id,"tipo":tipo,"estado":estado,"totalFilas":total,"filasCreadas":creadas,"filasDuplicadas":duplicadas,"filasRechazadas":rechazadas,"fechaCreacion":fecha})}};responderJSON(w,200,salida)}
func (s *Servidor) detalleImportacion(w http.ResponseWriter,r *http.Request){u:=sesionActual(r);var id,tipo,estado,nombre string;var total,creadas,duplicadas,rechazadas int;var fecha time.Time;err:=s.db.QueryRowContext(r.Context(),`SELECT t.trabajo_importacion_id,t.tipo,t.estado,a.nombre_original,t.total_filas,t.filas_creadas,t.filas_duplicadas,t.filas_rechazadas,t.fecha_creacion FROM spa.trabajos_importacion t JOIN spa.archivos_importados a ON a.empresa_id=t.empresa_id AND a.archivo_importado_id=t.archivo_importado_id WHERE t.empresa_id=$1 AND t.trabajo_importacion_id=$2`,u.EmpresaID,r.PathValue("id")).Scan(&id,&tipo,&estado,&nombre,&total,&creadas,&duplicadas,&rechazadas,&fecha);if err!=nil{responderProblema(w,r,404,"importacion_no_encontrada","Importacion no encontrada","");return};responderJSON(w,200,map[string]any{"id":id,"tipo":tipo,"estado":estado,"archivo":nombre,"totalFilas":total,"filasCreadas":creadas,"filasDuplicadas":duplicadas,"filasRechazadas":rechazadas,"fechaCreacion":fecha})}
func (s *Servidor) erroresImportacion(w http.ResponseWriter,r *http.Request){u:=sesionActual(r);rows,err:=s.db.QueryContext(r.Context(),`SELECT numero_fila,columna,codigo_error,mensaje FROM spa.errores_importacion WHERE empresa_id=$1 AND trabajo_importacion_id=$2 ORDER BY numero_fila NULLS FIRST LIMIT 500`,u.EmpresaID,r.PathValue("id"));if err!=nil{s.errorInterno(w,r,err);return};defer rows.Close();salida:=[]map[string]any{};for rows.Next(){var fila sql.NullInt64;var col,cod,msg string;if rows.Scan(&fila,&col,&cod,&msg)==nil{salida=append(salida,map[string]any{"fila":nuloInt(fila),"columna":col,"codigo":cod,"mensaje":msg})}};responderJSON(w,200,salida)}
func nuloInt(v sql.NullInt64)any{if v.Valid{return v.Int64};return nil}
