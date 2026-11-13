import { CurrencyPipe, DecimalPipe, DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { finalize, forkJoin } from 'rxjs';
import { CalculoTributario, ReglaTributaria, ResultadoTributario } from '../../core/models/api.models';
import { SpaApiService } from '../../core/services/spa-api.service';

@Component({selector:'app-tax',standalone:true,imports:[CurrencyPipe,DecimalPipe,DatePipe],templateUrl:'./tax.component.html',styleUrl:'./tax.component.scss',changeDetection:ChangeDetectionStrategy.OnPush})
export class TaxComponent{
  private readonly api=inject(SpaApiService);readonly anio=signal(Math.min(2028,Math.max(2025,new Date().getFullYear())));readonly sobre50k=signal(false);readonly regla=signal<ReglaTributaria|null>(null);readonly calculo=signal<CalculoTributario|null>(null);readonly historial=signal<CalculoTributario[]>([]);readonly cargando=signal(true);readonly calculando=signal(false);readonly error=signal('');readonly anios=[2025,2026,2027,2028];
  constructor(){this.cargar()}
  cargar():void{this.cargando.set(true);this.error.set('');forkJoin({regla:this.api.reglaTributaria(this.anio()),historial:this.api.calculosTributarios()}).pipe(finalize(()=>this.cargando.set(false))).subscribe({next:r=>{this.regla.set(r.regla);this.historial.set(r.historial)},error:e=>this.error.set(e?.error?.detail||'No fue posible cargar la configuración tributaria.')})}
  cambiarAnio(v:string):void{this.anio.set(Number(v));this.calculo.set(null);this.cargar()}
  calcular():void{this.calculando.set(true);this.error.set('');this.api.calcularTributos(this.anio(),this.sobre50k()).pipe(finalize(()=>this.calculando.set(false))).subscribe({next:r=>{this.calculo.set(r);this.historial.update(v=>[r,...v])},error:e=>this.error.set(e?.error?.detail||'No fue posible generar la proyección.')})}
  resultado(codigo:string):ResultadoTributario|undefined{return this.calculo()?.resultados?.find(v=>v.codigo===codigo)}
  numero(codigo:string):number{return Number(this.resultado(codigo)?.valor||0)}
  porcentaje(v:string|undefined):number{return Number(v||0)*100}
}
