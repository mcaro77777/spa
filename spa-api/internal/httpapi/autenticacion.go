package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type credenciales struct { Correo string `json:"correo"`; Contrasena string `json:"contrasena"` }
type registroEntrada struct { Correo string `json:"correo"`; Contrasena string `json:"contrasena"`; Nombre string `json:"nombre"`; Apellido string `json:"apellido"` }
type seleccionarEmpresaEntrada struct { EmpresaID string `json:"empresaId"` }

func (s *Servidor) registrar(w http.ResponseWriter,r *http.Request){
	var e registroEntrada; if err:=decodificarJSON(w,r,&e);err!=nil{responderProblema(w,r,400,"json_invalido","Solicitud invalida",err.Error());return}
	e.Correo=strings.ToLower(strings.TrimSpace(e.Correo)); e.Nombre=strings.TrimSpace(e.Nombre); e.Apellido=strings.TrimSpace(e.Apellido)
	if !strings.Contains(e.Correo,"@") || e.Nombre=="" || len(e.Contrasena)<10 { responderProblema(w,r,422,"registro_invalido","Datos incompletos","Indica un correo valido, nombre y una contrasena de al menos 10 caracteres.");return }
	hash,err:=bcrypt.GenerateFromPassword([]byte(e.Contrasena),12);if err!=nil{s.errorInterno(w,r,err);return}
	var id string
	err=s.db.QueryRowContext(r.Context(),`INSERT INTO spa.usuarios(correo,nombre,apellido,hash_contrasena) VALUES($1,$2,NULLIF($3,''),$4) RETURNING usuario_id`,e.Correo,e.Nombre,e.Apellido,string(hash)).Scan(&id)
	if err!=nil{if esDuplicado(err){responderProblema(w,r,409,"correo_registrado","Correo ya registrado","Ya existe una cuenta activa para ese correo.");return};s.errorInterno(w,r,err);return}
	s.crearSesion(w,r,id,e.Correo,"")
}

func (s *Servidor) login(w http.ResponseWriter,r *http.Request){
	var e credenciales;if err:=decodificarJSON(w,r,&e);err!=nil{responderProblema(w,r,400,"json_invalido","Solicitud invalida",err.Error());return}
	var id,correo,hash,estado string
	err:=s.db.QueryRowContext(r.Context(),`SELECT usuario_id,correo,hash_contrasena,estado FROM spa.usuarios WHERE lower(correo)=lower($1) AND eliminado_en IS NULL`,strings.TrimSpace(e.Correo)).Scan(&id,&correo,&hash,&estado)
	if err!=nil || estado!="activo" || bcrypt.CompareHashAndPassword([]byte(hash),[]byte(e.Contrasena))!=nil { time.Sleep(150*time.Millisecond);responderProblema(w,r,401,"credenciales_invalidas","Credenciales invalidas","Correo o contrasena incorrectos.");return }
	_,_=s.db.ExecContext(r.Context(),`UPDATE spa.usuarios SET ultimo_acceso=CURRENT_TIMESTAMP WHERE usuario_id=$1`,id)
	s.crearSesion(w,r,id,correo,"")
}

func (s *Servidor) crearSesion(w http.ResponseWriter,r *http.Request,usuarioID,correo,empresaID string){
	refresh,hash,err:=nuevoTokenSeguro();if err!=nil{s.errorInterno(w,r,err);return}
	sesionID:=nuevoUUID(); familia:=nuevoUUID(); expira:=time.Now().UTC().Add(s.cfg.DuracionRefresh)
	tx,err:=s.db.BeginTx(r.Context(),nil);if err!=nil{s.errorInterno(w,r,err);return};defer tx.Rollback()
	var empresa any=nil;if empresaID!=""{empresa=empresaID}
	_,err=tx.ExecContext(r.Context(),`INSERT INTO spa.sesiones(sesion_id,usuario_id,empresa_activa_id,familia_token_id,direccion_ip,agente_usuario,fecha_expiracion) VALUES($1,$2,$3,$4,$5,$6,$7)`,sesionID,usuarioID,empresa,familia,ipCliente(r),r.UserAgent(),expira)
	if err==nil{_,err=tx.ExecContext(r.Context(),`INSERT INTO spa.tokens_actualizacion(sesion_id,token_hash,fecha_expiracion) VALUES($1,$2,$3)`,sesionID,hash,expira)}
	if err==nil{err=tx.Commit()};if err!=nil{s.errorInterno(w,r,err);return}
	s.establecerCookieRefresh(w,refresh,expira); responderJSON(w,200,map[string]any{"accessToken":s.generarJWT(sesionJWT{UsuarioID:usuarioID,SesionID:sesionID,EmpresaID:empresaID,Correo:correo}),"expiresIn":int(s.cfg.DuracionAccess.Seconds()),"empresaId":valorONil(empresaID)})
}

