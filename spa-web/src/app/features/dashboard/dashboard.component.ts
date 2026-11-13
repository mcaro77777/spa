import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { CurrencyPipe } from '@angular/common';
import { finalize } from 'rxjs';
import { SpaApiService } from '../../core/services/spa-api.service';
import { DashboardResumen, Indicador } from '../../core/models/api.models';

@Component({selector:'app-dashboard',standalone:true,imports:[CurrencyPipe],templateUrl:'./dashboard.component.html',styleUrl:'./dashboard.component.scss',changeDetection:ChangeDetectionStrategy.OnPush})
export class DashboardComponent {
  private readonly api=inject(SpaApiService);readonly ahora=new Date();readonly year=signal(this.ahora.getFullYear());readonly month=signal(this.ahora.getMonth()+1);readonly datos=signal<DashboardResumen|null>(null);readonly cargando=signal(true);readonly error=signal('');
  readonly meses=['Año completo','Enero','Febrero','Marzo','Abril','Mayo','Junio','Julio','Agosto','Septiembre','Octubre','Noviembre','Diciembre'];
  constructor(){this.cargar()}
  cargar():void{this.cargando.set(true);this.error.set('');this.api.dashboard(this.year(),this.month()).pipe(finalize(()=>this.cargando.set(false))).subscribe({next:r=>this.datos.set(r),error:()=>this.error.set('No fue posible cargar el resumen.')});}
  descargar(tipo:'excel'|'pdf'):void{const solicitud=tipo==='excel'?this.api.reporteComprasVentas(this.year(),this.month()):this.api.reporteIva(this.year(),this.month());solicitud.subscribe({next:archivo=>{const url=URL.createObjectURL(archivo);const enlace=document.createElement('a');enlace.href=url;enlace.download=tipo==='excel'?`compras-ventas-${this.year()}-${this.month()}.xlsx`:`iva-estimado-${this.year()}-${this.month()}.pdf`;enlace.click();URL.revokeObjectURL(url)},error:()=>this.error.set('No fue posible generar el reporte.')});}
  valor(i:Indicador):number{return Number(i.valor)} icono(codigo:string):string{return ({ventas_netas:'pi pi-arrow-up-right',compras_netas:'pi pi-cart-minus',iva_debito:'pi pi-receipt',iva_credito:'pi pi-wallet',iva_pagar:'pi pi-calendar-clock',resultado_provisional:'pi pi-chart-line',pendientes_revision:'pi pi-exclamation-circle'} as Record<string,string>)[codigo]||'pi pi-circle'}
}

