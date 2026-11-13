/*
===============================================================================
 MiSpA IA - Esquema inicial PostgreSQL
 Compatible con PostgreSQL 16 y 17
 Listo para ejecutar en DBeaver sobre la base de datos seleccionada.
===============================================================================

IMPORTANTE SOBRE LA CREACION DE LA BASE DE DATOS
------------------------------------------------
DBeaver no cambia automaticamente de conexion despues de CREATE DATABASE.

Si deseas una base nueva:
  1. Abre un editor conectado a la base administrativa "postgres".
  2. Activa Auto-commit.
  3. Ejecuta solamente:

       CREATE DATABASE bd_spa
         WITH ENCODING = 'UTF8'
              TEMPLATE = template0;

  4. Crea/abre una conexion a bd_spa.
  5. Ejecuta desde BEGIN hasta COMMIT de este archivo.

Si utilizaras la base que ya aparece conectada en DBeaver, no ejecutes CREATE
DATABASE: pega y ejecuta directamente este script. Todos los objetos quedaran
aislados dentro del esquema "spa" y no tocaran el esquema "public".

El script es idempotente para la estructura y los catalogos iniciales. No borra
tablas ni datos existentes.
===============================================================================
*/

BEGIN;

CREATE SCHEMA IF NOT EXISTS spa;
COMMENT ON SCHEMA spa IS 'Objetos de la plataforma tributaria MiSpA IA';

SET LOCAL search_path TO spa, public;

-- =============================================================================
-- FUNCIONES COMUNES
-- =============================================================================

CREATE OR REPLACE FUNCTION spa.actualizar_fecha_modificacion()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.fecha_modificacion = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

COMMENT ON FUNCTION spa.actualizar_fecha_modificacion()
IS 'Actualiza automaticamente fecha_modificacion antes de cada UPDATE';

-- =============================================================================
-- SEGURIDAD, USUARIOS Y EMPRESAS
-- =============================================================================

