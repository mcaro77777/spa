package httpapi

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode"
)

type empresaEntrada struct {
	RazonSocial string `json:"razonSocial"`; NombreFantasia string `json:"nombreFantasia"`; RUT string `json:"rut"`
	Giro string `json:"giro"`; Correo string `json:"correo"`; Telefono string `json:"telefono"`; Direccion string `json:"direccion"`
	Comuna string `json:"comuna"`; Region string `json:"region"`; RegimenCodigo string `json:"regimenCodigo"`
}

func (s *Servidor) empresas(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r)
	if r.Method==http.MethodGet{
		rows,err:=s.db.QueryContext(r.Context(),`SELECT e.empresa_id,e.razon_social,e.nombre_fantasia,e.rut,e.estado,ro.codigo FROM spa.usuarios_empresas ue JOIN spa.empresas e ON e.empresa_id=ue.empresa_id JOIN spa.roles ro ON ro.rol_id=ue.rol_id WHERE ue.usuario_id=$1 AND ue.estado='activo' AND ue.eliminado_en IS NULL AND e.eliminado_en IS NULL ORDER BY e.razon_social`,u.UsuarioID)
		if err!=nil{s.errorInterno(w,r,err);return};defer rows.Close();salida:=[]map[string]any{}
		for rows.Next(){var id,razon,rut,estado,rol string;var fantasia sql.NullString;if err:=rows.Scan(&id,&razon,&fantasia,&rut,&estado,&rol);err!=nil{s.errorInterno(w,r,err);return};salida=append(salida,map[string]any{"id":id,"razonSocial":razon,"nombreFantasia":nuloString(fantasia),"rut":rut,"estado":estado,"rol":rol})}
		responderJSON(w,200,salida);return
	}
	var e empresaEntrada;if err:=decodificarJSON(w,r,&e);err!=nil{responderProblema(w,r,400,"json_invalido","Solicitud invalida",err.Error());return}
	e.RazonSocial=strings.TrimSpace(e.RazonSocial);rut:=normalizarRUT(e.RUT)
	if e.RazonSocial==""||!rutValido(rut){responderProblema(w,r,422,"empresa_invalida","Datos de empresa invalidos","Indica razon social y un RUT chileno valido.");return}
	if e.RegimenCodigo==""{e.RegimenCodigo="pro_pyme_general_14d3"}
	tx,err:=s.db.BeginTx(r.Context(),nil);if err!=nil{s.errorInterno(w,r,err);return};defer tx.Rollback();var empresaID string
	err=tx.QueryRowContext(r.Context(),`INSERT INTO spa.empresas(razon_social,nombre_fantasia,rut,rut_normalizado,giro,correo,telefono,direccion,comuna,region,regimen_tributario_id) SELECT $1,NULLIF($2,''),$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),regimen_tributario_id FROM spa.regimenes_tributarios WHERE codigo=$11 AND activo=true RETURNING empresa_id`,e.RazonSocial,e.NombreFantasia,e.RUT,rut,e.Giro,e.Correo,e.Telefono,e.Direccion,e.Comuna,e.Region,e.RegimenCodigo).Scan(&empresaID)
	if err==nil{_,err=tx.ExecContext(r.Context(),`INSERT INTO spa.usuarios_empresas(empresa_id,usuario_id,rol_id) SELECT $1,$2,rol_id FROM spa.roles WHERE codigo='propietario'`,empresaID,u.UsuarioID)}
	if err==nil{err=tx.Commit()};if err!=nil{if esDuplicado(err){responderProblema(w,r,409,"rut_registrado","Empresa ya registrada","Ya existe una empresa activa con ese RUT.");return};s.errorInterno(w,r,err);return}
	responderJSON(w,201,map[string]any{"id":empresaID,"razonSocial":e.RazonSocial,"rut":e.RUT})
}

func (s *Servidor) empresaDetalle(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);id:=r.PathValue("id");if id!=u.EmpresaID{responderProblema(w,r,404,"empresa_no_encontrada","Empresa no encontrada","");return}
	if r.Method==http.MethodGet{
		var x struct{ID,Razon,RUT,RUTN,Estado string;Fantasia,Giro,Correo,Telefono,Direccion,Comuna,Region,Regimen sql.NullString}
		err:=s.db.QueryRowContext(r.Context(),`SELECT e.empresa_id,e.razon_social,e.nombre_fantasia,e.rut,e.rut_normalizado,e.giro,e.correo,e.telefono,e.direccion,e.comuna,e.region,e.estado,rt.codigo FROM spa.empresas e LEFT JOIN spa.regimenes_tributarios rt ON rt.regimen_tributario_id=e.regimen_tributario_id WHERE e.empresa_id=$1 AND e.eliminado_en IS NULL`,id).Scan(&x.ID,&x.Razon,&x.Fantasia,&x.RUT,&x.RUTN,&x.Giro,&x.Correo,&x.Telefono,&x.Direccion,&x.Comuna,&x.Region,&x.Estado,&x.Regimen)
		if err==sql.ErrNoRows{responderProblema(w,r,404,"empresa_no_encontrada","Empresa no encontrada","");return};if err!=nil{s.errorInterno(w,r,err);return}
		responderJSON(w,200,map[string]any{"id":x.ID,"razonSocial":x.Razon,"nombreFantasia":nuloString(x.Fantasia),"rut":x.RUT,"rutNormalizado":x.RUTN,"giro":nuloString(x.Giro),"correo":nuloString(x.Correo),"telefono":nuloString(x.Telefono),"direccion":nuloString(x.Direccion),"comuna":nuloString(x.Comuna),"region":nuloString(x.Region),"estado":x.Estado,"regimenCodigo":nuloString(x.Regimen)});return
	}
	var e empresaEntrada;if err:=decodificarJSON(w,r,&e);err!=nil{responderProblema(w,r,400,"json_invalido","Solicitud invalida",err.Error());return};if strings.TrimSpace(e.RazonSocial)==""{responderProblema(w,r,422,"empresa_invalida","Razon social requerida","");return}
	_,err:=s.db.ExecContext(r.Context(),`UPDATE spa.empresas SET razon_social=$1,nombre_fantasia=NULLIF($2,''),giro=NULLIF($3,''),correo=NULLIF($4,''),telefono=NULLIF($5,''),direccion=NULLIF($6,''),comuna=NULLIF($7,''),region=NULLIF($8,'') WHERE empresa_id=$9`,e.RazonSocial,e.NombreFantasia,e.Giro,e.Correo,e.Telefono,e.Direccion,e.Comuna,e.Region,id)
	if err!=nil{s.errorInterno(w,r,err);return};w.WriteHeader(204)
}

func normalizarRUT(v string)string{var b strings.Builder;for _,r:=range strings.ToUpper(v){if unicode.IsDigit(r)||r=='K'{b.WriteRune(r)}};return b.String()}
func rutValido(rut string)bool{if len(rut)<8||len(rut)>9{return false};cuerpo:=rut[:len(rut)-1];suma,mul:=0,2;for i:=len(cuerpo)-1;i>=0;i--{d:=int(cuerpo[i]-'0');if d<0||d>9{return false};suma+=d*mul;mul++;if mul==8{mul=2}};resto:=11-(suma%11);dv:="0";if resto==10{dv="K"}else if resto<10{dv=string(rune('0'+resto))};return rut[len(rut)-1:]==dv}
func nuloString(v sql.NullString)any{if v.Valid{return v.String};return nil}
