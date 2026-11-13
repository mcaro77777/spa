package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

func (s *Servidor) reporteComprasVentasExcel(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);anio,mes:=periodo(r);inicio,fin:=rangoPeriodo(anio,mes);nombreEmpresa,rutEmpresa,err:=s.datosEmpresa(r,u.EmpresaID);if err!=nil{s.errorInterno(w,r,err);return}
	rows,err:=s.db.QueryContext(r.Context(),`SELECT direccion,tipo_documento,folio,fecha_emision,rut_emisor,COALESCE(razon_social_emisor,''),rut_receptor,COALESCE(razon_social_receptor,''),monto_neto::text,monto_exento::text,monto_iva::text,monto_otros_impuestos::text,monto_total::text,estado_pago,estado_revision FROM spa.documentos_tributarios WHERE empresa_id=$1 AND fecha_emision >= $2 AND fecha_emision < $3 AND eliminado_en IS NULL ORDER BY fecha_emision,folio`,u.EmpresaID,inicio,fin);if err!=nil{s.errorInterno(w,r,err);return};defer rows.Close()
	libro:=excelize.NewFile();defer libro.Close();hoja:="Compras y ventas";libro.SetSheetName("Sheet1",hoja)
	titulo,_:=libro.NewStyle(&excelize.Style{Font:&excelize.Font{Bold:true,Color:"FFFFFF",Size:16},Fill:excelize.Fill{Type:"pattern",Color:[]string{"08756B"},Pattern:1},Alignment:&excelize.Alignment{Vertical:"center"}})
	cabecera,_:=libro.NewStyle(&excelize.Style{Font:&excelize.Font{Bold:true,Color:"FFFFFF"},Fill:excelize.Fill{Type:"pattern",Color:[]string{"155E5A"},Pattern:1},Alignment:&excelize.Alignment{WrapText:true}})
	_,_=libro.NewStyle(&excelize.Style{NumFmt:1})
	libro.MergeCell(hoja,"A1","O1");libro.SetCellValue(hoja,"A1","MiSpA IA · Reporte de compras y ventas");libro.SetCellStyle(hoja,"A1","O1",titulo);libro.SetRowHeight(hoja,1,30)
	libro.SetCellValue(hoja,"A2",nombreEmpresa+" · "+rutEmpresa);libro.SetCellValue(hoja,"A3",fmt.Sprintf("Periodo: %s · Generado: %s",etiquetaPeriodo(anio,mes),time.Now().In(time.FixedZone("Chile",-3*60*60)).Format("02-01-2006 15:04")));libro.SetCellValue(hoja,"A4","Documento de apoyo. Los valores requieren revision y no constituyen una declaracion oficial.")
	encabezados:=[]string{"Dirección","Tipo","Folio","Fecha","RUT emisor","Razón social emisor","RUT receptor","Razón social receptor","Neto","Exento","IVA","Otros impuestos","Total","Estado pago","Estado revisión"};for i,v:=range encabezados{celda,_:=excelize.CoordinatesToCellName(i+1,6);libro.SetCellValue(hoja,celda,v)};libro.SetCellStyle(hoja,"A6","O6",cabecera)
	fila:=7;for rows.Next(){var direccion,tipo,folio,rutE,razonE,rutR,razonR,neto,exento,iva,otros,total,estadoPago,estadoRevision string;var fecha time.Time;if err:=rows.Scan(&direccion,&tipo,&folio,&fecha,&rutE,&razonE,&rutR,&razonR,&neto,&exento,&iva,&otros,&total,&estadoPago,&estadoRevision);err!=nil{s.errorInterno(w,r,err);return};valores:=[]any{direccion,tipo,folio,fecha.Format("2006-01-02"),rutE,razonE,rutR,razonR,neto,exento,iva,otros,total,estadoPago,estadoRevision};for i,v:=range valores{celda,_:=excelize.CoordinatesToCellName(i+1,fila);libro.SetCellValue(hoja,celda,v)};fila++}
	for _,col:=range []string{"A","B","C","D","E","G","N","O"}{libro.SetColWidth(hoja,col,col,15)};libro.SetColWidth(hoja,"F","F",28);libro.SetColWidth(hoja,"H","H",28);libro.SetColWidth(hoja,"I","M",16);_ = libro.AutoFilter(hoja,fmt.Sprintf("A6:O%d",maximo(fila-1,6)),nil);_ = libro.SetPanes(hoja,&excelize.Panes{Freeze:true,YSplit:6,TopLeftCell:"A7",ActivePane:"bottomLeft"})
	w.Header().Set("Content-Type","application/vnd.openxmlformats-officedocument.spreadsheetml.sheet");w.Header().Set("Content-Disposition",fmt.Sprintf(`attachment; filename="compras-ventas-%04d-%02d.xlsx"`,anio,mes));if err:=libro.Write(w);err!=nil{s.logger.Error("escribir reporte Excel","error",err)}
}

