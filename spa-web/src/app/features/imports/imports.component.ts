import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { finalize } from 'rxjs';
import { SpaApiService } from '../../core/services/spa-api.service';
import { Importacion } from '../../core/models/api.models';

@Component({selector:'app-imports',standalone:true,imports:[DatePipe],templateUrl:'./imports.component.html',styleUrl:'./imports.component.scss',changeDetection:ChangeDetectionStrategy.OnPush})
export class ImportsComponent {
  private readonly api=inject(SpaApiService);readonly archivo=signal<File|null>(null);readonly arrastrando=signal(false);readonly cargando=signal(false);readonly error=signal('');readonly resultado=signal<Importacion|null>(null);readonly historial=signal<Importacion[]>([]);
  constructor(){this.cargarHistorial()} seleccionar(event:Event):void{const input=event.target as HTMLInputElement;this.archivo.set(input.files?.[0]||null);this.error.set('');this.resultado.set(null)} soltar(event:DragEvent):void{event.preventDefault();this.arrastrando.set(false);this.archivo.set(event.dataTransfer?.files?.[0]||null)}
  importar():void{const f=this.archivo();if(!f)return;this.cargando.set(true);this.error.set('');this.api.importar(f).pipe(finalize(()=>this.cargando.set(false))).subscribe({next:r=>{this.resultado.set(r);this.archivo.set(null);this.cargarHistorial()},error:e=>this.error.set(e?.error?.detail||'No fue posible procesar el archivo.')});}
  cargarHistorial():void{this.api.importaciones().subscribe({next:r=>this.historial.set(r),error:()=>this.error.set('No fue posible cargar el historial.')})}
}

