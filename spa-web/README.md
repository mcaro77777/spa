# MiSpA Web

Cliente Angular 20 standalone y zoneless para MiSpA IA.

```bash
npm install
npm start
```

El desarrollo se sirve en `http://localhost:4200` y deriva `/api` a `http://localhost:8080`. Para producción:

```bash
npm run build
```

El resultado queda en `dist/spa-web/browser` y debe copiarse a `../spa-api/ClientApp` antes de construir la imagen final de la API.