func (s *Servidor) reporteIVAPDF(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);anio,mes:=periodo(r);if mes==0{mes=int(time.Now().Month())};inicio,fin:=rangoPeriodo(anio,mes);nombreEmpresa,rutEmpresa,err:=s.datosEmpresa(r,u.EmpresaID);if err!=nil{s.errorInterno(w,r,err);return};debito,credito,documentos,err:=s.consultarIVA(r,u.EmpresaID,inicio,fin);if err!=nil{s.errorInterno(w,r,err);return};pagar:=restarNoNegativo(debito,credito);remanente:=restarNoNegativo(credito,debito)
	pdf:=gofpdf.New("P","mm","A4","");pdf.SetMargins(18,18,18);pdf.AddPage();pdf.SetFillColor(8,117,107);pdf.Rect(0,0,210,42,"F");pdf.SetTextColor(255,255,255);pdf.SetFont("Arial","B",20);pdf.CellFormat(0,10,"MiSpA IA", "",1,"L",false,0,"");pdf.SetFont("Arial","",11);pdf.CellFormat(0,7,"Estimacion mensual de IVA", "",1,"L",false,0,"");pdf.Ln(18);pdf.SetTextColor(23,46,45);pdf.SetFont("Arial","B",14);pdf.CellFormat(0,8,nombreEmpresa,"",1,"L",false,0,"");pdf.SetFont("Arial","",10);pdf.CellFormat(0,6,"RUT: "+rutEmpresa,"",1,"L",false,0,"");pdf.CellFormat(0,6,"Periodo: "+etiquetaPeriodo(anio,mes),"",1,"L",false,0,"");pdf.CellFormat(0,6,"Generado: "+time.Now().Format("02-01-2006 15:04"),"",1,"L",false,0,"");pdf.Ln(8)
	filas:=[][2]string{{"IVA debito estimado",formatearCLP(debito)},{"IVA credito informado",formatearCLP(credito)},{"IVA estimado por pagar",formatearCLP(pagar)},{"Remanente estimado",formatearCLP(remanente)},{"Documentos considerados",fmt.Sprintf("%d",documentos)}};for i,f:=range filas{if i%2==0{pdf.SetFillColor(238,247,244)}else{pdf.SetFillColor(255,255,255)};pdf.SetFont("Arial","",10);pdf.CellFormat(105,10,f[0],"",0,"L",true,0,"");pdf.SetFont("Arial","B",10);pdf.CellFormat(51,10,f[1],"",1,"R",true,0,"")};pdf.Ln(10);pdf.SetFillColor(255,247,229);pdf.SetTextColor(98,73,24);pdf.SetFont("Arial","",9);pdf.MultiCell(0,6,"Advertencia: esta estimacion es una herramienta de apoyo. La procedencia del credito fiscal, notas de credito, proporcionalidad y otros antecedentes deben ser revisados antes de cualquier declaracion oficial.","","L",true)
	w.Header().Set("Content-Type","application/pdf");w.Header().Set("Content-Disposition",fmt.Sprintf(`attachment; filename="iva-estimado-%04d-%02d.pdf"`,anio,mes));if err:=pdf.Output(w);err!=nil{s.logger.Error("escribir reporte PDF","error",err)}
}

func (s *Servidor) datosEmpresa(r *http.Request,id string)(string,string,error){var nombre,rut string;err:=s.db.QueryRowContext(r.Context(),`SELECT razon_social,rut FROM spa.empresas WHERE empresa_id=$1 AND eliminado_en IS NULL`,id).Scan(&nombre,&rut);return nombre,rut,err}
func (s *Servidor) consultarIVA(r *http.Request,empresa string,inicio,fin time.Time)(string,string,int,error){var debito,credito string;var documentos int;err:=s.db.QueryRowContext(r.Context(),`SELECT COALESCE(sum(CASE WHEN direccion='venta' THEN CASE WHEN tipo_documento='nota_credito' THEN -monto_iva ELSE monto_iva END ELSE 0 END),0)::text,COALESCE(sum(CASE WHEN direccion='compra' THEN CASE WHEN tipo_documento='nota_credito' THEN -monto_iva ELSE monto_iva END ELSE 0 END),0)::text,count(*) FROM spa.documentos_tributarios WHERE empresa_id=$1 AND fecha_emision >= $2 AND fecha_emision < $3 AND eliminado_en IS NULL AND estado_documento<>'anulado'`,empresa,inicio,fin).Scan(&debito,&credito,&documentos);return debito,credito,documentos,err}
func rangoPeriodo(anio,mes int)(time.Time,time.Time){inicio:=time.Date(anio,time.January,1,0,0,0,0,time.UTC);fin:=inicio.AddDate(1,0,0);if mes>0{inicio=time.Date(anio,time.Month(mes),1,0,0,0,0,time.UTC);fin=inicio.AddDate(0,1,0)};return inicio,fin}
func etiquetaPeriodo(anio,mes int)string{if mes==0{return fmt.Sprintf("Anio comercial %d",anio)};meses:=[]string{"","enero","febrero","marzo","abril","mayo","junio","julio","agosto","septiembre","octubre","noviembre","diciembre"};return fmt.Sprintf("%s %d",meses[mes],anio)}
func formatearCLP(v string)string{d,err:=decimal.NewFromString(v);if err!=nil{return "$ "+strings.TrimSpace(v)};return "$ "+d.Round(0).StringFixed(0)}
func maximo(a,b int)int{if a>b{return a};return b}