func (s *Servidor) refrescar(w http.ResponseWriter,r *http.Request){
	cookie,err:=r.Cookie("spa_refresh");if err!=nil||cookie.Value==""{responderProblema(w,r,401,"refresh_invalido","Sesion expirada","Vuelve a iniciar sesion.");return}
	hash:=hashToken(cookie.Value);var tokenID,sesionID,usuarioID,correo string;var empresa sql.NullString;var expira time.Time
	err=s.db.QueryRowContext(r.Context(),`SELECT t.token_actualizacion_id,s.sesion_id,s.usuario_id,u.correo,s.empresa_activa_id,t.fecha_expiracion FROM spa.tokens_actualizacion t JOIN spa.sesiones s ON s.sesion_id=t.sesion_id JOIN spa.usuarios u ON u.usuario_id=s.usuario_id WHERE t.token_hash=$1 AND t.fecha_revocacion IS NULL AND t.fecha_uso IS NULL AND s.fecha_revocacion IS NULL`,hash).Scan(&tokenID,&sesionID,&usuarioID,&correo,&empresa,&expira)
	if err!=nil||time.Now().After(expira){s.borrarCookieRefresh(w);responderProblema(w,r,401,"refresh_invalido","Sesion expirada","Vuelve a iniciar sesion.");return}
	nuevo,hashNuevo,err:=nuevoTokenSeguro();if err!=nil{s.errorInterno(w,r,err);return};nuevoID:=nuevoUUID();nuevaExp:=time.Now().UTC().Add(s.cfg.DuracionRefresh)
	tx,err:=s.db.BeginTx(r.Context(),nil);if err!=nil{s.errorInterno(w,r,err);return};defer tx.Rollback()
	_,err=tx.ExecContext(r.Context(),`INSERT INTO spa.tokens_actualizacion(token_actualizacion_id,sesion_id,token_hash,fecha_expiracion) VALUES($1,$2,$3,$4)`,nuevoID,sesionID,hashNuevo,nuevaExp)
	var res sql.Result
	if err==nil{res,err=tx.ExecContext(r.Context(),`UPDATE spa.tokens_actualizacion SET fecha_uso=CURRENT_TIMESTAMP,reemplazado_por_id=$1 WHERE token_actualizacion_id=$2 AND fecha_uso IS NULL`,nuevoID,tokenID)}
	if err==nil{n,_:=res.RowsAffected();if n!=1{err=errors.New("token ya utilizado")}}
	if err==nil{_,err=tx.ExecContext(r.Context(),`UPDATE spa.sesiones SET ultimo_uso=CURRENT_TIMESTAMP,fecha_expiracion=$2 WHERE sesion_id=$1`,sesionID,nuevaExp)}
	if err==nil{err=tx.Commit()};if err!=nil{s.borrarCookieRefresh(w);responderProblema(w,r,401,"refresh_reutilizado","Sesion revocada","Vuelve a iniciar sesion.");return}
	s.establecerCookieRefresh(w,nuevo,nuevaExp);cid:="";if empresa.Valid{cid=empresa.String};responderJSON(w,200,map[string]any{"accessToken":s.generarJWT(sesionJWT{UsuarioID:usuarioID,SesionID:sesionID,EmpresaID:cid,Correo:correo}),"expiresIn":int(s.cfg.DuracionAccess.Seconds()),"empresaId":valorONil(cid)})
}

func (s *Servidor) logout(w http.ResponseWriter,r *http.Request){
	if c,err:=r.Cookie("spa_refresh");err==nil{_,_=s.db.ExecContext(r.Context(),`UPDATE spa.sesiones SET fecha_revocacion=CURRENT_TIMESTAMP,motivo_revocacion='cierre de sesion' WHERE sesion_id=(SELECT sesion_id FROM spa.tokens_actualizacion WHERE token_hash=$1)`,hashToken(c.Value))}
	s.borrarCookieRefresh(w);w.WriteHeader(204)
}

