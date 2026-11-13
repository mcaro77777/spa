import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { CurrencyPipe, DatePipe } from '@angular/common';
import { finalize } from 'rxjs';
import { SpaApiService } from '../../core/services/spa-api.service';
import { PaginaDocumentos } from '../../core/models/api.models';

@Component({selector:'app-documents',standalone:true,imports:[CurrencyPipe,DatePipe],templateUrl:'./documents.component.html',styleUrl:'./documents.component.scss',changeDetection:ChangeDetectionStrategy.OnPush})
export class DocumentsComponent {
  private readonly api=inject(SpaApiService);readonly datos=signal<PaginaDocumentos|null>(null);readonly direccion=signal('');readonly pagina=signal(1);readonly cargando=signal(true);readonly error=signal('');
  constructor(){this.cargar()} cargar():void{this.cargando.set(true);this.api.documentos({direction:this.direccion(),page:this.pagina(),pageSize:25}).pipe(finalize(()=>this.cargando.set(false))).subscribe({next:r=>this.datos.set(r),error:()=>this.error.set('No fue posible cargar los documentos.')});}
  filtrar(v:string):void{this.direccion.set(v);this.pagina.set(1);this.cargar()} cambiarPagina(delta:number):void{this.pagina.update(p=>p+delta);this.cargar()}
}

