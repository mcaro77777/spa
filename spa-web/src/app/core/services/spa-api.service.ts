import { inject, Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import { DashboardResumen, EmpresaResumen, Importacion, PaginaDocumentos } from '../models/api.models';

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
}

