package httpapi

import (
	"testing"
	"time"

	"spa-api/internal/configuracion"
)

func servidorPrueba() *Servidor {
	return &Servidor{cfg: configuracion.Configuracion{JWTSecret:"01234567890123456789012345678901",JWTIssuer:"spa-api",JWTAudience:"spa-web",DuracionAccess:15*time.Minute}}
}

func TestRUT(t *testing.T){
	casos:=map[string]bool{"76.086.428-5":true,"76192083-9":true,"12.345.678-5":true,"12.345.678-9":false,"":false}
	for entrada,esperado:=range casos{if obtenido:=rutValido(normalizarRUT(entrada));obtenido!=esperado{t.Errorf("rutValido(%q)=%v; esperado %v",entrada,obtenido,esperado)}}
}

func TestDecimalCLP(t *testing.T){
	casos:=map[string]string{"600000":"600000","600.50":"600.5","600.000,50":"600000.5","$ 1.234,5":"1234.5"}
	for entrada,esperado:=range casos{d,err:=decimalCLP(entrada);if err!=nil{t.Fatalf("decimalCLP(%q): %v",entrada,err)};if d.String()!=esperado{t.Errorf("decimalCLP(%q)=%s; esperado %s",entrada,d.String(),esperado)}}
	if _,err:=decimalCLP("-1");err==nil{t.Error("se acepto un monto negativo")}
}

func TestJWTFirmado(t *testing.T){
	s:=servidorPrueba();original:=sesionJWT{UsuarioID:nuevoUUID(),SesionID:nuevoUUID(),EmpresaID:nuevoUUID(),Correo:"persona@ejemplo.cl"};token:=s.generarJWT(original);obtenido,ok:=s.validarJWT(token)
	if !ok||obtenido.UsuarioID!=original.UsuarioID||obtenido.EmpresaID!=original.EmpresaID{t.Fatalf("JWT invalido: %#v, %v",obtenido,ok)}
	if _,ok:=s.validarJWT(token+"alterado");ok{t.Error("se acepto un JWT alterado")}
}
