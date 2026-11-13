package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type claveContexto string
const claveSesion claveContexto = "sesion"

type sesionJWT struct { UsuarioID string; SesionID string; EmpresaID string; Correo string }

type jwtPayload struct {
	Sub string `json:"sub"`; SID string `json:"sid"`; CID string `json:"cid,omitempty"`; Email string `json:"email"`
	Iss string `json:"iss"`; Aud string `json:"aud"`; Iat int64 `json:"iat"`; Exp int64 `json:"exp"`
}

func (s *Servidor) generarJWT(u sesionJWT) string {
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	ahora := time.Now().UTC()
	header := enc(map[string]string{"alg":"HS256","typ":"JWT"})
	payload := enc(jwtPayload{Sub:u.UsuarioID,SID:u.SesionID,CID:u.EmpresaID,Email:u.Correo,Iss:s.cfg.JWTIssuer,Aud:s.cfg.JWTAudience,Iat:ahora.Unix(),Exp:ahora.Add(s.cfg.DuracionAccess).Unix()})
	entrada := header+"."+payload
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret)); _, _ = mac.Write([]byte(entrada))
	return entrada+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Servidor) validarJWT(token string) (sesionJWT, bool) {
	var vacio sesionJWT; partes := strings.Split(token, "."); if len(partes) != 3 { return vacio, false }
	entrada := partes[0]+"."+partes[1]; firma, err := base64.RawURLEncoding.DecodeString(partes[2]); if err != nil { return vacio, false }
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret)); _, _ = mac.Write([]byte(entrada)); if !hmac.Equal(firma, mac.Sum(nil)) { return vacio, false }
	b, err := base64.RawURLEncoding.DecodeString(partes[1]); if err != nil { return vacio, false }
	var p jwtPayload
	if json.Unmarshal(b,&p) != nil || p.Exp <= time.Now().Unix() || p.Iss != s.cfg.JWTIssuer || p.Aud != s.cfg.JWTAudience || p.Sub == "" || p.SID == "" { return vacio, false }
	return sesionJWT{UsuarioID:p.Sub,SesionID:p.SID,EmpresaID:p.CID,Correo:p.Email}, true
}

func (s *Servidor) requerirSesion(empresa bool, siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.validarJWT(bearer(r)); if !ok { responderProblema(w,r,401,"sesion_invalida","No autorizado","La sesion no existe o expiro."); return }
		if empresa && u.EmpresaID == "" { responderProblema(w,r,409,"empresa_no_seleccionada","Empresa no seleccionada","Selecciona una empresa para continuar."); return }
		if empresa {
			var activo bool
			err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM spa.usuarios_empresas WHERE empresa_id=$1 AND usuario_id=$2 AND estado='activo' AND eliminado_en IS NULL)`,u.EmpresaID,u.UsuarioID).Scan(&activo)
			if err != nil || !activo { responderProblema(w,r,403,"sin_acceso_empresa","Acceso denegado","La membresia no esta activa."); return }
		}
		siguiente.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),claveSesion,u)))
	})
}

func sesionActual(r *http.Request) sesionJWT { u,_ := r.Context().Value(claveSesion).(sesionJWT); return u }
func bearer(r *http.Request) string { p:=strings.SplitN(strings.TrimSpace(r.Header.Get("Authorization"))," ",2); if len(p)==2 && strings.EqualFold(p[0],"Bearer") { return p[1] }; return "" }

func middlewareBase(logger *slog.Logger, urlBase string, desarrollo bool, siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		inicio:=time.Now(); origen:=r.Header.Get("Origin"); permitido:=origenPermitido(origen,r.Host,urlBase,desarrollo)
		w.Header().Set("X-Content-Type-Options","nosniff"); w.Header().Set("Referrer-Policy","same-origin"); w.Header().Set("X-Frame-Options","DENY")
		if origen!="" && permitido { w.Header().Set("Access-Control-Allow-Origin",origen); w.Header().Set("Vary","Origin"); w.Header().Set("Access-Control-Allow-Credentials","true"); w.Header().Set("Access-Control-Allow-Headers","Authorization, Content-Type, Idempotency-Key"); w.Header().Set("Access-Control-Allow-Methods","GET, POST, PATCH, DELETE, OPTIONS") }
		if r.Method==http.MethodOptions { if !permitido { responderProblema(w,r,403,"origen_no_permitido","Origen no permitido",""); return }; w.WriteHeader(204); return }
		if !permitido { responderProblema(w,r,403,"origen_no_permitido","Origen no permitido",""); return }
		siguiente.ServeHTTP(w,r); logger.Info("solicitud HTTP","metodo",r.Method,"ruta",r.URL.Path,"duracion",time.Since(inicio))
	})
}

func origenPermitido(origen,host,urlBase string,desarrollo bool) bool {
	if origen=="" || desarrollo || strings.EqualFold(strings.TrimRight(origen,"/"),strings.TrimRight(urlBase,"/")) { return true }
	u,err:=url.ParseRequestURI(origen); return err==nil && u.Scheme!="" && strings.EqualFold(u.Host,host)
}

