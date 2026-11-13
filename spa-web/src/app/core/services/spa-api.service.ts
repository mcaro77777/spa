import { inject, Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import { CandidatoConciliacion, CuentaBancaria, DashboardResumen, EmpresaResumen, Importacion, PaginaDocumentos, PaginaMovimientos, ResumenBancario } from '../models/api.models';

export interface CrearEmpresa {
  razonSocial: string; nombreFantasia: string; rut: string; giro: string; correo: string;
  telefono: string; direccion: string; comuna: string; region: string; regimenCodigo: string;
}

@Injectable({ providedIn: 'root' })
export class SpaApiService {
  private readonly http = inject(HttpClient);
  private readonly api = environment.apiUrl;

  crearEmpresa(datos: CrearEmpresa): Observable<EmpresaResumen> { return this.http.post<EmpresaResumen>(`${this.api}/companies`, datos); }
  dashboard(year: number, month: number): Observable<DashboardResumen> {
    return this.http.get<DashboardResumen>(`${this.api}/dashboard/summary`, { params: { year, month } });
  }
  documentos(filtros: { direction?: string; page?: number; pageSize?: number }): Observable<PaginaDocumentos> {
    let params = new HttpParams();
    for (const [clave, valor] of Object.entries(filtros)) if (valor !== undefined && valor !== '') params = params.set(clave, String(valor));
    return this.http.get<PaginaDocumentos>(`${this.api}/invoices`, { params });
  }
  importar(archivo: File): Observable<Importacion> {
    const datos = new FormData(); datos.append('archivo', archivo);
    return this.http.post<Importacion>(`${this.api}/imports/invoices`, datos, { headers: { 'Idempotency-Key': `${archivo.name}-${archivo.size}-${archivo.lastModified}` } });
  }
  importaciones(): Observable<Importacion[]> { return this.http.get<Importacion[]>(`${this.api}/imports`); }
  reporteComprasVentas(year: number, month: number): Observable<Blob> { return this.http.get(`${this.api}/reports/purchases-sales.xlsx`, { params: { year, month }, responseType: 'blob' }); }
  reporteIva(year: number, month: number): Observable<Blob> { return this.http.get(`${this.api}/reports/vat-estimate.pdf`, { params: { year, month }, responseType: 'blob' }); }
  cuentasBancarias():Observable<CuentaBancaria[]>{return this.http.get<CuentaBancaria[]>(`${this.api}/bank-accounts`)}
  crearCuentaBancaria(datos:{banco:string;tipoCuenta:string;numeroEnmascarado:string;moneda:string}):Observable<CuentaBancaria>{return this.http.post<CuentaBancaria>(`${this.api}/bank-accounts`,datos)}
  importarCartola(cuentaId:string,archivo:File):Observable<Importacion>{const datos=new FormData();datos.append('cuentaId',cuentaId);datos.append('archivo',archivo);return this.http.post<Importacion>(`${this.api}/imports/bank-statements`,datos,{headers:{'Idempotency-Key':`cartola-${cuentaId}-${archivo.name}-${archivo.size}-${archivo.lastModified}`}})}
  movimientosBancarios(filtros:{accountId?:string;classification?:string;page?:number;pageSize?:number}):Observable<PaginaMovimientos>{let params=new HttpParams();for(const [clave,valor] of Object.entries(filtros))if(valor!==undefined&&valor!=='')params=params.set(clave,String(valor));return this.http.get<PaginaMovimientos>(`${this.api}/bank-transactions`,{params})}
  resumenBancario():Observable<ResumenBancario>{return this.http.get<ResumenBancario>(`${this.api}/banking/summary`)}
  clasificarMovimiento(id:string,datos:{clasificacion:string;justificacion:string}):Observable<{id:string}>{return this.http.patch<{id:string}>(`${this.api}/bank-transactions/${id}/classification`,datos)}
  candidatosConciliacion(id:string):Observable<CandidatoConciliacion[]>{return this.http.get<CandidatoConciliacion[]>(`${this.api}/bank-transactions/${id}/candidates`)}
  conciliarMovimiento(id:string,datos:{documentoId:string;monto:string;observacion:string}):Observable<{id:string}>{return this.http.post<{id:string}>(`${this.api}/bank-transactions/${id}/reconciliations`,datos)}
}

