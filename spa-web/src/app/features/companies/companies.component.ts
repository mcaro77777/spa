import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { ReactiveFormsModule, FormBuilder, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { finalize, switchMap } from 'rxjs';
import { AuthService } from '../../core/auth/auth.service';
import { ProblemaApi } from '../../core/models/api.models';
import { SpaApiService } from '../../core/services/spa-api.service';

@Component({selector:'app-companies',standalone:true,imports:[ReactiveFormsModule],templateUrl:'./companies.component.html',styleUrl:'./companies.component.scss',changeDetection:ChangeDetectionStrategy.OnPush})
export class CompaniesComponent {
  readonly auth=inject(AuthService);private readonly api=inject(SpaApiService);private readonly fb=inject(FormBuilder);private readonly router=inject(Router);
  readonly creando=signal(false);readonly cargando=signal(false);readonly error=signal('');
  readonly form=this.fb.nonNullable.group({razonSocial:['',Validators.required],nombreFantasia:[''],rut:['',Validators.required],giro:[''],correo:['',Validators.email],telefono:[''],direccion:[''],comuna:[''],region:[''],regimenCodigo:['pro_pyme_general_14d3',Validators.required]});
  seleccionar(id:string):void{this.cargando.set(true);this.auth.seleccionarEmpresa(id).pipe(finalize(()=>this.cargando.set(false))).subscribe({next:()=>void this.router.navigate(['/app/dashboard']),error:()=>this.error.set('No fue posible abrir la empresa.')});}
  crear():void{if(this.form.invalid){this.form.markAllAsTouched();return}this.cargando.set(true);this.error.set('');this.api.crearEmpresa(this.form.getRawValue()).pipe(switchMap(e=>this.auth.seleccionarEmpresa(e.id)),finalize(()=>this.cargando.set(false))).subscribe({next:()=>void this.router.navigate(['/app/dashboard']),error:(e:HttpErrorResponse)=>this.error.set((e.error as ProblemaApi|undefined)?.detail||'No fue posible crear la empresa.')});}
  salir():void{this.auth.cerrarSesion();}
}

