/*
===============================================================================
 PASO OPCIONAL 0 - Crear la base de datos de MiSpA IA
===============================================================================

Ejecutar en DBeaver conectado a la base administrativa "postgres" y con
Auto-commit activado. PostgreSQL no permite CREATE DATABASE dentro de una
transaccion.

Si EasyPanel ya creo la base de datos indicada en sus variables de entorno,
omite este archivo y ejecuta directamente crear_esquema_spa.sql sobre esa base.

Despues de ejecutar este archivo, crea o abre una conexion DBeaver a "bd_spa".
===============================================================================
*/

CREATE DATABASE bd_spa
    WITH
    ENCODING = 'UTF8'
    TEMPLATE = template0
    CONNECTION LIMIT = -1;

COMMENT ON DATABASE bd_spa
IS 'Base de datos de la plataforma tributaria MiSpA IA';
