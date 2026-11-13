import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { AuthService } from '../core/auth/auth.service';

@Component({selector:'app-layout',standalone:true,imports:[RouterLink,RouterLinkActive,RouterOutlet],templateUrl:'./app-layout.component.html',styleUrl:'./app-layout.component.scss',changeDetection:ChangeDetectionStrategy.OnPush})
export class AppLayoutComponent {
  readonly auth=inject(AuthService);readonly menuAbierto=signal(false);
  readonly items=[{label:'Resumen',route:'/app/dashboard',icon:'pi pi-chart-pie'},{label:'Documentos',route:'/app/documentos',icon:'pi pi-file'},{label:'Importaciones',route:'/app/importaciones',icon:'pi pi-upload'}];
  cerrar():void{this.menuAbierto.set(false)} cambiarEmpresa():void{location.assign('/empresas')} salir():void{this.auth.cerrarSesion()}
}

