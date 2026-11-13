package configuracion

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Configuracion struct {
	Entorno, Puerto, URLBase, DSN, JWTSecret, JWTIssuer, JWTAudience string
	DirectorioCliente, DirectorioUploads                             string
	ProveedorIA, OpenAIAPIKey, OpenAIModelo                          string
	DuracionAccess, DuracionRefresh                                  time.Duration
	MaxArchivoBytes, MaxConexiones, TimeoutIASegundos                int
}

func Cargar() (Configuracion, error) {
	c := Configuracion{
		Entorno: valor("APP_ENV", "development"), Puerto: valor("PORT", "8080"),
		URLBase: valor("APP_BASE_URL", "http://localhost:8080"), DSN: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret: os.Getenv("JWT_SECRET"), JWTIssuer: valor("JWT_ISSUER", "spa-api"), JWTAudience: valor("JWT_AUDIENCE", "spa-web"),
		DirectorioCliente: valor("CLIENT_APP_DIR", "ClientApp"), DirectorioUploads: valor("UPLOAD_DIR", ".dev-uploads"),
		ProveedorIA: strings.ToLower(valor("AI_PROVIDER", "local")), OpenAIAPIKey: strings.TrimSpace(os.Getenv("OPENAI_API_KEY")), OpenAIModelo: strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
	}
	var err error
	c.MaxConexiones, err = entero("DB_MAX_OPEN_CONNS", 20)
	if err != nil {
		return c, err
	}
	c.MaxArchivoBytes, err = entero("MAX_UPLOAD_MB", 10)
	if err != nil {
		return c, err
	}
	c.MaxArchivoBytes *= 1024 * 1024
	c.TimeoutIASegundos, err = entero("AI_TIMEOUT_SECONDS", 30)
	if err != nil {
		return c, err
	}
	minutos, err := entero("ACCESS_TOKEN_MINUTES", 15)
	if err != nil {
		return c, err
	}
	dias, err := entero("REFRESH_TOKEN_DAYS", 30)
	if err != nil {
		return c, err
	}
	c.DuracionAccess = time.Duration(minutos) * time.Minute
	c.DuracionRefresh = time.Duration(dias) * 24 * time.Hour
	if c.DSN == "" {
		return c, errors.New("DATABASE_URL es obligatoria")
	}
	if len(c.JWTSecret) < 32 {
		return c, errors.New("JWT_SECRET debe tener al menos 32 caracteres")
	}
	if c.ProveedorIA != "local" && c.ProveedorIA != "openai" {
		return c, errors.New("AI_PROVIDER debe ser local u openai")
	}
	if c.ProveedorIA == "openai" && (c.OpenAIAPIKey == "" || c.OpenAIModelo == "") {
		return c, errors.New("OPENAI_API_KEY y OPENAI_MODEL son obligatorios cuando AI_PROVIDER=openai")
	}
	u, err := url.ParseRequestURI(c.URLBase)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return c, errors.New("APP_BASE_URL debe ser una URL valida")
	}
	return c, nil
}

func (c Configuracion) Produccion() bool { return strings.EqualFold(c.Entorno, "production") }
func valor(nombre, defecto string) string {
	if v := strings.TrimSpace(os.Getenv(nombre)); v != "" {
		return v
	}
	return defecto
}
func entero(nombre string, defecto int) (int, error) {
	n, err := strconv.Atoi(valor(nombre, strconv.Itoa(defecto)))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s debe ser un entero positivo", nombre)
	}
	return n, nil
}
