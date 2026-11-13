import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { finalize } from 'rxjs';
import { ConversacionIA } from '../../core/models/api.models';
import { SpaApiService } from '../../core/services/spa-api.service';

interface MensajeVista{rol:'usuario'|'asistente';contenido:string;proveedor?:string;referencias?:string[];advertencias?:string[]}

@Component({selector:'app-assistant',standalone:true,imports:[FormsModule,DatePipe],templateUrl:'./assistant.component.html',styleUrl:'./assistant.component.scss',changeDetection:ChangeDetectionStrategy.OnPush})
export class AssistantComponent{
  private readonly api=inject(SpaApiService);readonly conversaciones=signal<ConversacionIA[]>([]);readonly conversacionId=signal('');readonly mensajes=signal<MensajeVista[]>([]);readonly texto=signal('');readonly consentimiento=signal(false);readonly enviando=signal(false);readonly error=signal('');readonly preguntas=['¿Cuánto podría pagar de impuesto?','¿Qué información tengo pendiente?','Explícame mi resultado tributario','¿Cuál es la diferencia entre caja y utilidad?'];
  constructor(){this.api.conversaciones().subscribe({next:v=>this.conversaciones.set(v)})}
  nueva():void{this.conversacionId.set('');this.mensajes.set([]);this.texto.set('');this.error.set('')}
  usar(pregunta:string):void{this.texto.set(pregunta)}
  enviar():void{const mensaje=this.texto().trim();if(!mensaje||this.enviando())return;this.mensajes.update(v=>[...v,{rol:'usuario',contenido:mensaje}]);this.texto.set('');this.enviando.set(true);this.error.set('');this.api.conversar({conversacionId:this.conversacionId(),mensaje,consentimientoExterno:this.consentimiento()}).pipe(finalize(()=>this.enviando.set(false))).subscribe({next:r=>{this.conversacionId.set(r.conversacionId);this.mensajes.update(v=>[...v,{rol:'asistente',contenido:r.respuesta,proveedor:r.proveedor,referencias:r.referencias,advertencias:r.advertencias}]);this.api.conversaciones().subscribe(v=>this.conversaciones.set(v))},error:e=>{this.error.set(e?.error?.detail||'No fue posible obtener una respuesta.');this.mensajes.update(v=>v.slice(0,-1))}})}
}
