# Fuentes y límites del motor tributario

Fecha de revisión: 2026-11-12.

## Reglas incorporadas

- Régimen Pro Pyme General: artículo 14 letra D N°3 de la LIR.
- Tasa IDPC transitoria: 12,5% para años comerciales 2025, 2026 y 2027; 15% para 2028.
- PPM: para 2025–2027 se utiliza 0,125% hasta 50.000 UF y 0,25% sobre ese límite, según la condición informada por el usuario. Para 2028 se conservan las tasas permanentes de 0,25% y 0,5%, porque la rebaja transitoria de PPM termina en 2027.

Fuente primaria: [Circular SII N°53 de 2025](https://www.sii.cl/normativa_legislacion/circulares/2025/circu53.pdf).

Contexto oficial del régimen y Operación Renta: [Regímenes tributarios, SII](https://www.sii.cl/destacados/renta/2026/intermediarios/regimenes_tributarios/).

## Límites deliberados

La primera proyección usa cobros y egresos bancarios clasificados. No confirma procedencia tributaria, pérdidas, depreciaciones, correcciones, créditos, CPT, RAI, REX ni SAC. Estos últimos se mantienen en estado `incompleto` hasta incorporar saldos de apertura y validación profesional.

El resultado es informativo y no genera ni presenta formularios o declaraciones ante el SII.

## Asistente

El motor local explica exclusivamente agregados obtenidos mediante consultas internas de solo lectura. La integración opcional usa OpenAI Responses API con `store: false`; solo recibe un contexto agregado y anonimizado cuando el usuario otorga consentimiento explícito.
