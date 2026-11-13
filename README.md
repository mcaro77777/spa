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

## Estado del MVP

Incluye registro, login, refresh token rotatorio, empresas, aislamiento por membresia, importacion CSV idempotente, documentos, dashboard e IVA estimado. Los calculos son informativos y no reemplazan al SII ni la revision de un contador.