CREATE TABLE IF NOT EXISTS spa.usuarios (
    usuario_id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    correo                  varchar(320) NOT NULL,
    nombre                  varchar(120) NOT NULL,
    apellido                varchar(120),
    rut                     varchar(20),
    rut_normalizado         varchar(9),
    hash_contrasena         text NOT NULL,
    estado                  varchar(20) NOT NULL DEFAULT 'activo',
    correo_verificado       boolean NOT NULL DEFAULT false,
    fecha_verificacion      timestamptz,
    ultimo_acceso           timestamptz,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT ck_usuarios_correo_no_vacio CHECK (btrim(correo) <> ''),
    CONSTRAINT ck_usuarios_nombre_no_vacio CHECK (btrim(nombre) <> ''),
    CONSTRAINT ck_usuarios_estado CHECK (estado IN ('pendiente', 'activo', 'bloqueado', 'inactivo')),
    CONSTRAINT ck_usuarios_rut_normalizado CHECK (
        rut_normalizado IS NULL OR rut_normalizado ~ '^[0-9]{7,8}[0-9K]$'
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_usuarios_correo_activo
    ON spa.usuarios (lower(correo))
    WHERE eliminado_en IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_usuarios_rut_activo
    ON spa.usuarios (rut_normalizado)
    WHERE rut_normalizado IS NOT NULL AND eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_usuarios_estado
    ON spa.usuarios (estado)
    WHERE eliminado_en IS NULL;

COMMENT ON TABLE spa.usuarios IS 'Cuentas de usuario globales de la plataforma';
COMMENT ON COLUMN spa.usuarios.hash_contrasena IS 'Hash Argon2id o bcrypt; nunca una contrasena en texto plano';
COMMENT ON COLUMN spa.usuarios.rut_normalizado IS 'RUT sin puntos ni guion y con digito verificador en mayuscula';

CREATE TABLE IF NOT EXISTS spa.roles (
    rol_id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo                  varchar(50) NOT NULL UNIQUE,
    nombre                  varchar(100) NOT NULL,
    descripcion             text,
    es_sistema              boolean NOT NULL DEFAULT true,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_roles_codigo CHECK (codigo ~ '^[a-z][a-z0-9_]*$')
);

CREATE TABLE IF NOT EXISTS spa.permisos (
    permiso_id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo                  varchar(100) NOT NULL UNIQUE,
    nombre                  varchar(150) NOT NULL,
    descripcion             text,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_permisos_codigo CHECK (codigo ~ '^[a-z][a-z0-9_.]*$')
);

CREATE TABLE IF NOT EXISTS spa.roles_permisos (
    rol_id                  uuid NOT NULL REFERENCES spa.roles (rol_id) ON DELETE CASCADE,
    permiso_id              uuid NOT NULL REFERENCES spa.permisos (permiso_id) ON DELETE CASCADE,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (rol_id, permiso_id)
);

CREATE TABLE IF NOT EXISTS spa.regimenes_tributarios (
    regimen_tributario_id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo                  varchar(50) NOT NULL UNIQUE,
    nombre                  varchar(200) NOT NULL,
    descripcion             text,
    referencia_normativa    text,
    activo                  boolean NOT NULL DEFAULT true,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS spa.empresas (
    empresa_id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    razon_social                varchar(250) NOT NULL,
    nombre_fantasia             varchar(250),
    rut                         varchar(20) NOT NULL,
    rut_normalizado             varchar(9) NOT NULL,
    giro                        varchar(300),
    correo                      varchar(320),
    telefono                    varchar(50),
    direccion                   varchar(300),
    comuna                      varchar(120),
    region                      varchar(120),
    regimen_tributario_id       uuid REFERENCES spa.regimenes_tributarios (regimen_tributario_id),
    fecha_inicio_actividades    date,
    zona_horaria                varchar(80) NOT NULL DEFAULT 'America/Santiago',
    moneda                      char(3) NOT NULL DEFAULT 'CLP',
    estado                      varchar(20) NOT NULL DEFAULT 'activa',
    fecha_creacion              timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en                timestamptz,
    CONSTRAINT ck_empresas_razon_social CHECK (btrim(razon_social) <> ''),
    CONSTRAINT ck_empresas_rut_normalizado CHECK (rut_normalizado ~ '^[0-9]{7,8}[0-9K]$'),
    CONSTRAINT ck_empresas_estado CHECK (estado IN ('activa', 'suspendida', 'inactiva')),
    CONSTRAINT ck_empresas_moneda CHECK (moneda ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_empresas_rut_activo
    ON spa.empresas (rut_normalizado)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_empresas_regimen
    ON spa.empresas (regimen_tributario_id)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.usuarios_empresas (
    usuario_empresa_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    usuario_id              uuid NOT NULL REFERENCES spa.usuarios (usuario_id) ON DELETE CASCADE,
    rol_id                  uuid NOT NULL REFERENCES spa.roles (rol_id),
    estado                  varchar(20) NOT NULL DEFAULT 'activo',
    fecha_ingreso           timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT ck_usuarios_empresas_estado CHECK (estado IN ('pendiente', 'activo', 'suspendido', 'revocado')),
    CONSTRAINT uq_usuarios_empresas_id_empresa UNIQUE (empresa_id, usuario_empresa_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_usuarios_empresas_membresia_activa
    ON spa.usuarios_empresas (empresa_id, usuario_id)
    WHERE eliminado_en IS NULL AND estado <> 'revocado';

CREATE INDEX IF NOT EXISTS ix_usuarios_empresas_usuario
    ON spa.usuarios_empresas (usuario_id, estado)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_usuarios_empresas_empresa_rol
    ON spa.usuarios_empresas (empresa_id, rol_id, estado)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.invitaciones_empresa (
    invitacion_id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    correo_invitado         varchar(320) NOT NULL,
    rol_id                  uuid NOT NULL REFERENCES spa.roles (rol_id),
    token_hash              varchar(128) NOT NULL UNIQUE,
    estado                  varchar(20) NOT NULL DEFAULT 'pendiente',
    invitado_por            uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_expiracion        timestamptz NOT NULL,
    fecha_aceptacion        timestamptz,
    usuario_aceptante_id    uuid REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_invitaciones_estado CHECK (estado IN ('pendiente', 'aceptada', 'revocada', 'expirada')),
    CONSTRAINT ck_invitaciones_expiracion CHECK (fecha_expiracion > fecha_creacion)
);

CREATE INDEX IF NOT EXISTS ix_invitaciones_empresa_estado
    ON spa.invitaciones_empresa (empresa_id, estado, fecha_expiracion);

CREATE INDEX IF NOT EXISTS ix_invitaciones_correo
    ON spa.invitaciones_empresa (lower(correo_invitado), estado);

CREATE TABLE IF NOT EXISTS spa.sesiones (
    sesion_id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id              uuid NOT NULL REFERENCES spa.usuarios (usuario_id) ON DELETE CASCADE,
    empresa_activa_id       uuid REFERENCES spa.empresas (empresa_id),
    familia_token_id        uuid NOT NULL DEFAULT gen_random_uuid(),
    direccion_ip            inet,
    agente_usuario          text,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ultimo_uso              timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_expiracion        timestamptz NOT NULL,
    fecha_revocacion        timestamptz,
    motivo_revocacion       varchar(200),
    CONSTRAINT ck_sesiones_expiracion CHECK (fecha_expiracion > fecha_creacion)
);

CREATE INDEX IF NOT EXISTS ix_sesiones_usuario_activas
    ON spa.sesiones (usuario_id, fecha_expiracion)
    WHERE fecha_revocacion IS NULL;

CREATE INDEX IF NOT EXISTS ix_sesiones_familia
    ON spa.sesiones (familia_token_id);

CREATE TABLE IF NOT EXISTS spa.tokens_actualizacion (
    token_actualizacion_id  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sesion_id               uuid NOT NULL REFERENCES spa.sesiones (sesion_id) ON DELETE CASCADE,
    token_hash              varchar(128) NOT NULL UNIQUE,
    reemplazado_por_id      uuid REFERENCES spa.tokens_actualizacion (token_actualizacion_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_expiracion        timestamptz NOT NULL,
    fecha_uso               timestamptz,
    fecha_revocacion        timestamptz,
    CONSTRAINT ck_tokens_expiracion CHECK (fecha_expiracion > fecha_creacion)
);

CREATE INDEX IF NOT EXISTS ix_tokens_actualizacion_sesion
    ON spa.tokens_actualizacion (sesion_id, fecha_expiracion)
    WHERE fecha_revocacion IS NULL;

-- =============================================================================
-- PERIODOS Y REGLAS TRIBUTARIAS VERSIONADAS
-- =============================================================================

CREATE TABLE IF NOT EXISTS spa.anios_tributarios (
    anio_tributario_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    anio_comercial          smallint NOT NULL,
    anio_tributario         smallint NOT NULL,
    fecha_inicio            date NOT NULL,
    fecha_fin               date NOT NULL,
    estado                  varchar(20) NOT NULL DEFAULT 'borrador',
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_anios_tributarios UNIQUE (anio_comercial, anio_tributario),
    CONSTRAINT ck_anios_rango CHECK (anio_tributario = anio_comercial + 1),
    CONSTRAINT ck_anios_fechas CHECK (fecha_fin >= fecha_inicio),
    CONSTRAINT ck_anios_estado CHECK (estado IN ('borrador', 'vigente', 'cerrado'))
);

CREATE TABLE IF NOT EXISTS spa.reglas_tributarias (
    regla_tributaria_id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    regimen_tributario_id   uuid REFERENCES spa.regimenes_tributarios (regimen_tributario_id),
    anio_tributario_id      uuid NOT NULL REFERENCES spa.anios_tributarios (anio_tributario_id),
    codigo                  varchar(100) NOT NULL,
    nombre                  varchar(250) NOT NULL,
    tipo                    varchar(40) NOT NULL,
    valor_decimal           numeric(30,10),
    valor_texto             text,
    configuracion           jsonb NOT NULL DEFAULT '{}'::jsonb,
    vigente_desde           date NOT NULL,
    vigente_hasta           date,
    fuente_oficial          text NOT NULL,
    fecha_consulta_fuente   date NOT NULL,
    version                 integer NOT NULL DEFAULT 1,
    estado                  varchar(20) NOT NULL DEFAULT 'borrador',
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_reglas_version UNIQUE (codigo, anio_tributario_id, regimen_tributario_id, version),
    CONSTRAINT ck_reglas_tipo CHECK (tipo IN ('tasa', 'tope', 'formula', 'condicion', 'parametro', 'redondeo')),
    CONSTRAINT ck_reglas_vigencia CHECK (vigente_hasta IS NULL OR vigente_hasta >= vigente_desde),
    CONSTRAINT ck_reglas_version CHECK (version > 0),
    CONSTRAINT ck_reglas_estado CHECK (estado IN ('borrador', 'vigente', 'reemplazada', 'derogada'))
);

CREATE INDEX IF NOT EXISTS ix_reglas_tributarias_busqueda
    ON spa.reglas_tributarias (anio_tributario_id, regimen_tributario_id, codigo, estado);

CREATE INDEX IF NOT EXISTS ix_reglas_tributarias_configuracion
    ON spa.reglas_tributarias USING gin (configuracion);

CREATE TABLE IF NOT EXISTS spa.tramos_tributarios (
    tramo_tributario_id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    regla_tributaria_id     uuid NOT NULL REFERENCES spa.reglas_tributarias (regla_tributaria_id) ON DELETE CASCADE,
    numero_tramo            smallint NOT NULL,
    limite_inferior         numeric(30,10) NOT NULL,
    limite_superior         numeric(30,10),
    tasa                    numeric(18,10) NOT NULL,
    rebaja                  numeric(30,10) NOT NULL DEFAULT 0,
    unidad                  varchar(20) NOT NULL DEFAULT 'CLP',
    CONSTRAINT uq_tramos_numero UNIQUE (regla_tributaria_id, numero_tramo),
    CONSTRAINT ck_tramos_numero CHECK (numero_tramo > 0),
    CONSTRAINT ck_tramos_limites CHECK (limite_superior IS NULL OR limite_superior > limite_inferior),
    CONSTRAINT ck_tramos_tasa CHECK (tasa >= 0),
    CONSTRAINT ck_tramos_unidad CHECK (unidad IN ('CLP', 'UTM', 'UTA', 'PORCENTAJE'))
);

CREATE TABLE IF NOT EXISTS spa.indicadores_tributarios (
    indicador_tributario_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo                  varchar(30) NOT NULL,
    nombre                  varchar(150) NOT NULL,
    periodo                 date NOT NULL,
    valor                   numeric(30,10) NOT NULL,
    fuente_oficial          text NOT NULL,
    fecha_consulta_fuente   date NOT NULL,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_indicadores_periodo UNIQUE (codigo, periodo),
    CONSTRAINT ck_indicadores_valor CHECK (valor >= 0)
);

CREATE INDEX IF NOT EXISTS ix_indicadores_tributarios_codigo_periodo
    ON spa.indicadores_tributarios (codigo, periodo DESC);

-- =============================================================================
-- CONTRAPARTES, ARCHIVOS E IMPORTACIONES
-- =============================================================================

CREATE TABLE IF NOT EXISTS spa.contrapartes (
    contraparte_id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    rut                     varchar(20) NOT NULL,
    rut_normalizado         varchar(9) NOT NULL,
    razon_social            varchar(250) NOT NULL,
    nombre_fantasia         varchar(250),
    giro                    varchar(300),
    correo                  varchar(320),
    telefono                varchar(50),
    direccion               varchar(300),
    es_cliente              boolean NOT NULL DEFAULT false,
    es_proveedor            boolean NOT NULL DEFAULT false,
    activo                  boolean NOT NULL DEFAULT true,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_contrapartes_empresa_id UNIQUE (empresa_id, contraparte_id),
    CONSTRAINT ck_contrapartes_rut CHECK (rut_normalizado ~ '^[0-9]{7,8}[0-9K]$'),
    CONSTRAINT ck_contrapartes_tipo CHECK (es_cliente OR es_proveedor),
    CONSTRAINT ck_contrapartes_razon_social CHECK (btrim(razon_social) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_contrapartes_empresa_rut
    ON spa.contrapartes (empresa_id, rut_normalizado)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_contrapartes_empresa_nombre
    ON spa.contrapartes (empresa_id, lower(razon_social))
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.archivos_importados (
    archivo_importado_id    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    nombre_original         varchar(500) NOT NULL,
    nombre_almacenado       varchar(500) NOT NULL,
    ruta_almacenamiento     text NOT NULL,
    tipo_mime               varchar(150) NOT NULL,
    extension               varchar(20),
    tamano_bytes            bigint NOT NULL,
    hash_sha256             char(64) NOT NULL,
    origen                  varchar(40) NOT NULL,
    subido_por              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_archivos_empresa_id UNIQUE (empresa_id, archivo_importado_id),
    CONSTRAINT ck_archivos_tamano CHECK (tamano_bytes > 0),
    CONSTRAINT ck_archivos_hash CHECK (hash_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_archivos_origen CHECK (origen IN ('sii_csv', 'sii_excel', 'sii_xml', 'banco', 'manual', 'sistema', 'otro'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_archivos_empresa_hash
    ON spa.archivos_importados (empresa_id, hash_sha256)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_archivos_empresa_fecha
    ON spa.archivos_importados (empresa_id, fecha_creacion DESC)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.trabajos_importacion (
    trabajo_importacion_id  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    archivo_importado_id    uuid NOT NULL,
    tipo                    varchar(40) NOT NULL,
    estado                  varchar(30) NOT NULL DEFAULT 'pendiente',
    clave_idempotencia      varchar(200),
    total_filas             integer NOT NULL DEFAULT 0,
    filas_creadas           integer NOT NULL DEFAULT 0,
    filas_actualizadas      integer NOT NULL DEFAULT 0,
    filas_duplicadas        integer NOT NULL DEFAULT 0,
    filas_rechazadas        integer NOT NULL DEFAULT 0,
    configuracion           jsonb NOT NULL DEFAULT '{}'::jsonb,
    mensaje_error           text,
    iniciado_por            uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_inicio            timestamptz,
    fecha_fin               timestamptz,
    CONSTRAINT uq_trabajos_empresa_id UNIQUE (empresa_id, trabajo_importacion_id),
    CONSTRAINT fk_trabajos_archivo_empresa FOREIGN KEY (empresa_id, archivo_importado_id)
        REFERENCES spa.archivos_importados (empresa_id, archivo_importado_id),
    CONSTRAINT ck_trabajos_tipo CHECK (tipo IN ('documentos_tributarios', 'cartola_bancaria', 'dte_xml', 'otro')),
    CONSTRAINT ck_trabajos_estado CHECK (estado IN ('pendiente', 'procesando', 'completado', 'completado_con_errores', 'fallido', 'cancelado')),
    CONSTRAINT ck_trabajos_contadores CHECK (
        total_filas >= 0 AND filas_creadas >= 0 AND filas_actualizadas >= 0
        AND filas_duplicadas >= 0 AND filas_rechazadas >= 0
    ),
    CONSTRAINT ck_trabajos_fechas CHECK (fecha_fin IS NULL OR fecha_inicio IS NULL OR fecha_fin >= fecha_inicio)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_trabajos_empresa_idempotencia
    ON spa.trabajos_importacion (empresa_id, clave_idempotencia)
    WHERE clave_idempotencia IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_trabajos_empresa_estado
    ON spa.trabajos_importacion (empresa_id, estado, fecha_creacion DESC);

CREATE TABLE IF NOT EXISTS spa.errores_importacion (
    error_importacion_id    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    trabajo_importacion_id  uuid NOT NULL,
    numero_fila             integer,
    columna                 varchar(150),
    codigo_error            varchar(100) NOT NULL,
    mensaje                 text NOT NULL,
    datos_fila              jsonb,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_errores_trabajo_empresa FOREIGN KEY (empresa_id, trabajo_importacion_id)
        REFERENCES spa.trabajos_importacion (empresa_id, trabajo_importacion_id) ON DELETE CASCADE,
    CONSTRAINT ck_errores_numero_fila CHECK (numero_fila IS NULL OR numero_fila > 0)
);

CREATE INDEX IF NOT EXISTS ix_errores_importacion_trabajo
    ON spa.errores_importacion (empresa_id, trabajo_importacion_id, numero_fila);

-- =============================================================================
-- DOCUMENTOS TRIBUTARIOS, LINEAS Y PAGOS
-- =============================================================================

CREATE TABLE IF NOT EXISTS spa.documentos_tributarios (
    documento_tributario_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    contraparte_id          uuid,
    trabajo_importacion_id  uuid,
    archivo_importado_id    uuid,
    direccion               varchar(10) NOT NULL,
    tipo_documento          varchar(40) NOT NULL,
    codigo_sii              integer,
    folio                   varchar(50) NOT NULL,
    rut_emisor              varchar(20) NOT NULL,
    rut_emisor_normalizado  varchar(9) NOT NULL,
    razon_social_emisor     varchar(250),
    rut_receptor            varchar(20) NOT NULL,
    rut_receptor_normalizado varchar(9) NOT NULL,
    razon_social_receptor   varchar(250),
    fecha_emision           date NOT NULL,
    fecha_vencimiento       date,
    fecha_recepcion         date,
    moneda                  char(3) NOT NULL DEFAULT 'CLP',
    monto_neto              numeric(20,4) NOT NULL DEFAULT 0,
    monto_exento            numeric(20,4) NOT NULL DEFAULT 0,
    monto_iva               numeric(20,4) NOT NULL DEFAULT 0,
    monto_otros_impuestos   numeric(20,4) NOT NULL DEFAULT 0,
    monto_total             numeric(20,4) NOT NULL,
    monto_pagado            numeric(20,4) NOT NULL DEFAULT 0,
    estado_documento        varchar(30) NOT NULL DEFAULT 'registrado',
    estado_pago             varchar(20) NOT NULL DEFAULT 'pendiente',
    estado_revision         varchar(30) NOT NULL DEFAULT 'pendiente_revision',
    fuente                  varchar(30) NOT NULL,
    documento_referencia_id uuid,
    motivo_referencia       varchar(250),
    datos_originales        jsonb,
    creado_por              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    modificado_por          uuid REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_documentos_empresa_id UNIQUE (empresa_id, documento_tributario_id),
    CONSTRAINT fk_documentos_contraparte_empresa FOREIGN KEY (empresa_id, contraparte_id)
        REFERENCES spa.contrapartes (empresa_id, contraparte_id),
    CONSTRAINT fk_documentos_trabajo_empresa FOREIGN KEY (empresa_id, trabajo_importacion_id)
        REFERENCES spa.trabajos_importacion (empresa_id, trabajo_importacion_id),
    CONSTRAINT fk_documentos_archivo_empresa FOREIGN KEY (empresa_id, archivo_importado_id)
        REFERENCES spa.archivos_importados (empresa_id, archivo_importado_id),
    CONSTRAINT fk_documentos_referencia_empresa FOREIGN KEY (empresa_id, documento_referencia_id)
        REFERENCES spa.documentos_tributarios (empresa_id, documento_tributario_id),
    CONSTRAINT ck_documentos_direccion CHECK (direccion IN ('compra', 'venta')),
    CONSTRAINT ck_documentos_tipo CHECK (tipo_documento IN ('factura', 'factura_exenta', 'nota_credito', 'nota_debito', 'boleta', 'boleta_exenta', 'liquidacion_factura', 'otro')),
    CONSTRAINT ck_documentos_rut_emisor CHECK (rut_emisor_normalizado ~ '^[0-9]{7,8}[0-9K]$'),
    CONSTRAINT ck_documentos_rut_receptor CHECK (rut_receptor_normalizado ~ '^[0-9]{7,8}[0-9K]$'),
    CONSTRAINT ck_documentos_fechas CHECK (fecha_vencimiento IS NULL OR fecha_vencimiento >= fecha_emision),
    CONSTRAINT ck_documentos_montos CHECK (
        monto_neto >= 0 AND monto_exento >= 0 AND monto_iva >= 0
        AND monto_otros_impuestos >= 0 AND monto_total >= 0 AND monto_pagado >= 0
    ),
    CONSTRAINT ck_documentos_pago_total CHECK (monto_pagado <= monto_total),
    CONSTRAINT ck_documentos_estado CHECK (estado_documento IN ('registrado', 'aceptado', 'reclamado', 'anulado', 'observado')),
    CONSTRAINT ck_documentos_estado_pago CHECK (estado_pago IN ('pendiente', 'parcial', 'pagado', 'no_aplica')),
    CONSTRAINT ck_documentos_estado_revision CHECK (estado_revision IN ('confirmado', 'estimado', 'incompleto', 'pendiente_revision', 'rechazado')),
    CONSTRAINT ck_documentos_fuente CHECK (fuente IN ('importacion_csv', 'importacion_excel', 'dte_xml', 'manual', 'api_proveedor', 'servicio_oficial'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_documentos_identidad_activa
    ON spa.documentos_tributarios (
        empresa_id, direccion, tipo_documento, folio,
        rut_emisor_normalizado, rut_receptor_normalizado
    )
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_documentos_empresa_fecha
    ON spa.documentos_tributarios (empresa_id, fecha_emision DESC)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_documentos_empresa_direccion_periodo
    ON spa.documentos_tributarios (empresa_id, direccion, fecha_emision DESC)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_documentos_empresa_estado_pago
    ON spa.documentos_tributarios (empresa_id, estado_pago, fecha_vencimiento)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_documentos_empresa_contraparte
    ON spa.documentos_tributarios (empresa_id, contraparte_id, fecha_emision DESC)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_documentos_datos_originales
    ON spa.documentos_tributarios USING gin (datos_originales)
    WHERE datos_originales IS NOT NULL;

CREATE TABLE IF NOT EXISTS spa.lineas_documento (
    linea_documento_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    documento_tributario_id uuid NOT NULL,
    numero_linea            integer NOT NULL,
    codigo_item             varchar(100),
    descripcion             text NOT NULL,
    cantidad                numeric(20,6) NOT NULL,
    unidad_medida           varchar(30),
    precio_unitario         numeric(20,6) NOT NULL,
    monto_descuento         numeric(20,4) NOT NULL DEFAULT 0,
    monto_recargo           numeric(20,4) NOT NULL DEFAULT 0,
    monto_neto              numeric(20,4) NOT NULL,
    tasa_iva                numeric(18,10),
    monto_iva               numeric(20,4) NOT NULL DEFAULT 0,
    monto_total             numeric(20,4) NOT NULL,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_lineas_documento_empresa FOREIGN KEY (empresa_id, documento_tributario_id)
        REFERENCES spa.documentos_tributarios (empresa_id, documento_tributario_id) ON DELETE CASCADE,
    CONSTRAINT uq_lineas_documento_numero UNIQUE (empresa_id, documento_tributario_id, numero_linea),
    CONSTRAINT ck_lineas_numero CHECK (numero_linea > 0),
    CONSTRAINT ck_lineas_cantidad CHECK (cantidad > 0),
    CONSTRAINT ck_lineas_montos CHECK (
        precio_unitario >= 0 AND monto_descuento >= 0 AND monto_recargo >= 0
        AND monto_neto >= 0 AND monto_iva >= 0 AND monto_total >= 0
    ),
    CONSTRAINT ck_lineas_tasa_iva CHECK (tasa_iva IS NULL OR tasa_iva >= 0)
);

CREATE INDEX IF NOT EXISTS ix_lineas_documento
    ON spa.lineas_documento (empresa_id, documento_tributario_id, numero_linea);

CREATE TABLE IF NOT EXISTS spa.pagos_documentos (
    pago_documento_id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    documento_tributario_id uuid NOT NULL,
    fecha_pago              date NOT NULL,
    monto                   numeric(20,4) NOT NULL,
    medio_pago              varchar(40),
    referencia              varchar(200),
    observacion             text,
    creado_por              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT fk_pagos_documento_empresa FOREIGN KEY (empresa_id, documento_tributario_id)
        REFERENCES spa.documentos_tributarios (empresa_id, documento_tributario_id),
    CONSTRAINT uq_pagos_empresa_id UNIQUE (empresa_id, pago_documento_id),
    CONSTRAINT ck_pagos_monto CHECK (monto > 0)
);

CREATE INDEX IF NOT EXISTS ix_pagos_documento
    ON spa.pagos_documentos (empresa_id, documento_tributario_id, fecha_pago DESC)
    WHERE eliminado_en IS NULL;

-- =============================================================================
-- BANCA Y CONCILIACION
-- =============================================================================

CREATE TABLE IF NOT EXISTS spa.cuentas_bancarias (
    cuenta_bancaria_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    banco                   varchar(150) NOT NULL,
    tipo_cuenta             varchar(40) NOT NULL,
    numero_enmascarado      varchar(100) NOT NULL,
    identificador_externo   varchar(200),
    moneda                  char(3) NOT NULL DEFAULT 'CLP',
    saldo_inicial           numeric(20,4) NOT NULL DEFAULT 0,
    fecha_saldo_inicial     date,
    activa                  boolean NOT NULL DEFAULT true,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_cuentas_empresa_id UNIQUE (empresa_id, cuenta_bancaria_id),
    CONSTRAINT ck_cuentas_tipo CHECK (tipo_cuenta IN ('corriente', 'vista', 'ahorro', 'credito', 'otra')),
    CONSTRAINT ck_cuentas_moneda CHECK (moneda ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_cuentas_empresa_identificador
    ON spa.cuentas_bancarias (empresa_id, identificador_externo)
    WHERE identificador_externo IS NOT NULL AND eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_cuentas_empresa_activas
    ON spa.cuentas_bancarias (empresa_id, activa)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.movimientos_bancarios (
    movimiento_bancario_id  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    cuenta_bancaria_id      uuid NOT NULL,
    trabajo_importacion_id  uuid,
    fecha_movimiento        date NOT NULL,
    fecha_contabilizacion   date,
    descripcion             text NOT NULL,
    referencia              varchar(250),
    monto                   numeric(20,4) NOT NULL,
    saldo_informado         numeric(20,4),
    moneda                  char(3) NOT NULL DEFAULT 'CLP',
    hash_movimiento         char(64) NOT NULL,
    clasificacion           varchar(50) NOT NULL DEFAULT 'pendiente_revision',
    origen_clasificacion    varchar(20) NOT NULL DEFAULT 'sin_clasificar',
    confianza_sugerencia    numeric(5,4),
    explicacion_sugerencia  text,
    confirmado_por          uuid REFERENCES spa.usuarios (usuario_id),
    fecha_confirmacion      timestamptz,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_movimientos_empresa_id UNIQUE (empresa_id, movimiento_bancario_id),
    CONSTRAINT fk_movimientos_cuenta_empresa FOREIGN KEY (empresa_id, cuenta_bancaria_id)
        REFERENCES spa.cuentas_bancarias (empresa_id, cuenta_bancaria_id),
    CONSTRAINT fk_movimientos_trabajo_empresa FOREIGN KEY (empresa_id, trabajo_importacion_id)
        REFERENCES spa.trabajos_importacion (empresa_id, trabajo_importacion_id),
    CONSTRAINT ck_movimientos_monto CHECK (monto <> 0),
    CONSTRAINT ck_movimientos_hash CHECK (hash_movimiento ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_movimientos_clasificacion CHECK (clasificacion IN (
        'cobro_cliente', 'pago_proveedor', 'gasto_empresarial', 'transferencia_propietario',
        'posible_distribucion', 'sueldo_empresarial', 'prestamo', 'devolucion',
        'gasto_personal', 'transferencia_cuentas_propias', 'impuesto', 'comision_bancaria',
        'pendiente_revision'
    )),
    CONSTRAINT ck_movimientos_origen_clasificacion CHECK (origen_clasificacion IN ('sin_clasificar', 'manual', 'regla', 'ia')),
    CONSTRAINT ck_movimientos_confianza CHECK (confianza_sugerencia IS NULL OR confianza_sugerencia BETWEEN 0 AND 1)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_movimientos_cuenta_hash
    ON spa.movimientos_bancarios (empresa_id, cuenta_bancaria_id, hash_movimiento)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_movimientos_empresa_fecha
    ON spa.movimientos_bancarios (empresa_id, fecha_movimiento DESC)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_movimientos_empresa_clasificacion
    ON spa.movimientos_bancarios (empresa_id, clasificacion, fecha_movimiento DESC)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.conciliaciones (
    conciliacion_id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    movimiento_bancario_id  uuid NOT NULL,
    estado                  varchar(30) NOT NULL DEFAULT 'confirmada',
    monto_conciliado        numeric(20,4) NOT NULL,
    observacion             text,
    creado_por              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_conciliaciones_empresa_id UNIQUE (empresa_id, conciliacion_id),
    CONSTRAINT fk_conciliaciones_movimiento_empresa FOREIGN KEY (empresa_id, movimiento_bancario_id)
        REFERENCES spa.movimientos_bancarios (empresa_id, movimiento_bancario_id),
    CONSTRAINT ck_conciliaciones_estado CHECK (estado IN ('sugerida', 'confirmada', 'rechazada', 'anulada')),
    CONSTRAINT ck_conciliaciones_monto CHECK (monto_conciliado > 0)
);

CREATE INDEX IF NOT EXISTS ix_conciliaciones_movimiento
    ON spa.conciliaciones (empresa_id, movimiento_bancario_id, estado)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.conciliaciones_documentos (
    conciliacion_documento_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    conciliacion_id         uuid NOT NULL,
    documento_tributario_id uuid NOT NULL,
    monto_aplicado          numeric(20,4) NOT NULL,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_conc_doc_conciliacion_empresa FOREIGN KEY (empresa_id, conciliacion_id)
        REFERENCES spa.conciliaciones (empresa_id, conciliacion_id) ON DELETE CASCADE,
    CONSTRAINT fk_conc_doc_documento_empresa FOREIGN KEY (empresa_id, documento_tributario_id)
        REFERENCES spa.documentos_tributarios (empresa_id, documento_tributario_id),
    CONSTRAINT uq_conciliaciones_documentos UNIQUE (empresa_id, conciliacion_id, documento_tributario_id),
    CONSTRAINT ck_conc_doc_monto CHECK (monto_aplicado > 0)
);

CREATE INDEX IF NOT EXISTS ix_conciliaciones_documento
    ON spa.conciliaciones_documentos (empresa_id, documento_tributario_id);

-- =============================================================================
-- GASTOS Y MOVIMIENTOS DEL PROPIETARIO
-- =============================================================================

CREATE TABLE IF NOT EXISTS spa.clasificaciones_gasto (
    clasificacion_gasto_id  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo                  varchar(60) NOT NULL UNIQUE,
    nombre                  varchar(150) NOT NULL,
    descripcion             text,
    tratamiento_defecto     varchar(30) NOT NULL DEFAULT 'pendiente_revision',
    requiere_justificacion  boolean NOT NULL DEFAULT true,
    activa                  boolean NOT NULL DEFAULT true,
    CONSTRAINT ck_clasificaciones_tratamiento CHECK (tratamiento_defecto IN ('aceptado', 'rechazado', 'parcial', 'pendiente_revision'))
);

CREATE TABLE IF NOT EXISTS spa.gastos (
    gasto_id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    documento_tributario_id uuid,
    movimiento_bancario_id  uuid,
    clasificacion_gasto_id  uuid REFERENCES spa.clasificaciones_gasto (clasificacion_gasto_id),
    fecha_gasto             date NOT NULL,
    descripcion             text NOT NULL,
    monto_neto              numeric(20,4) NOT NULL DEFAULT 0,
    monto_iva               numeric(20,4) NOT NULL DEFAULT 0,
    monto_total             numeric(20,4) NOT NULL,
    tratamiento_tributario  varchar(30) NOT NULL DEFAULT 'pendiente_revision',
    porcentaje_aceptado     numeric(7,4),
    justificacion           text,
    estado_revision         varchar(30) NOT NULL DEFAULT 'pendiente_revision',
    revisado_por            uuid REFERENCES spa.usuarios (usuario_id),
    fecha_revision          timestamptz,
    creado_por              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_gastos_empresa_id UNIQUE (empresa_id, gasto_id),
    CONSTRAINT fk_gastos_documento_empresa FOREIGN KEY (empresa_id, documento_tributario_id)
        REFERENCES spa.documentos_tributarios (empresa_id, documento_tributario_id),
    CONSTRAINT fk_gastos_movimiento_empresa FOREIGN KEY (empresa_id, movimiento_bancario_id)
        REFERENCES spa.movimientos_bancarios (empresa_id, movimiento_bancario_id),
    CONSTRAINT ck_gastos_montos CHECK (monto_neto >= 0 AND monto_iva >= 0 AND monto_total >= 0),
    CONSTRAINT ck_gastos_tratamiento CHECK (tratamiento_tributario IN ('aceptado', 'rechazado', 'parcial', 'pendiente_revision')),
    CONSTRAINT ck_gastos_porcentaje CHECK (porcentaje_aceptado IS NULL OR porcentaje_aceptado BETWEEN 0 AND 100),
    CONSTRAINT ck_gastos_revision CHECK (estado_revision IN ('confirmado', 'estimado', 'incompleto', 'pendiente_revision', 'rechazado'))
);

CREATE INDEX IF NOT EXISTS ix_gastos_empresa_fecha
    ON spa.gastos (empresa_id, fecha_gasto DESC)
    WHERE eliminado_en IS NULL;

CREATE INDEX IF NOT EXISTS ix_gastos_empresa_tratamiento
    ON spa.gastos (empresa_id, tratamiento_tributario, estado_revision)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.propietarios (
    propietario_id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    usuario_id              uuid REFERENCES spa.usuarios (usuario_id),
    rut                     varchar(20) NOT NULL,
    rut_normalizado         varchar(9) NOT NULL,
    nombre_completo         varchar(250) NOT NULL,
    porcentaje_participacion numeric(7,4) NOT NULL DEFAULT 100,
    fecha_desde             date NOT NULL,
    fecha_hasta             date,
    activo                  boolean NOT NULL DEFAULT true,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_propietarios_empresa_id UNIQUE (empresa_id, propietario_id),
    CONSTRAINT ck_propietarios_rut CHECK (rut_normalizado ~ '^[0-9]{7,8}[0-9K]$'),
    CONSTRAINT ck_propietarios_participacion CHECK (porcentaje_participacion > 0 AND porcentaje_participacion <= 100),
    CONSTRAINT ck_propietarios_fechas CHECK (fecha_hasta IS NULL OR fecha_hasta >= fecha_desde)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_propietarios_empresa_rut_activo
    ON spa.propietarios (empresa_id, rut_normalizado)
    WHERE activo = true;

CREATE TABLE IF NOT EXISTS spa.ingresos_propietario (
    ingreso_propietario_id  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    propietario_id          uuid NOT NULL,
    anio_comercial          smallint NOT NULL,
    tipo_ingreso            varchar(40) NOT NULL,
    descripcion             text,
    monto_bruto             numeric(20,4) NOT NULL,
    monto_retencion         numeric(20,4) NOT NULL DEFAULT 0,
    monto_exento            numeric(20,4) NOT NULL DEFAULT 0,
    confirmado              boolean NOT NULL DEFAULT false,
    fuente                  varchar(30) NOT NULL DEFAULT 'manual',
    creado_por              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT fk_ingresos_propietario_empresa FOREIGN KEY (empresa_id, propietario_id)
        REFERENCES spa.propietarios (empresa_id, propietario_id),
    CONSTRAINT ck_ingresos_propietario_anio CHECK (anio_comercial BETWEEN 2000 AND 2200),
    CONSTRAINT ck_ingresos_propietario_tipo CHECK (tipo_ingreso IN ('remuneracion', 'sueldo_empresarial', 'honorarios', 'arriendo', 'retiro', 'distribucion', 'otro')),
    CONSTRAINT ck_ingresos_propietario_montos CHECK (monto_bruto >= 0 AND monto_retencion >= 0 AND monto_exento >= 0)
);

CREATE INDEX IF NOT EXISTS ix_ingresos_propietario_periodo
    ON spa.ingresos_propietario (empresa_id, propietario_id, anio_comercial, tipo_ingreso)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.movimientos_propietario (
    movimiento_propietario_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    propietario_id          uuid NOT NULL,
    movimiento_bancario_id  uuid,
    fecha_movimiento        date NOT NULL,
    tipo                    varchar(40) NOT NULL,
    monto                   numeric(20,4) NOT NULL,
    descripcion             text,
    estado_revision         varchar(30) NOT NULL DEFAULT 'pendiente_revision',
    confirmado_por          uuid REFERENCES spa.usuarios (usuario_id),
    fecha_confirmacion      timestamptz,
    creado_por              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_mov_propietario_empresa_id UNIQUE (empresa_id, movimiento_propietario_id),
    CONSTRAINT fk_mov_propietario_empresa FOREIGN KEY (empresa_id, propietario_id)
        REFERENCES spa.propietarios (empresa_id, propietario_id),
    CONSTRAINT fk_mov_propietario_banco_empresa FOREIGN KEY (empresa_id, movimiento_bancario_id)
        REFERENCES spa.movimientos_bancarios (empresa_id, movimiento_bancario_id),
    CONSTRAINT ck_mov_propietario_tipo CHECK (tipo IN ('retiro', 'distribucion', 'sueldo_empresarial', 'prestamo_propietario', 'devolucion_prestamo', 'aporte_capital', 'gasto_personal', 'otro')),
    CONSTRAINT ck_mov_propietario_monto CHECK (monto > 0),
    CONSTRAINT ck_mov_propietario_revision CHECK (estado_revision IN ('confirmado', 'estimado', 'incompleto', 'pendiente_revision', 'rechazado'))
);

CREATE INDEX IF NOT EXISTS ix_movimientos_propietario_periodo
    ON spa.movimientos_propietario (empresa_id, propietario_id, fecha_movimiento DESC)
    WHERE eliminado_en IS NULL;

-- =============================================================================
-- CALCULOS, CREDITOS Y REGISTROS TRIBUTARIOS
-- =============================================================================

CREATE TABLE IF NOT EXISTS spa.calculos_tributarios (
    calculo_tributario_id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    anio_tributario_id      uuid NOT NULL REFERENCES spa.anios_tributarios (anio_tributario_id),
    regimen_tributario_id   uuid NOT NULL REFERENCES spa.regimenes_tributarios (regimen_tributario_id),
    tipo_calculo            varchar(40) NOT NULL,
    periodo_desde           date NOT NULL,
    periodo_hasta           date NOT NULL,
    version_motor           varchar(50) NOT NULL,
    version_reglas          varchar(100) NOT NULL,
    estado                  varchar(30) NOT NULL DEFAULT 'pendiente',
    resultado_principal     numeric(20,4),
    moneda                  char(3) NOT NULL DEFAULT 'CLP',
    supuestos               jsonb NOT NULL DEFAULT '[]'::jsonb,
    advertencias            jsonb NOT NULL DEFAULT '[]'::jsonb,
    datos_faltantes         jsonb NOT NULL DEFAULT '[]'::jsonb,
    hash_entradas           char(64),
    creado_por              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_finalizacion      timestamptz,
    CONSTRAINT uq_calculos_empresa_id UNIQUE (empresa_id, calculo_tributario_id),
    CONSTRAINT ck_calculos_tipo CHECK (tipo_calculo IN ('iva', 'ppm', 'primera_categoria', 'global_complementario', 'utilidad_tributaria', 'simulacion_retiro', 'comparacion_regimen')),
    CONSTRAINT ck_calculos_periodo CHECK (periodo_hasta >= periodo_desde),
    CONSTRAINT ck_calculos_estado CHECK (estado IN ('pendiente', 'procesando', 'completado', 'pendiente_verificar', 'fallido', 'anulado')),
    CONSTRAINT ck_calculos_hash CHECK (hash_entradas IS NULL OR hash_entradas ~ '^[0-9a-f]{64}$')
);

CREATE INDEX IF NOT EXISTS ix_calculos_empresa_periodo
    ON spa.calculos_tributarios (empresa_id, tipo_calculo, periodo_desde, periodo_hasta, fecha_creacion DESC);

CREATE TABLE IF NOT EXISTS spa.entradas_calculo_tributario (
    entrada_calculo_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    calculo_tributario_id   uuid NOT NULL,
    codigo                  varchar(100) NOT NULL,
    descripcion             text,
    valor_decimal           numeric(30,10),
    valor_texto             text,
    valor_json              jsonb,
    tipo_origen             varchar(50) NOT NULL,
    origen_id               uuid,
    confirmado              boolean NOT NULL DEFAULT false,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_entradas_calculo_empresa FOREIGN KEY (empresa_id, calculo_tributario_id)
        REFERENCES spa.calculos_tributarios (empresa_id, calculo_tributario_id) ON DELETE CASCADE,
    CONSTRAINT uq_entradas_calculo_codigo UNIQUE (empresa_id, calculo_tributario_id, codigo),
    CONSTRAINT ck_entradas_valor CHECK (num_nonnulls(valor_decimal, valor_texto, valor_json) = 1),
    CONSTRAINT ck_entradas_origen CHECK (tipo_origen IN ('documento', 'movimiento_bancario', 'regla', 'usuario', 'calculo_anterior', 'sistema'))
);

CREATE INDEX IF NOT EXISTS ix_entradas_calculo
    ON spa.entradas_calculo_tributario (empresa_id, calculo_tributario_id);

CREATE TABLE IF NOT EXISTS spa.resultados_calculo_tributario (
    resultado_calculo_id    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    calculo_tributario_id   uuid NOT NULL,
    codigo                  varchar(100) NOT NULL,
    nombre                  varchar(200) NOT NULL,
    valor_decimal           numeric(30,10),
    valor_texto             text,
    valor_json              jsonb,
    estado                  varchar(30) NOT NULL DEFAULT 'estimado',
    formula_aplicada        text,
    detalle                 jsonb NOT NULL DEFAULT '{}'::jsonb,
    orden                   integer NOT NULL DEFAULT 0,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_resultados_calculo_empresa FOREIGN KEY (empresa_id, calculo_tributario_id)
        REFERENCES spa.calculos_tributarios (empresa_id, calculo_tributario_id) ON DELETE CASCADE,
    CONSTRAINT uq_resultados_calculo_codigo UNIQUE (empresa_id, calculo_tributario_id, codigo),
    CONSTRAINT ck_resultados_valor CHECK (num_nonnulls(valor_decimal, valor_texto, valor_json) = 1),
    CONSTRAINT ck_resultados_estado CHECK (estado IN ('confirmado', 'estimado', 'incompleto', 'pendiente_revision'))
);

CREATE INDEX IF NOT EXISTS ix_resultados_calculo
    ON spa.resultados_calculo_tributario (empresa_id, calculo_tributario_id, orden);

CREATE TABLE IF NOT EXISTS spa.creditos_tributarios (
    credito_tributario_id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    calculo_origen_id       uuid,
    codigo                  varchar(80) NOT NULL,
    nombre                  varchar(200) NOT NULL,
    anio_origen             smallint NOT NULL,
    monto_inicial           numeric(20,4) NOT NULL,
    monto_disponible        numeric(20,4) NOT NULL,
    derecho_devolucion      boolean NOT NULL DEFAULT false,
    tasa_credito            numeric(18,10),
    estado                  varchar(30) NOT NULL DEFAULT 'estimado',
    condiciones             text,
    fuente_normativa        text,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_creditos_empresa_id UNIQUE (empresa_id, credito_tributario_id),
    CONSTRAINT fk_creditos_calculo_empresa FOREIGN KEY (empresa_id, calculo_origen_id)
        REFERENCES spa.calculos_tributarios (empresa_id, calculo_tributario_id),
    CONSTRAINT ck_creditos_anio CHECK (anio_origen BETWEEN 2000 AND 2200),
    CONSTRAINT ck_creditos_montos CHECK (monto_inicial >= 0 AND monto_disponible >= 0 AND monto_disponible <= monto_inicial),
    CONSTRAINT ck_creditos_tasa CHECK (tasa_credito IS NULL OR tasa_credito >= 0),
    CONSTRAINT ck_creditos_estado CHECK (estado IN ('confirmado', 'estimado', 'agotado', 'vencido', 'pendiente_revision', 'rechazado'))
);

CREATE INDEX IF NOT EXISTS ix_creditos_empresa_disponibles
    ON spa.creditos_tributarios (empresa_id, anio_origen, codigo, estado)
    WHERE monto_disponible > 0;

CREATE TABLE IF NOT EXISTS spa.registros_tributarios (
    registro_tributario_id  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    anio_comercial          smallint NOT NULL,
    tipo_registro           varchar(30) NOT NULL,
    saldo_inicial           numeric(20,4) NOT NULL DEFAULT 0,
    saldo_final             numeric(20,4) NOT NULL DEFAULT 0,
    estado                  varchar(30) NOT NULL DEFAULT 'estimado',
    version_reglas          varchar(100) NOT NULL,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_registros_empresa_id UNIQUE (empresa_id, registro_tributario_id),
    CONSTRAINT uq_registros_empresa_anio_tipo UNIQUE (empresa_id, anio_comercial, tipo_registro),
    CONSTRAINT ck_registros_anio CHECK (anio_comercial BETWEEN 2000 AND 2200),
    CONSTRAINT ck_registros_tipo CHECK (tipo_registro IN ('RAI', 'REX', 'SAC', 'DDAN', 'STUT', 'OTRO')),
    CONSTRAINT ck_registros_estado CHECK (estado IN ('confirmado', 'estimado', 'incompleto', 'pendiente_revision'))
);

CREATE TABLE IF NOT EXISTS spa.movimientos_registro_tributario (
    movimiento_registro_id  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    registro_tributario_id  uuid NOT NULL,
    credito_tributario_id   uuid,
    fecha_movimiento        date NOT NULL,
    tipo_movimiento         varchar(30) NOT NULL,
    monto                   numeric(20,4) NOT NULL,
    descripcion             text NOT NULL,
    origen_tipo             varchar(50) NOT NULL,
    origen_id               uuid,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_mov_registro_empresa FOREIGN KEY (empresa_id, registro_tributario_id)
        REFERENCES spa.registros_tributarios (empresa_id, registro_tributario_id) ON DELETE CASCADE,
    CONSTRAINT fk_mov_registro_credito_empresa FOREIGN KEY (empresa_id, credito_tributario_id)
        REFERENCES spa.creditos_tributarios (empresa_id, credito_tributario_id),
    CONSTRAINT ck_mov_registro_tipo CHECK (tipo_movimiento IN ('apertura', 'incremento', 'disminucion', 'imputacion', 'ajuste', 'cierre')),
    CONSTRAINT ck_mov_registro_monto CHECK (monto >= 0)
);

CREATE INDEX IF NOT EXISTS ix_movimientos_registro
    ON spa.movimientos_registro_tributario (empresa_id, registro_tributario_id, fecha_movimiento);

CREATE TABLE IF NOT EXISTS spa.distribuciones_tributarias (
    distribucion_tributaria_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    propietario_id          uuid NOT NULL,
    movimiento_propietario_id uuid,
    calculo_tributario_id   uuid,
    fecha_distribucion      date NOT NULL,
    monto_bruto             numeric(20,4) NOT NULL,
    base_imponible          numeric(20,4),
    credito_asignado        numeric(20,4) NOT NULL DEFAULT 0,
    incremento              numeric(20,4) NOT NULL DEFAULT 0,
    estado                  varchar(30) NOT NULL DEFAULT 'pendiente_revision',
    detalle_asignacion      jsonb NOT NULL DEFAULT '{}'::jsonb,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_distribuciones_propietario_empresa FOREIGN KEY (empresa_id, propietario_id)
        REFERENCES spa.propietarios (empresa_id, propietario_id),
    CONSTRAINT fk_distribuciones_movimiento_empresa FOREIGN KEY (empresa_id, movimiento_propietario_id)
        REFERENCES spa.movimientos_propietario (empresa_id, movimiento_propietario_id),
    CONSTRAINT fk_distribuciones_calculo_empresa FOREIGN KEY (empresa_id, calculo_tributario_id)
        REFERENCES spa.calculos_tributarios (empresa_id, calculo_tributario_id),
    CONSTRAINT ck_distribuciones_montos CHECK (
        monto_bruto > 0 AND (base_imponible IS NULL OR base_imponible >= 0)
        AND credito_asignado >= 0 AND incremento >= 0
    ),
    CONSTRAINT ck_distribuciones_estado CHECK (estado IN ('confirmado', 'estimado', 'incompleto', 'pendiente_revision', 'anulado'))
);

CREATE INDEX IF NOT EXISTS ix_distribuciones_empresa_fecha
    ON spa.distribuciones_tributarias (empresa_id, fecha_distribucion DESC, propietario_id);

-- =============================================================================
-- ASISTENTE IA Y AUDITORIA
-- =============================================================================

CREATE TABLE IF NOT EXISTS spa.conversaciones_ia (
    conversacion_ia_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    usuario_id              uuid NOT NULL REFERENCES spa.usuarios (usuario_id),
    titulo                  varchar(250),
    estado                  varchar(20) NOT NULL DEFAULT 'activa',
    consentimiento_datos    boolean NOT NULL DEFAULT false,
    proveedor               varchar(100),
    modelo                  varchar(100),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_modificacion      timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    eliminado_en            timestamptz,
    CONSTRAINT uq_conversaciones_empresa_id UNIQUE (empresa_id, conversacion_ia_id),
    CONSTRAINT ck_conversaciones_estado CHECK (estado IN ('activa', 'archivada', 'cerrada'))
);

CREATE INDEX IF NOT EXISTS ix_conversaciones_usuario
    ON spa.conversaciones_ia (empresa_id, usuario_id, fecha_modificacion DESC)
    WHERE eliminado_en IS NULL;

CREATE TABLE IF NOT EXISTS spa.mensajes_ia (
    mensaje_ia_id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid NOT NULL REFERENCES spa.empresas (empresa_id) ON DELETE CASCADE,
    conversacion_ia_id      uuid NOT NULL,
    rol                     varchar(20) NOT NULL,
    contenido               text NOT NULL,
    herramientas_usadas     jsonb NOT NULL DEFAULT '[]'::jsonb,
    referencias_internas    jsonb NOT NULL DEFAULT '[]'::jsonb,
    advertencias            jsonb NOT NULL DEFAULT '[]'::jsonb,
    tokens_entrada          integer,
    tokens_salida           integer,
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_mensajes_conversacion_empresa FOREIGN KEY (empresa_id, conversacion_ia_id)
        REFERENCES spa.conversaciones_ia (empresa_id, conversacion_ia_id) ON DELETE CASCADE,
    CONSTRAINT ck_mensajes_rol CHECK (rol IN ('usuario', 'asistente', 'sistema', 'herramienta')),
    CONSTRAINT ck_mensajes_tokens CHECK (
        (tokens_entrada IS NULL OR tokens_entrada >= 0)
        AND (tokens_salida IS NULL OR tokens_salida >= 0)
    )
);

CREATE INDEX IF NOT EXISTS ix_mensajes_conversacion
    ON spa.mensajes_ia (empresa_id, conversacion_ia_id, fecha_creacion);

CREATE TABLE IF NOT EXISTS spa.registros_auditoria (
    registro_auditoria_id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              uuid REFERENCES spa.empresas (empresa_id) ON DELETE SET NULL,
    usuario_id              uuid REFERENCES spa.usuarios (usuario_id) ON DELETE SET NULL,
    sesion_id               uuid REFERENCES spa.sesiones (sesion_id) ON DELETE SET NULL,
    accion                  varchar(100) NOT NULL,
    entidad                 varchar(100) NOT NULL,
    entidad_id              uuid,
    resultado               varchar(20) NOT NULL DEFAULT 'exitoso',
    datos_anteriores        jsonb,
    datos_nuevos            jsonb,
    metadatos               jsonb NOT NULL DEFAULT '{}'::jsonb,
    direccion_ip            inet,
    agente_usuario          text,
    identificador_solicitud varchar(100),
    fecha_creacion          timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_auditoria_resultado CHECK (resultado IN ('exitoso', 'rechazado', 'fallido'))
);

CREATE INDEX IF NOT EXISTS ix_auditoria_empresa_fecha
    ON spa.registros_auditoria (empresa_id, fecha_creacion DESC);

CREATE INDEX IF NOT EXISTS ix_auditoria_usuario_fecha
    ON spa.registros_auditoria (usuario_id, fecha_creacion DESC);

CREATE INDEX IF NOT EXISTS ix_auditoria_entidad
    ON spa.registros_auditoria (empresa_id, entidad, entidad_id, fecha_creacion DESC);

CREATE INDEX IF NOT EXISTS ix_auditoria_solicitud
    ON spa.registros_auditoria (identificador_solicitud)
    WHERE identificador_solicitud IS NOT NULL;

-- =============================================================================
-- VISTAS DE APOYO
-- =============================================================================

CREATE OR REPLACE VIEW spa.vista_saldos_documentos AS
SELECT
    d.empresa_id,
    d.documento_tributario_id,
    d.direccion,
    d.tipo_documento,
    d.folio,
    d.fecha_emision,
    d.fecha_vencimiento,
    d.rut_emisor_normalizado,
    d.razon_social_emisor,
    d.rut_receptor_normalizado,
    d.razon_social_receptor,
    d.monto_total,
    d.monto_pagado,
    d.monto_total - d.monto_pagado AS saldo_pendiente,
    d.estado_pago,
    d.estado_revision
FROM spa.documentos_tributarios d
WHERE d.eliminado_en IS NULL;

COMMENT ON VIEW spa.vista_saldos_documentos
IS 'Documentos activos con su saldo pendiente calculado';

CREATE OR REPLACE VIEW spa.vista_resumen_importaciones AS
SELECT
    t.empresa_id,
    t.trabajo_importacion_id,
    t.tipo,
    t.estado,
    a.nombre_original,
    a.hash_sha256,
    t.total_filas,
    t.filas_creadas,
    t.filas_actualizadas,
    t.filas_duplicadas,
    t.filas_rechazadas,
    t.fecha_creacion,
    t.fecha_inicio,
    t.fecha_fin
FROM spa.trabajos_importacion t
JOIN spa.archivos_importados a
  ON a.empresa_id = t.empresa_id
 AND a.archivo_importado_id = t.archivo_importado_id;

COMMENT ON VIEW spa.vista_resumen_importaciones
IS 'Resumen de archivos y resultados de importacion por empresa';

-- =============================================================================
-- TRIGGERS DE FECHA DE MODIFICACION
-- =============================================================================

DO $$
DECLARE
    nombre_tabla text;
BEGIN
    FOREACH nombre_tabla IN ARRAY ARRAY[
        'usuarios',
        'regimenes_tributarios',
        'empresas',
        'usuarios_empresas',
        'anios_tributarios',
        'reglas_tributarias',
        'contrapartes',
        'documentos_tributarios',
        'cuentas_bancarias',
        'movimientos_bancarios',
        'conciliaciones',
        'gastos',
        'propietarios',
        'ingresos_propietario',
        'movimientos_propietario',
        'creditos_tributarios',
        'registros_tributarios',
        'distribuciones_tributarias',
        'conversaciones_ia'
    ]
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS trg_%I_fecha_modificacion ON spa.%I', nombre_tabla, nombre_tabla);
        EXECUTE format(
            'CREATE TRIGGER trg_%I_fecha_modificacion BEFORE UPDATE ON spa.%I FOR EACH ROW EXECUTE FUNCTION spa.actualizar_fecha_modificacion()',
            nombre_tabla,
            nombre_tabla
        );
    END LOOP;
END;
$$;

-- =============================================================================
-- CATALOGOS INICIALES (SIN TASAS NI VALORES TRIBUTARIOS INVENTADOS)
-- =============================================================================

INSERT INTO spa.roles (codigo, nombre, descripcion, es_sistema)
VALUES
    ('propietario', 'Propietario', 'Administra su empresa, usuarios y datos financieros', true),
    ('contador', 'Contador', 'Revisa antecedentes y calculos de empresas autorizadas', true),
    ('administrador_saas', 'Administrador SaaS', 'Administra la plataforma sin acceso financiero implicito', true)
ON CONFLICT (codigo) DO UPDATE
SET nombre = EXCLUDED.nombre,
    descripcion = EXCLUDED.descripcion;

INSERT INTO spa.permisos (codigo, nombre, descripcion)
VALUES
    ('empresa.ver', 'Ver empresa', 'Consulta datos basicos de la empresa'),
    ('empresa.editar', 'Editar empresa', 'Modifica datos basicos de la empresa'),
    ('usuarios.gestionar', 'Gestionar usuarios', 'Invita, modifica o revoca miembros'),
    ('documentos.ver', 'Ver documentos', 'Consulta documentos tributarios'),
    ('documentos.crear', 'Crear documentos', 'Registra documentos manualmente'),
    ('documentos.editar', 'Editar documentos', 'Corrige documentos permitidos'),
    ('importaciones.ejecutar', 'Ejecutar importaciones', 'Carga archivos de documentos o bancos'),
    ('banca.ver', 'Ver banca', 'Consulta cuentas y movimientos bancarios'),
    ('banca.conciliar', 'Conciliar movimientos', 'Confirma conciliaciones bancarias'),
    ('gastos.revisar', 'Revisar gastos', 'Clasifica y revisa tratamiento de gastos'),
    ('tributario.ver', 'Ver calculos tributarios', 'Consulta calculos y trazabilidad'),
    ('tributario.calcular', 'Ejecutar calculos tributarios', 'Genera estimaciones y simulaciones'),
    ('reportes.generar', 'Generar reportes', 'Descarga reportes de la empresa'),
    ('auditoria.ver', 'Ver auditoria', 'Consulta la bitacora de la empresa'),
    ('ia.usar', 'Usar asistente IA', 'Consulta el asistente bajo consentimiento')
ON CONFLICT (codigo) DO UPDATE
SET nombre = EXCLUDED.nombre,
    descripcion = EXCLUDED.descripcion;

INSERT INTO spa.roles_permisos (rol_id, permiso_id)
SELECT r.rol_id, p.permiso_id
FROM spa.roles r
CROSS JOIN spa.permisos p
WHERE r.codigo = 'propietario'
ON CONFLICT DO NOTHING;

INSERT INTO spa.roles_permisos (rol_id, permiso_id)
SELECT r.rol_id, p.permiso_id
FROM spa.roles r
JOIN spa.permisos p ON p.codigo IN (
    'empresa.ver', 'documentos.ver', 'documentos.crear', 'documentos.editar',
    'importaciones.ejecutar', 'banca.ver', 'banca.conciliar', 'gastos.revisar',
    'tributario.ver', 'tributario.calcular', 'reportes.generar', 'auditoria.ver'
)
WHERE r.codigo = 'contador'
ON CONFLICT DO NOTHING;

INSERT INTO spa.regimenes_tributarios (codigo, nombre, descripcion, referencia_normativa, activo)
VALUES
    (
        'pro_pyme_general_14d3',
        'Pro Pyme General',
        'Regimen Pro Pyme General. Las tasas y reglas se cargan por anio desde fuentes oficiales.',
        'Articulo 14 D N. 3 de la Ley sobre Impuesto a la Renta; verificar texto vigente.',
        true
    ),
    (
        'pro_pyme_transparente_14d8',
        'Pro Pyme Transparente',
        'Regimen Pro Pyme Transparente. Las tasas y reglas se cargan por anio desde fuentes oficiales.',
        'Articulo 14 D N. 8 de la Ley sobre Impuesto a la Renta; verificar texto vigente.',
        true
    )
ON CONFLICT (codigo) DO UPDATE
SET nombre = EXCLUDED.nombre,
    descripcion = EXCLUDED.descripcion,
    referencia_normativa = EXCLUDED.referencia_normativa,
    activo = EXCLUDED.activo;

INSERT INTO spa.clasificaciones_gasto (codigo, nombre, descripcion, tratamiento_defecto, requiere_justificacion)
VALUES
    ('operacional', 'Gasto operacional', 'Gasto relacionado con la operacion habitual', 'pendiente_revision', true),
    ('remuneraciones', 'Remuneraciones', 'Remuneraciones y obligaciones asociadas', 'pendiente_revision', true),
    ('servicios', 'Servicios', 'Servicios contratados por la empresa', 'pendiente_revision', true),
    ('activo_fijo', 'Activo fijo', 'Adquisicion potencial de activo fijo', 'pendiente_revision', true),
    ('impuestos', 'Impuestos', 'Pagos de impuestos y obligaciones fiscales', 'pendiente_revision', true),
    ('financiero', 'Gasto financiero', 'Comisiones, intereses y otros cargos financieros', 'pendiente_revision', true),
    ('personal', 'Gasto personal', 'Gasto potencialmente personal del propietario', 'rechazado', true),
    ('sin_clasificar', 'Sin clasificar', 'Pendiente de revision y antecedentes', 'pendiente_revision', true)
ON CONFLICT (codigo) DO UPDATE
SET nombre = EXCLUDED.nombre,
    descripcion = EXCLUDED.descripcion,
    tratamiento_defecto = EXCLUDED.tratamiento_defecto,
    requiere_justificacion = EXCLUDED.requiere_justificacion;

-- No se insertan tasas, tramos, UTM, UTA ni topes. Deben obtenerse de fuentes
-- oficiales vigentes, registrar su fecha de consulta y pasar revision contable.

COMMIT;

-- =============================================================================
-- CONSULTAS DE VERIFICACION (ejecutar despues del COMMIT)
-- =============================================================================

SELECT
    table_schema AS esquema,
    count(*) AS cantidad_tablas
FROM information_schema.tables
WHERE table_schema = 'spa'
  AND table_type = 'BASE TABLE'
GROUP BY table_schema;

SELECT
    schemaname AS esquema,
    count(*) AS cantidad_indices
FROM pg_indexes
WHERE schemaname = 'spa'
GROUP BY schemaname;

SELECT codigo, nombre
FROM spa.roles
ORDER BY codigo;

SELECT codigo, nombre
FROM spa.regimenes_tributarios
ORDER BY codigo;
