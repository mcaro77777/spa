# Despliegue en EasyPanel

MiSpA se despliega como **un solo servicio**. Go sirve la API y el build Angular incluido en `spa-api/ClientApp`.

## Fuente

- Tipo: GitHub
- Repositorio: `mcaro77777/spa`
- Rama: `main`
- Ruta de compilación: `/spa-api`
- Dockerfile: `Dockerfile`
- Puerto interno: `8080`

No cree un segundo servicio para `spa-web`: el frontend ya está integrado en la imagen de `spa-api`.

## Variables obligatorias

```text
APP_ENV=production
PORT=8080
APP_BASE_URL=https://DOMINIO_PUBLICO
DATABASE_URL=postgres://USUARIO:CLAVE@HOST:PUERTO/bd_inventario?sslmode=disable
JWT_SECRET=SECRETO_ALEATORIO_DE_AL_MENOS_32_BYTES
AI_PROVIDER=local
AI_TIMEOUT_SECONDS=30
```

El asistente local no requiere credenciales adicionales. Para habilitar el proveedor OpenAI configure también `OPENAI_API_KEY` y `OPENAI_MODEL` como variables de ejecución y cambie `AI_PROVIDER=openai`. El usuario deberá otorgar consentimiento explícito desde la interfaz antes de enviar agregados anonimizados.

`APP_BASE_URL` debe coincidir exactamente con el origen público, sin `/` final. En producción debe usar HTTPS, ya que la cookie de renovación se marca como `Secure`.

Si la contraseña de PostgreSQL contiene caracteres reservados como `@`, `:`, `/`, `?`, `#` o `%`, debe codificarse como URL antes de construir `DATABASE_URL`.

## Variables recomendadas

```text
JWT_ISSUER=spa-api
JWT_AUDIENCE=spa-web
ACCESS_TOKEN_MINUTES=15
REFRESH_TOKEN_DAYS=30
DB_MAX_OPEN_CONNS=20
MAX_UPLOAD_MB=10
UPLOAD_DIR=/data/uploads
CLIENT_APP_DIR=/app/ClientApp
```

## Persistencia

Agregue un volumen persistente:

```text
/data/uploads
```

Sin este volumen, los CSV y documentos originales se perderán al recrear el contenedor.

## Dominio y comprobaciones

- Habilite HTTPS para el dominio público.
- Configure el proxy hacia el puerto interno `8080`.
- Healthcheck: `GET /api/health`.
- El endpoint debe responder HTTP 200 con `ok: true` y `database: ok`.
- Abra `/acceso` y compruebe registro, login y renovación de sesión.

## Base de datos existente

El esquema `spa` ya debe existir en `bd_inventario`. La aplicación verifica su existencia al iniciar, pero no ejecuta migraciones destructivas automáticamente.

Desde el contenedor de EasyPanel, `HOST:PUERTO` debe ser una dirección alcanzable. Si la base se encuentra en otro proyecto o red de EasyPanel y no existe DNS interno compartido, use el host y puerto expuestos por el servicio PostgreSQL y restrinja el acceso mediante firewall.
