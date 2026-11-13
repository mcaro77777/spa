# MiSpA IA

MVP de una plataforma de apoyo tributario para microempresas chilenas.

## Estructura

- `spa-api`: API Go 1.24, PostgreSQL, importaciones y servidor del frontend.
- `spa-web`: Angular 20 standalone, zoneless y responsive.
- `inventario-api` e `inventario-web`: referencias de arquitectura; no forman parte del producto.

## Inicio rapido con Docker

Compile Angular y copie el resultado a la API:

```powershell
cd spa-web
npm ci
npm run build
cd ..\spa-api
.\scripts\publicar-angular.ps1
```

Luego, desde la raiz:

```powershell
$env:JWT_SECRET = 'reemplace-por-un-secreto-de-al-menos-32-caracteres'
$env:POSTGRES_PASSWORD = 'reemplace-esta-clave'
docker compose up --build
```

Abra `http://localhost:8080`. En la primera inicializacion PostgreSQL aplica el esquema `spa` automaticamente.

## Estado del producto

Las fases 1 a 4 incluyen registro, sesión segura, empresas, aislamiento por membresía, documentos e IVA estimado, cuentas bancarias y conciliación; además incorporan una proyección Pro Pyme General trazable y un asistente de explicación con herramientas internas de solo lectura.

Formato de cartola canónico: `fecha`, `descripcion` (también `glosa`), `referencia`, `monto` y `saldo` opcional. Se incluye una plantilla en `spa-api/testdata/cartola_ejemplo.csv`.

Los cálculos y clasificaciones son informativos y no reemplazan al SII ni la revisión de un contador.

## Motor tributario y asistente

Las reglas IDPC/PPM para años comerciales 2025–2028 se registran con versión, fuente oficial y fecha de consulta. Los cálculos no inventan reglas para períodos sin respaldo y marcan RAI/SAC como incompletos hasta contar con saldos de apertura y revisión profesional.

El asistente usa por defecto `AI_PROVIDER=local`. Para habilitar OpenAI Responses API configure `AI_PROVIDER=openai`, `OPENAI_API_KEY` y `OPENAI_MODEL`. Solo se envían agregados anonimizados cuando el usuario activa el consentimiento; las solicitudes usan `store: false`.