func (s *Servidor) seleccionarEmpresa(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);var e seleccionarEmpresaEntrada;if err:=decodificarJSON(w,r,&e);err!=nil{responderProblema(w,r,400,"json_invalido","Solicitud invalida",err.Error());return}
	var existe bool;err:=s.db.QueryRowContext(r.Context(),`SELECT EXISTS(SELECT 1 FROM spa.usuarios_empresas WHERE empresa_id=$1 AND usuario_id=$2 AND estado='activo' AND eliminado_en IS NULL)`,e.EmpresaID,u.UsuarioID).Scan(&existe)
	if err!=nil||!existe{responderProblema(w,r,403,"sin_acceso_empresa","Acceso denegado","No tienes acceso activo a esa empresa.");return}
	res,err:=s.db.ExecContext(r.Context(),`UPDATE spa.sesiones SET empresa_activa_id=$1,ultimo_uso=CURRENT_TIMESTAMP WHERE sesion_id=$2 AND usuario_id=$3 AND fecha_revocacion IS NULL`,e.EmpresaID,u.SesionID,u.UsuarioID)
	if err!=nil{s.errorInterno(w,r,err);return};n,_:=res.RowsAffected();if n!=1{responderProblema(w,r,401,"sesion_invalida","Sesion invalida","");return}
	u.EmpresaID=e.EmpresaID;responderJSON(w,200,map[string]any{"accessToken":s.generarJWT(u),"expiresIn":int(s.cfg.DuracionAccess.Seconds()),"empresaId":e.EmpresaID})
}

func (s *Servidor) miSesion(w http.ResponseWriter,r *http.Request){
	u:=sesionActual(r);rows,err:=s.db.QueryContext(r.Context(),`SELECT e.empresa_id,e.razon_social,e.rut,ro.codigo FROM spa.usuarios_empresas ue JOIN spa.empresas e ON e.empresa_id=ue.empresa_id JOIN spa.roles ro ON ro.rol_id=ue.rol_id WHERE ue.usuario_id=$1 AND ue.estado='activo' AND ue.eliminado_en IS NULL AND e.eliminado_en IS NULL ORDER BY e.razon_social`,u.UsuarioID)
	if err!=nil{s.errorInterno(w,r,err);return};defer rows.Close();empresas:=[]map[string]any{}
	for rows.Next(){var id,nombre,rut,rol string;if rows.Scan(&id,&nombre,&rut,&rol)==nil{empresas=append(empresas,map[string]any{"id":id,"razonSocial":nombre,"rut":rut,"rol":rol})}}
	responderJSON(w,200,map[string]any{"usuario":map[string]any{"id":u.UsuarioID,"correo":u.Correo},"empresaActivaId":valorONil(u.EmpresaID),"empresas":empresas})
}

func (s *Servidor) establecerCookieRefresh(w http.ResponseWriter,token string,expira time.Time){http.SetCookie(w,&http.Cookie{Name:"spa_refresh",Value:token,Path:"/api/auth",HttpOnly:true,Secure:s.cfg.Produccion(),SameSite:http.SameSiteStrictMode,Expires:expira,MaxAge:int(time.Until(expira).Seconds())})}
func (s *Servidor) borrarCookieRefresh(w http.ResponseWriter){http.SetCookie(w,&http.Cookie{Name:"spa_refresh",Value:"",Path:"/api/auth",HttpOnly:true,Secure:s.cfg.Produccion(),SameSite:http.SameSiteStrictMode,Expires:time.Unix(0,0),MaxAge:-1})}
func nuevoTokenSeguro()(string,string,error){b:=make([]byte,32);if _,err:=rand.Read(b);err!=nil{return "","",err};t:=base64.RawURLEncoding.EncodeToString(b);return t,hashToken(t),nil}
func hashToken(v string)string{h:=sha256.Sum256([]byte(v));return hex.EncodeToString(h[:])}
func nuevoUUID()string{b:=make([]byte,16);_,_=rand.Read(b);b[6]=(b[6]&0x0f)|0x40;b[8]=(b[8]&0x3f)|0x80;return hex.EncodeToString(b[0:4])+"-"+hex.EncodeToString(b[4:6])+"-"+hex.EncodeToString(b[6:8])+"-"+hex.EncodeToString(b[8:10])+"-"+hex.EncodeToString(b[10:16])}
func ipCliente(r *http.Request)string{if v:=strings.TrimSpace(strings.Split(r.RemoteAddr,":")[0]);v!=""{return v};return "127.0.0.1"}
func valorONil(v string)any{if v==""{return nil};return v}
func esDuplicado(err error)bool{return err!=nil&&strings.Contains(strings.ToLower(err.Error()),"duplicate key")}
func (s *Servidor) errorInterno(w http.ResponseWriter,r *http.Request,err error){s.logger.Error("error interno","error",err,"ruta",r.URL.Path);responderProblema(w,r,500,"error_interno","Error interno","No fue posible completar la operacion.")}
