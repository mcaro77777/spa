# MiSpA API

API del MVP de MiSpA IA en Go 1.24, `net/http`, PostgreSQL y SQL explicito.

## Configuracion

Ejecute `scripts/crear_esquema_spa.sql` en PostgreSQL, copie `.env.example` y defina las variables en el entorno. La aplicacion no carga archivos `.env` automaticamente.

```bash
go test ./...
go vet ./...
go run ./cmd/api check-db
go run ./cmd/api serve
```

La API escucha en `http://localhost:8080`. Angular usa `/api` y en produccion puede compilarse dentro de `ClientApp` para operar bajo el mismo origen.

## CSV de documentos

La plantilla se encuentra en `testdata/documentos_ejemplo.csv`. El RUT de la empresa del ejemplo es `76.086.428-5`; reemplazelo por el RUT de la empresa registrada antes de importar.

Los montos se procesan con decimal exacto y los errores se registran por fila. Reimportar el mismo archivo no duplica documentos.

## CSV de cartolas bancarias

La plantilla `testdata/cartola_ejemplo.csv` admite separador coma o punto y coma. Las columnas mínimas son `fecha`, `descripcion`/`glosa` y `monto`; `referencia` y `saldo` son opcionales. Las fechas admiten `YYYY-MM-DD`, `DD-MM-YYYY` o `DD/MM/YYYY` y los montos pueden usar formato chileno.

Cada movimiento se identifica mediante un hash estable por cuenta. La conciliación valida empresa, dirección, saldo del movimiento y saldo pendiente del documento dentro de una transacción SQL.
