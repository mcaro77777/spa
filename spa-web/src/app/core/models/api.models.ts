export interface EmpresaResumen { id: string; razonSocial: string; nombreFantasia?: string | null; rut: string; estado?: string; rol: string }
export interface Sesion { usuario: { id: string; correo: string }; empresaActivaId: string | null; empresas: EmpresaResumen[] }
export interface TokenResponse { accessToken: string; expiresIn: number; empresaId: string | null }
export interface ProblemaApi { type?: string; title: string; status: number; detail?: string; code?: string }
export interface Indicador { codigo: string; etiqueta: string; valor: string; estado: 'confirmado'|'estimado'|'incompleto'|'pendiente_revision'; detalle?: string }
export interface DashboardResumen { year: number; month: number; updatedAt: string; indicators: Indicador[] }
export interface Documento {
  id: string; direccion: 'compra'|'venta'; tipoDocumento: string; folio: string;
  rutEmisor: string; razonSocialEmisor?: string | null; rutReceptor: string; razonSocialReceptor?: string | null;
  fechaEmision: string; fechaVencimiento?: string | null; montoNeto: string; montoExento: string;
  montoIva: string; montoTotal: string; estadoPago: string; estadoRevision: string;
}
export interface PaginaDocumentos { items: Documento[]; page: number; pageSize: number; total: number; totalPages: number }
export interface Importacion { id: string; tipo: string; estado: string; totalFilas: number; filasCreadas: number; filasDuplicadas: number; filasRechazadas: number; fechaCreacion?: string }
export interface CuentaBancaria { id:string; banco:string; tipoCuenta:string; numeroEnmascarado:string; moneda:string; activa:boolean }
export interface MovimientoBancario {
  id:string; cuentaId:string; banco:string; cuenta:string; fecha:string; descripcion:string; referencia:string;
  monto:string; saldo?:string|null; clasificacion:string; origenClasificacion:string; justificacion:string; conciliado:boolean;
}
export interface PaginaMovimientos { items:MovimientoBancario[]; page:number; pageSize:number; total:number; totalPages:number }
export interface ResumenBancario { total:number; pendientes:number; conciliados:number; gastos:number; movimientosPropietario:number; alertas:number }
export interface CandidatoConciliacion { id:string; tipoDocumento:string; folio:string; fechaEmision:string; montoTotal:string; montoPagado:string; contraparte:string }

